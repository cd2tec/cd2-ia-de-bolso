// Package agenda roda as consultas salvas agendadas sem a tela. Para não haver duas versões da
// montagem do SQL e das regras de horário, o Motor avalia os mesmos arquivos web/*.js da página
// num interpretador JS embutido (goja).
package agenda

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dop251/goja"
)

//go:embed intl.js
var intlJS string

// modulos são os arquivos de web/, na ordem em que um só pode exigir os anteriores.
var modulos = []string{"chart", "catalog", "notas", "toolbox", "consultas", "ask", "agenda", "condicao", "resumo", "aprofundar"}

// cola liga o Go aos mesmos prepararSalva e resultadoSalva que o rodarSalva (ask.js) usa antes e
// depois do runSql, e aos horários da agenda. Tudo entra e sai como JSON; datas em ms Unix.
// Sem tela, o motivo da recusa vai em erro: o texto do guardSql, ou o erro que o banco devolveu.
const cola = `(function(g){
"use strict";
const Ask = g.__mod['./ask.js'], Tb = g.__mod['./toolbox.js'], Ag = g.__mod['./agenda.js'], Cond = g.__mod['./condicao.js'], Res = g.__mod['./resumo.js'], Ap = g.__mod['./aprofundar.js'];
g.__cd2 = {
  horarios(cj, de, ate){
    const c = JSON.parse(cj), a = c && c.agenda;
    return JSON.stringify(a ? Ag.horarios(a, new Date(de), new Date(ate)).map(d => d.getTime()) : []);
  },
  preparar(cj, ref, motor){
    const c = JSON.parse(cj);
    let valores = []; // sem agenda, montarSql usa o valor salvo de cada parâmetro
    if(c && c.agenda){
      const v = Ag.valoresDaExecucao(c, new Date(ref));
      if(!v.ok) return JSON.stringify({ erro:v.erro });
      valores = v.valores;
    }
    const p = Ask.prepararSalva(c, valores, motor);
    if(p.ok) return JSON.stringify({ sql:p.sql, valores:p.valores });
    return JSON.stringify({ erro:p.barrado ? Tb.guardSql(p.sql).error : p.erro });
  },
  resultado(cj, texto, valores){
    const r = Ask.parseRows(texto);
    if(!r.ok && Ask.isTimeout(r.error)) r.timeout = true;
    const out = Ask.resultadoSalva(JSON.parse(cj), r, valores, '');
    if(!r.ok && !r.timeout) out.erro = r.error;
    return JSON.stringify(out);
  },
  julgar(cj, rj, aj){
    const c = JSON.parse(cj), cond = c && c.agenda ? c.agenda.condicao : null;
    return JSON.stringify(Cond.julgar(cond == null ? null : cond, JSON.parse(rj), JSON.parse(aj)));
  },
  resumoPedido(cj, rj, jj, aj){
    return JSON.stringify(Res.pedido(JSON.parse(cj), JSON.parse(rj), JSON.parse(jj), JSON.parse(aj)));
  },
  resumoConferir(texto, cj, rj, jj, aj){
    return JSON.stringify(Res.conferir(texto, JSON.parse(cj), JSON.parse(rj), JSON.parse(jj), JSON.parse(aj)));
  },
  aprofIds(cj){ return JSON.stringify(Ap.ids(JSON.parse(cj))); },
  aprofOpcoes(cj, lj){ return JSON.stringify(Ap.opcoes(JSON.parse(cj), JSON.parse(lj))); },
  aprofPedido(cj, jj, oj){ return JSON.stringify(Ap.pedido(JSON.parse(cj), JSON.parse(jj), JSON.parse(oj))); },
  aprofConferir(texto, oj){ return JSON.stringify(Ap.conferir(texto, JSON.parse(oj))); },
  aprofDeNovo(mj, texto, confj){
    return JSON.stringify({ mensagens:Ap.novaTentativa(JSON.parse(mj), texto, JSON.parse(confj)), max:Ap.MAX_TENTATIVAS });
  },
  resumoDeNovo(mj, texto, confj){
    return JSON.stringify({ mensagens:Res.novaTentativa(JSON.parse(mj), texto, JSON.parse(confj)), max:Res.MAX_TENTATIVAS });
  }
};
})(globalThis);`

// Motor guarda um interpretador com os módulos de web/ carregados. O goja não é seguro entre
// goroutines: toda chamada passa pelo mutex.
type Motor struct {
	mu sync.Mutex
	vm *goja.Runtime
}

// Preparo é o SQL pronto para o db_read_only_query. Erro != "" quando montarSql, guardSql ou os
// períodos da agenda recusaram; é a mesma mensagem da tela.
type Preparo struct {
	SQL     string   `json:"sql"`
	Valores []string `json:"valores"`
	Erro    string   `json:"erro"`
}

// Resultado tem os campos do retorno de rodarSalva. Motivo: "sql" | "vazio" | "timeout" |
// "timeout_front" (este último quem põe é o agendador, quando o prazo dele estoura). Erro leva o
// erro do banco quando Motivo é "sql", ou a recusa do Preparar.
type Resultado struct {
	Ok      bool             `json:"ok"`
	Motivo  string           `json:"motivo,omitempty"`
	Erro    string           `json:"erro,omitempty"`
	Visual  string           `json:"visual,omitempty"`
	X       string           `json:"x,omitempty"`
	Y       string           `json:"y,omitempty"`
	Titulo  string           `json:"titulo,omitempty"`
	Frase   string           `json:"frase,omitempty"`
	Colunas []string         `json:"colunas,omitempty"`
	Linhas  []map[string]any `json:"linhas,omitempty"`
	Todas   []map[string]any `json:"todas,omitempty"`
	Aviso   string           `json:"aviso,omitempty"`
}

// Julgamento é o que web/condicao.js (julgar) decidiu sobre uma execução de consulta com condição.
// Aviso diz se a execução notifica e conta como novidade; Disparou, se a regra bateu. Valor, Base e
// Variacao (em %) vêm quando a condição foi conferida; Texto é a frase do aviso e do card.
type Julgamento struct {
	Avaliada bool     `json:"avaliada"`
	Disparou bool     `json:"disparou"`
	Aviso    bool     `json:"aviso"`
	Motivo   string   `json:"motivo,omitempty"`
	Valor    *float64 `json:"valor,omitempty"`
	Base     *float64 `json:"base,omitempty"`
	Variacao *float64 `json:"variacao,omitempty"`
	N        int      `json:"n,omitempty"`
	Texto    string   `json:"texto"`
}

// PedidoResumo é o que web/resumo.js (pedido) monta para o modelo quando a condição disparou. O mesmo
// formato serve à escolha do aprofundamento (web/aprofundar.js): Nome "escolha" e Thinking ligado, que
// só ela pede (CIDB-17). Nome vazio é "resumo".
type PedidoResumo struct {
	Mensagens []map[string]string `json:"mensagens"`
	Schema    json.RawMessage     `json:"schema"`
	MaxTokens int                 `json:"maxTokens"`
	Nome      string              `json:"nome,omitempty"`
	Thinking  bool                `json:"thinking,omitempty"`
}

// OpcaoAprof é uma consulta vinculada que o modelo pode escolher: o id e o nome que vai no enum.
type OpcaoAprof struct {
	Id   string `json:"id"`
	Nome string `json:"nome"`
}

// Escolha é o que web/aprofundar.js (conferir) decidiu sobre a resposta da escolha: Id e Nome quando
// Ok; senão Motivo ("formato" | "fora") e Detalhe.
type Escolha struct {
	Ok      bool   `json:"ok"`
	Id      string `json:"id,omitempty"`
	Nome    string `json:"nome,omitempty"`
	Motivo  string `json:"motivo,omitempty"`
	Detalhe string `json:"detalhe,omitempty"`
}

// Conferencia é o que web/resumo.js (conferir) decidiu sobre a resposta do modelo: Linhas quando
// Ok; senão Motivo ("formato" | "numero") e Detalhe.
type Conferencia struct {
	Ok      bool     `json:"ok"`
	Linhas  []string `json:"linhas,omitempty"`
	Motivo  string   `json:"motivo,omitempty"`
	Detalhe string   `json:"detalhe,omitempty"`
}

// MaxBase é quantas execuções anteriores o agendador lê para a base da condição (o MAX_N do JS).
const MaxBase = 30

// TemCondicao diz se a consulta salva tem agenda.condicao.
func TemCondicao(consulta json.RawMessage) bool {
	var c struct {
		Agenda *struct {
			Condicao json.RawMessage `json:"condicao"`
		} `json:"agenda"`
	}
	if json.Unmarshal(consulta, &c) != nil || c.Agenda == nil {
		return false
	}
	t := string(c.Agenda.Condicao)
	return t != "" && t != "null"
}

// NovoMotor avalia os módulos de webDir na ordem de modulos. Cada um roda com module/exports
// próprios, e require('./x.js') devolve o module.exports de um módulo já avaliado.
func NovoMotor(webDir string) (*Motor, error) {
	vm := goja.New()
	if _, err := vm.RunScript("intl.js", intlJS); err != nil {
		return nil, fmt.Errorf("agenda: intl.js: %w", err)
	}
	mods := vm.NewObject()
	if err := vm.Set("__mod", mods); err != nil {
		return nil, err
	}
	err := vm.Set("require", func(nome string) (goja.Value, error) {
		v := mods.Get(nome)
		if v == nil {
			return nil, fmt.Errorf("require: %s não carregado", nome)
		}
		return v, nil
	})
	if err != nil {
		return nil, err
	}
	for _, nome := range modulos {
		arq := filepath.Join(webDir, nome+".js")
		src, err := os.ReadFile(arq)
		if err != nil {
			return nil, fmt.Errorf("agenda: %w", err)
		}
		mod := vm.NewObject()
		if err := mod.Set("exports", vm.NewObject()); err != nil {
			return nil, err
		}
		if err := vm.Set("module", mod); err != nil {
			return nil, err
		}
		if err := vm.Set("exports", mod.Get("exports")); err != nil {
			return nil, err
		}
		if _, err := vm.RunScript(arq, string(src)); err != nil {
			return nil, fmt.Errorf("agenda: %s: %w", nome, err)
		}
		if err := mods.Set("./"+nome+".js", mod.Get("exports")); err != nil {
			return nil, err
		}
	}
	if _, err := vm.RunString("delete globalThis.module; delete globalThis.exports;"); err != nil {
		return nil, err
	}
	if _, err := vm.RunScript("cola.js", cola); err != nil {
		return nil, fmt.Errorf("agenda: cola: %w", err)
	}
	return &Motor{vm: vm}, nil
}

// chamar roda __cd2[fn](args...) e decodifica o JSON devolvido em out.
func (m *Motor) chamar(out any, fn string, args ...any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := goja.AssertFunction(m.vm.Get("__cd2").ToObject(m.vm).Get(fn))
	if !ok {
		return fmt.Errorf("agenda: %s não existe", fn)
	}
	vs := make([]goja.Value, len(args))
	for i, a := range args {
		vs[i] = m.vm.ToValue(a)
	}
	v, err := f(goja.Undefined(), vs...)
	if err != nil {
		return fmt.Errorf("agenda: %s: %w", fn, err)
	}
	return json.Unmarshal([]byte(v.String()), out)
}

// Horarios lista os horários da agenda da consulta em (de, ate]. Sem agenda ou desligada: vazio.
func (m *Motor) Horarios(consulta json.RawMessage, de, ate time.Time) ([]time.Time, error) {
	var ms []int64
	if err := m.chamar(&ms, "horarios", string(consulta), de.UnixMilli(), ate.UnixMilli()); err != nil {
		return nil, err
	}
	out := make([]time.Time, len(ms))
	for i, x := range ms {
		out[i] = time.UnixMilli(x)
	}
	return out, nil
}

// Preparar faz o trecho de rodarSalva antes do runSql: valores da execução em ref (pela agenda,
// ou os salvos), montarSql, guardSql e prepareSql para o motor do banco.
func (m *Motor) Preparar(consulta json.RawMessage, ref time.Time, motor string) (Preparo, error) {
	var p Preparo
	err := m.chamar(&p, "preparar", string(consulta), ref.UnixMilli(), motor)
	return p, err
}

// Resultado faz o trecho de rodarSalva depois do runSql (resultadoSalva) sobre o texto devolvido
// pela tool. valores segue para o resultadoSalva como no rodarSalva, mas não volta no Resultado:
// os valores que rodaram ficam na Execucao.
func (m *Motor) Resultado(consulta json.RawMessage, textoTool string, valores []string) (Resultado, error) {
	if valores == nil {
		valores = []string{}
	}
	var r Resultado
	err := m.chamar(&r, "resultado", string(consulta), textoTool, valores)
	return r, err
}

// Julgar confere a condição da consulta contra o resultado e as linhas das execuções anteriores
// (da mais nova para a mais velha). Sem condição: nil.
func (m *Motor) Julgar(consulta json.RawMessage, r Resultado, anteriores [][]map[string]any) (*Julgamento, error) {
	if anteriores == nil {
		anteriores = [][]map[string]any{}
	}
	rj, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	aj, err := json.Marshal(anteriores)
	if err != nil {
		return nil, err
	}
	var j *Julgamento
	if err := m.chamar(&j, "julgar", string(consulta), string(rj), string(aj)); err != nil {
		return nil, err
	}
	return j, nil
}

// ResumoPedido monta o pedido do resumo. nil quando não há resumo (a condição não disparou).
// ap: o aprofundamento que rodou (História 4), ou nil; sem ele, o pedido é o da História 3.
func (m *Motor) ResumoPedido(consulta json.RawMessage, r Resultado, j *Julgamento, ap *Aprofundamento) (*PedidoResumo, error) {
	rj, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	jj, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}
	aj, err := json.Marshal(ap.paraResumo())
	if err != nil {
		return nil, err
	}
	var p *PedidoResumo
	if err := m.chamar(&p, "resumoPedido", string(consulta), string(rj), string(jj), string(aj)); err != nil {
		return nil, err
	}
	return p, nil
}

// ResumoConferir confere o texto que o modelo devolveu contra os mesmos dados do pedido.
func (m *Motor) ResumoConferir(texto string, consulta json.RawMessage, r Resultado, j *Julgamento, ap *Aprofundamento) (Conferencia, error) {
	var c Conferencia
	rj, err := json.Marshal(r)
	if err != nil {
		return c, err
	}
	jj, err := json.Marshal(j)
	if err != nil {
		return c, err
	}
	aj, err := json.Marshal(ap.paraResumo())
	if err != nil {
		return c, err
	}
	err = m.chamar(&c, "resumoConferir", texto, string(consulta), string(rj), string(jj), string(aj))
	return c, err
}

// AprofIds lista os ids vinculados como aprofundamento na agenda da consulta (até 3, sem repetição).
func (m *Motor) AprofIds(consulta json.RawMessage) ([]string, error) {
	var ids []string
	err := m.chamar(&ids, "aprofIds", string(consulta))
	return ids, err
}

// AprofOpcoes resolve os ids vinculados contra a lista das consultas salvas: as que existem, com o nome
// que vai no enum.
func (m *Motor) AprofOpcoes(consulta json.RawMessage, lista []OpcaoAprof) ([]OpcaoAprof, error) {
	if lista == nil {
		lista = []OpcaoAprof{}
	}
	lj, err := json.Marshal(lista)
	if err != nil {
		return nil, err
	}
	var ops []OpcaoAprof
	err = m.chamar(&ops, "aprofOpcoes", string(consulta), string(lj))
	return ops, err
}

// AprofPedido monta o pedido da escolha. nil quando não há escolha (não disparou ou sem opções).
func (m *Motor) AprofPedido(consulta json.RawMessage, j *Julgamento, ops []OpcaoAprof) (*PedidoResumo, error) {
	jj, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}
	oj, err := json.Marshal(ops)
	if err != nil {
		return nil, err
	}
	var p *PedidoResumo
	if err := m.chamar(&p, "aprofPedido", string(consulta), string(jj), string(oj)); err != nil {
		return nil, err
	}
	return p, nil
}

// AprofConferir confere a resposta da escolha contra as opções do pedido.
func (m *Motor) AprofConferir(texto string, ops []OpcaoAprof) (Escolha, error) {
	var e Escolha
	oj, err := json.Marshal(ops)
	if err != nil {
		return e, err
	}
	err = m.chamar(&e, "aprofConferir", texto, string(oj))
	return e, err
}

// AprofDeNovo monta as mensagens da tentativa seguinte da escolha e diz o limite de tentativas.
func (m *Motor) AprofDeNovo(mensagens []map[string]string, texto string, e Escolha) ([]map[string]string, int, error) {
	mj, err := json.Marshal(mensagens)
	if err != nil {
		return nil, 0, err
	}
	ej, err := json.Marshal(e)
	if err != nil {
		return nil, 0, err
	}
	var out struct {
		Mensagens []map[string]string `json:"mensagens"`
		Max       int                 `json:"max"`
	}
	if err := m.chamar(&out, "aprofDeNovo", string(mj), texto, string(ej)); err != nil {
		return nil, 0, err
	}
	return out.Mensagens, out.Max, nil
}

// ResumoDeNovo monta as mensagens da tentativa seguinte (a resposta recusada e o motivo) e diz o limite
// de tentativas (MAX_TENTATIVAS de web/resumo.js).
func (m *Motor) ResumoDeNovo(mensagens []map[string]string, texto string, c Conferencia) ([]map[string]string, int, error) {
	mj, err := json.Marshal(mensagens)
	if err != nil {
		return nil, 0, err
	}
	cj, err := json.Marshal(c)
	if err != nil {
		return nil, 0, err
	}
	var out struct {
		Mensagens []map[string]string `json:"mensagens"`
		Max       int                 `json:"max"`
	}
	if err := m.chamar(&out, "resumoDeNovo", string(mj), texto, string(cj)); err != nil {
		return nil, 0, err
	}
	return out.Mensagens, out.Max, nil
}
