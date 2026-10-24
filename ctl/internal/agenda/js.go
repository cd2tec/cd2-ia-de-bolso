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
var modulos = []string{"chart", "catalog", "notas", "toolbox", "consultas", "ask", "agenda"}

// cola expõe ao Go os trechos de rodarSalva (ask.js) antes e depois do runSql, e os horários da
// agenda. Tudo entra e sai como JSON; datas em milissegundos Unix.
const cola = `(function(g){
"use strict";
const Ask = g.__mod['./ask.js'], Tb = g.__mod['./toolbox.js'], Cons = g.__mod['./consultas.js'], Ag = g.__mod['./agenda.js'];
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
    const m = Cons.montarSql(c, valores);
    if(!m.ok) return JSON.stringify({ erro:m.erro });
    const gs = Tb.guardSql(m.sql);
    if(!gs.ok) return JSON.stringify({ erro:gs.error });
    return JSON.stringify({ sql:Tb.prepareSql(gs.sql, motor), valores:m.valores });
  },
  resultado(cj, texto){
    const c = JSON.parse(cj) || {};
    const r = Ask.parseRows(texto);
    if(!r.ok) return JSON.stringify({ ok:false, motivo:Ask.isTimeout(r.error) ? 'timeout' : 'sql' });
    if(!r.rows.length || r.rows.every(l => Object.values(l).every(v => v == null || v === ''))) return JSON.stringify({ ok:false, motivo:'vazio' });
    const colunas = Object.keys(r.rows[0]);
    const v = Ask.pickVisual({ visual:c.visual, x:c.x, y:c.y }, colunas, r.rows);
    const out = { ok:true, visual:v.visual, x:v.x, y:v.y, titulo:String(c.nome || ''),
      frase:Ask.frase({ visual:v.visual, x:v.x, y:v.y, linhas:v.linhas, todas:r.rows }), colunas, linhas:v.linhas, todas:r.rows };
    if(v.aviso) out.aviso = v.aviso;
    return JSON.stringify(out);
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
// "timeout_front" (este último quem põe é o agendador, quando o prazo dele estoura).
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

// Resultado faz o trecho de rodarSalva depois do runSql sobre o texto devolvido pela tool:
// parseRows, a regra do vazio, pickVisual e frase. valores não entra no cálculo: os valores que
// rodaram ficam na Execucao, não no Resultado.
func (m *Motor) Resultado(consulta json.RawMessage, textoTool string, valores []string) (Resultado, error) {
	_ = valores
	var r Resultado
	err := m.chamar(&r, "resultado", string(consulta), textoTool)
	return r, err
}
