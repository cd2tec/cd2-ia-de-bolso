package agenda

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/notas"
)

var (
	motorUnico     *Motor
	motorUnicoErr  error
	motorUnicoOnce sync.Once
)

// motorComum: um Motor só para os testes do agendador (ele é seguro entre goroutines).
func motorComum(t *testing.T) *Motor {
	t.Helper()
	motorUnicoOnce.Do(func() { motorUnico, motorUnicoErr = NovoMotor(webDir) })
	if motorUnicoErr != nil {
		t.Fatal(motorUnicoErr)
	}
	return motorUnico
}

var bancoT = notas.Banco{Motor: "postgres", Host: "h", Porta: 5432, Base: "b"}

const linhasT = `[{"loja":"A","total":1234.5},{"loja":"B","total":10}]`

// ambiente falso: banco conectado, notas em memória e Consultar trocável.
type ambiente struct {
	mu        sync.Mutex
	conectado bool
	banco     notas.Banco
	consultas []map[string]any
	consultar func(ctx context.Context, sql string) (string, error)
	chamadas  atomic.Int32
	agora     func() time.Time // nil: sábado 24/10 12:00
	lerErr    error            // != nil: Consultas falha com ele
	pronto    func() bool      // nil: o banco está pronto
}

func (f *ambiente) relogio(r func() time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.agora = r
}

func (f *ambiente) por(id, nome string, agenda any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := map[string]any{"id": id, "nome": nome, "sql": "SELECT loja, total FROM vendas",
		"visual": "barra", "x": "loja", "y": "total", "params": []any{}, "criada": "2026-10-01"}
	if agenda != nil {
		c["agenda"] = agenda
	}
	f.consultas = append(f.consultas, c)
}

func (f *ambiente) apagar(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, c := range f.consultas {
		if c["id"] == id {
			f.consultas = append(f.consultas[:i], f.consultas[i+1:]...)
			return
		}
	}
}

func novoAmbiente() *ambiente {
	return &ambiente{conectado: true, banco: bancoT,
		consultar: func(context.Context, string) (string, error) { return linhasT, nil }}
}

func novoAgendador(t *testing.T, f *ambiente) *Agendador {
	t.Helper()
	return NovoAgendador(Opcoes{
		Agora: func() time.Time {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.agora != nil {
				return f.agora()
			}
			return time.Date(2026, 10, 24, 12, 0, 0, 0, time.Local)
		},
		Banco: func() (notas.Banco, bool) {
			f.mu.Lock()
			defer f.mu.Unlock()
			return f.banco, f.conectado
		},
		Consultas: func(notas.Banco) (json.RawMessage, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.lerErr != nil {
				return nil, f.lerErr
			}
			return json.Marshal(map[string]any{"versao": 1, "tabelas": map[string]any{}, "consultas": f.consultas})
		},
		Consultar: func(ctx context.Context, sql string) (string, error) {
			f.chamadas.Add(1)
			return f.consultar(ctx, sql)
		},
		Pronto: func() bool {
			f.mu.Lock()
			p := f.pronto
			f.mu.Unlock()
			return p == nil || p()
		},
		Motor: motorComum(t),
		Hist:  novoHist(t, t.TempDir()),
		Tick:  30 * time.Second,
	})
}

// ligarExecutor sobe o executor e, no fim do teste, para e espera ele sair.
func ligarExecutor(t *testing.T, a *Agendador) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	fim := make(chan struct{})
	go func() { a.executar(ctx); close(fim) }()
	t.Cleanup(func() { cancel(); <-fim })
}

func historico(t *testing.T, a *Agendador, id string) []Execucao {
	t.Helper()
	is, _, err := a.o.Hist.Pagina(notas.Chave(bancoT), id, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	return is
}

func dia(d, h, m int) time.Time { return time.Date(2026, 10, d, h, m, 0, 0, time.Local) }

var cada5 = map[string]any{"ligado": true, "freq": "minutos", "a_cada": 5}

func TestPerdasAoAbrirViramUmaLinha(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", cada5)
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	aberta, agora := dia(23, 20, 0), dia(26, 8, 0) // sexta 20:00, segunda 08:00
	if err := a.o.Hist.MarcarAberta(notas.Chave(bancoT), aberta); err != nil {
		t.Fatal(err)
	}
	a.Passo(agora)
	a.esperarFila()
	is := historico(t, a, "c1")
	if len(is) != 2 || is[0].Tipo != "agendada" || is[1].Tipo != "perda" {
		t.Fatalf("histórico = %+v", is)
	}
	p := is[1]
	n := int(agora.Add(-time.Minute).Sub(aberta) / (5 * time.Minute)) // 719
	if _, o1 := aberta.Zone(); func() bool { _, o2 := agora.Zone(); return o1 != o2 }() {
		// Fuso com horário de verão no meio: a conta em minutos não vale, os horários são de relógio.
		raw, _ := json.Marshal(f.consultas[0])
		hs, _ := a.o.Motor.Horarios(raw, aberta, agora.Add(-time.Minute))
		n = len(hs)
	}
	if p.N != n || p.Motivo != "fechada" {
		t.Fatalf("perda = %+v, quero N=%d fechada", p, n)
	}
	if !p.De.Equal(dia(23, 20, 5)) || !p.Ate.Equal(dia(26, 7, 55)) || !p.Ref.Equal(*p.Ate) {
		t.Errorf("de %v até %v ref %v", p.De, p.Ate, p.Ref)
	}
	if !is[0].Ref.Equal(agora) {
		t.Errorf("rodou com ref %v", is[0].Ref)
	}
	av, ultimo := a.Avisos(0)
	if len(av) != 2 || ultimo != av[1].Seq {
		t.Fatalf("avisos = %+v, %d", av, ultimo)
	}
	if av[0].Titulo != "IA de Bolso" || av[0].Texto != itoa(p.N)+" execuções não rodaram enquanto a IA estava fechada" {
		t.Errorf("aviso das perdas = %+v", av[0])
	}
}

func TestPerdaUmaSoNoSingular(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", map[string]any{"ligado": true, "freq": "diario", "a_cada": 1, "hora": "07:00"})
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	_ = a.o.Hist.MarcarAberta(notas.Chave(bancoT), dia(25, 20, 0))
	a.Passo(dia(26, 8, 0))
	a.esperarFila()
	av, _ := a.Avisos(0)
	if len(av) != 1 || av[0].Texto != "1 execução não rodou enquanto a IA estava fechada" {
		t.Errorf("avisos = %+v", av)
	}
}

func TestPerdaOutroBanco(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", cada5)
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	_ = a.o.Hist.MarcarAberta(notas.Chave(bancoT), dia(26, 7, 0))
	_ = a.o.Hist.MarcarAberta(notas.Chave(notas.Banco{Motor: "postgres", Host: "outro"}), dia(26, 7, 30))
	a.Passo(dia(26, 8, 0))
	a.esperarFila()
	is := historico(t, a, "c1")
	if len(is) != 2 || is[1].Tipo != "perda" || is[1].Motivo != "outro_banco" || is[1].N != 11 {
		t.Fatalf("histórico = %+v", is)
	}
}

func TestPrimeiraVezSemAbertaNaoPerde(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", cada5)
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	a.Passo(dia(26, 8, 2))
	a.esperarFila()
	if is := historico(t, a, "c1"); len(is) != 0 {
		t.Fatalf("histórico = %+v", is)
	}
	if av, _ := a.Avisos(0); len(av) != 0 {
		t.Errorf("avisos = %+v", av)
	}
	if b, _, _ := a.o.Hist.Aberta(notas.Chave(bancoT)); !b.Equal(dia(26, 8, 2)) {
		t.Errorf("aberta = %v", b)
	}
}

func TestComputadorDormiu(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", map[string]any{"ligado": true, "freq": "diario", "a_cada": 1, "hora": "07:30"})
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	a.Passo(dia(26, 7, 0))
	a.Passo(dia(26, 7, 0).Add(30 * time.Second))
	a.Passo(dia(26, 9, 0))
	a.esperarFila()
	is := historico(t, a, "c1")
	if len(is) != 1 || is[0].Tipo != "perda" || is[0].Motivo != "dormindo" || is[0].N != 1 || !is[0].Ate.Equal(dia(26, 7, 30)) {
		t.Fatalf("histórico = %+v", is)
	}
	if n := f.chamadas.Load(); n != 0 {
		t.Errorf("Consultar chamado %d vezes", n)
	}
	if av, _ := a.Avisos(0); len(av) != 0 {
		t.Errorf("avisos = %+v", av)
	}
}

func TestRelogioVoltou(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", cada5)
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	a.Passo(dia(26, 10, 0))
	a.esperarFila()
	a.Passo(dia(26, 9, 0))
	a.esperarFila()
	if is := historico(t, a, "c1"); len(is) != 1 {
		t.Fatalf("depois de voltar: %+v", is)
	}
	if b, _, _ := a.o.Hist.Aberta(notas.Chave(bancoT)); !b.Equal(dia(26, 10, 0)) {
		t.Errorf("aberta = %v", b)
	}
	a.Passo(dia(26, 10, 5))
	a.Passo(dia(26, 10, 5).Add(30 * time.Second))
	a.esperarFila()
	is := historico(t, a, "c1")
	if len(is) != 2 || is[0].Tipo != "agendada" || !is[0].Ref.Equal(dia(26, 10, 5)) || !is[1].Ref.Equal(dia(26, 10, 0)) {
		t.Fatalf("histórico = %+v", is)
	}
}

func TestNaHoraRoda(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", map[string]any{"ligado": true, "freq": "diario", "a_cada": 1, "hora": "14:00"})
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	a.Passo(dia(26, 14, 0).Add(40 * time.Second))
	a.esperarFila()
	is := historico(t, a, "c1")
	if len(is) != 1 || is[0].Tipo != "agendada" || !is[0].Ref.Equal(dia(26, 14, 0)) || is[0].Resultado == nil || !is[0].Resultado.Ok {
		t.Fatalf("histórico = %+v", is)
	}
	av, _ := a.Avisos(0)
	if len(av) != 1 || av[0].ConsultaId != "c1" || av[0].Titulo != "IA de Bolso · Vendas" ||
		av[0].Texto != "Rodou às 14:00 · "+is[0].Resultado.Frase || is[0].Resultado.Frase == "" {
		t.Fatalf("avisos = %+v", av)
	}
	// Fora da folga de 1 minuto não roda.
	a2 := novoAgendador(t, f)
	ligarExecutor(t, a2)
	a2.Passo(dia(27, 14, 1).Add(time.Second))
	a2.esperarFila()
	if is := historico(t, a2, "c1"); len(is) != 0 {
		t.Errorf("rodou atrasado: %+v", is)
	}
}

func TestAvisoSemFrase(t *testing.T) {
	cases := []struct {
		nome      string
		consultar func(context.Context, string) (string, error)
		texto     string
	}{
		{"prazo", func(context.Context, string) (string, error) { return "", ErrPrazo }, mensagens["timeout_front"]},
		{"banco fora do ar", func(context.Context, string) (string, error) { return "", errors.New("conexão recusada") }, "conexão recusada"},
		{"vazio", func(context.Context, string) (string, error) { return "[]", nil }, mensagens["vazio"]},
		{"erro do banco", func(context.Context, string) (string, error) { return `ERRO: relation "x" does not exist`, nil }, `relation "x" does not exist`},
		{"tabela", func(context.Context, string) (string, error) { return `[{"a":1,"b":2,"c":3}]`, nil }, "1 linha."},
	}
	for _, c := range cases {
		f := novoAmbiente()
		f.consultar = c.consultar
		ag := map[string]any{"ligado": true, "freq": "diario", "a_cada": 1, "hora": "14:00"}
		if c.nome == "tabela" {
			f.mu.Lock()
			f.consultas = []map[string]any{{"id": "c1", "nome": "T", "sql": "SELECT a, b, c FROM t", "visual": "tabela", "params": []any{}, "agenda": ag}}
			f.mu.Unlock()
		} else {
			f.por("c1", "Vendas", ag)
		}
		a := novoAgendador(t, f)
		ligarExecutor(t, a)
		a.Passo(dia(26, 14, 0))
		a.esperarFila()
		av, _ := a.Avisos(0)
		if len(av) != 1 || av[0].Texto != "Rodou às 14:00 · "+c.texto {
			t.Errorf("%s: avisos = %+v", c.nome, av)
		}
		if is := historico(t, a, "c1"); c.nome == "banco fora do ar" && (len(is) != 1 || is[0].Resultado.Motivo != "") {
			t.Errorf("%s: %+v", c.nome, is)
		}
	}
}

func TestTextoSemFraseContaLinhas(t *testing.T) {
	r := Resultado{Ok: true, Todas: []map[string]any{{"a": 1}, {"a": 2}}}
	if s := textoDoResultado(r); s != "2 linhas" {
		t.Errorf("texto = %q", s)
	}
}

func TestAnteriorAindaRodando(t *testing.T) {
	f := novoAmbiente()
	entrou, solta := make(chan struct{}, 1), make(chan struct{})
	f.consultar = func(context.Context, string) (string, error) {
		entrou <- struct{}{}
		<-solta
		return linhasT, nil
	}
	f.por("c1", "Vendas", cada5)
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	a.Passo(dia(26, 10, 0))
	<-entrou
	for s := 30; s <= 300; s += 30 {
		a.Passo(dia(26, 10, 0).Add(time.Duration(s) * time.Second))
	}
	close(solta)
	a.esperarFila()
	is := historico(t, a, "c1")
	if len(is) != 2 || is[0].Tipo != "agendada" || is[1].Tipo != "perda" || is[1].Motivo != "anterior" ||
		is[1].N != 1 || !is[1].Ate.Equal(dia(26, 10, 5)) {
		t.Fatalf("histórico = %+v", is)
	}
	if n := f.chamadas.Load(); n != 1 {
		t.Errorf("Consultar chamado %d vezes", n)
	}
}

func TestFilaUnica(t *testing.T) {
	f := novoAmbiente()
	var dentro, max atomic.Int32
	f.consultar = func(context.Context, string) (string, error) {
		n := dentro.Add(1)
		for {
			m := max.Load()
			if n <= m || max.CompareAndSwap(m, n) {
				break
			}
		}
		defer dentro.Add(-1)
		return linhasT, nil
	}
	ag := map[string]any{"ligado": true, "freq": "diario", "a_cada": 1, "hora": "14:00"}
	f.por("c1", "Vendas", ag)
	f.por("c2", "Compras", ag)
	f.por("c3", "Sem agenda", nil)
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := a.RodarAgora(context.Background(), bancoT, "c3", 0); err != nil {
			t.Error(err)
		}
	}()
	a.Passo(dia(26, 14, 0))
	wg.Wait()
	a.esperarFila()
	if n := f.chamadas.Load(); n != 3 || max.Load() != 1 {
		t.Errorf("chamadas %d, máximo em paralelo %d", n, max.Load())
	}
	for _, id := range []string{"c1", "c2", "c3"} {
		if is := historico(t, a, id); len(is) != 1 {
			t.Errorf("%s: %+v", id, is)
		}
	}
}

func TestConsultaApagadaNaFila(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", cada5)
	a := novoAgendador(t, f)
	a.Passo(dia(26, 10, 0)) // sem executor: fica na fila
	f.apagar("c1")
	ligarExecutor(t, a)
	a.esperarFila()
	if is := historico(t, a, "c1"); len(is) != 0 || f.chamadas.Load() != 0 {
		t.Fatalf("histórico = %+v, chamadas %d", is, f.chamadas.Load())
	}
	if av, _ := a.Avisos(0); len(av) != 0 {
		t.Errorf("avisos = %+v", av)
	}
}

func TestBancoMudouNaFila(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", cada5)
	a := novoAgendador(t, f)
	a.Passo(dia(26, 10, 0))
	f.mu.Lock()
	f.banco = notas.Banco{Motor: "postgres", Host: "outro"}
	f.mu.Unlock()
	ligarExecutor(t, a)
	a.esperarFila()
	if f.chamadas.Load() != 0 {
		t.Fatalf("rodou no outro banco")
	}
	for _, b := range []notas.Banco{bancoT, {Motor: "postgres", Host: "outro"}} {
		if is, _, _ := a.o.Hist.Pagina(notas.Chave(b), "c1", 0, 50); len(is) != 0 {
			t.Errorf("%s: %+v", b.Host, is)
		}
	}
}

func TestApagadaDuranteConsultar(t *testing.T) {
	f := novoAmbiente()
	f.consultar = func(context.Context, string) (string, error) {
		f.apagar("c1")
		return linhasT, nil
	}
	f.por("c1", "Vendas", cada5)
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	a.Passo(dia(26, 10, 0))
	a.esperarFila()
	if is := historico(t, a, "c1"); len(is) != 0 || f.chamadas.Load() != 1 {
		t.Fatalf("histórico = %+v, chamadas %d", is, f.chamadas.Load())
	}
	if av, _ := a.Avisos(0); len(av) != 0 {
		t.Errorf("avisos = %+v", av)
	}
}

func TestVencidoNaFilaViraAnterior(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", cada5)
	a := novoAgendador(t, f)
	a.Passo(dia(26, 10, 0)) // sem executor: fica na fila
	f.relogio(func() time.Time { return dia(26, 10, 6) })
	ligarExecutor(t, a)
	a.esperarFila()
	is := historico(t, a, "c1")
	if len(is) != 1 || is[0].Tipo != "perda" || is[0].Motivo != "anterior" || is[0].N != 1 ||
		!is[0].De.Equal(dia(26, 10, 0)) || !is[0].Ate.Equal(dia(26, 10, 0)) || f.chamadas.Load() != 0 {
		t.Fatalf("histórico = %+v, chamadas %d", is, f.chamadas.Load())
	}
	if av, _ := a.Avisos(0); len(av) != 0 {
		t.Errorf("avisos = %+v", av)
	}
}

func TestAtrasoCurtoViraDormindo(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", map[string]any{"ligado": true, "freq": "diario", "a_cada": 1, "hora": "07:30"})
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	a.Passo(dia(26, 7, 29).Add(50 * time.Second))
	a.Passo(dia(26, 7, 31).Add(time.Second)) // 71 s depois: menos que 3 ticks
	a.esperarFila()
	is := historico(t, a, "c1")
	if len(is) != 1 || is[0].Motivo != "dormindo" || !is[0].Ate.Equal(dia(26, 7, 30)) || f.chamadas.Load() != 0 {
		t.Fatalf("histórico = %+v", is)
	}
}

// Relógio de verdade: o monotônico para quando o computador dorme, e a parede anda. As
// decisões seguem a parede, e nada guardado leva a leitura monotônica.
func TestRelogioDeParede(t *testing.T) {
	t1 := time.Now()
	slot := t1.Add(time.Hour)
	f := novoAmbiente()
	f.relogio(time.Now)
	f.por("c1", "Vendas", map[string]any{"ligado": true, "freq": "diario", "a_cada": 1, "hora": slot.Format("15:04")})
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	a.Passo(t1)
	a.Passo(t1.Add(30 * time.Second))
	if strings.Contains(a.ultimo.String(), "m=") {
		t.Errorf("ultimo com monotônico: %v", a.ultimo)
	}
	a.Passo(time.Now().Round(0).Add(2 * time.Hour)) // acordou: a parede pulou 2 h
	a.esperarFila()
	is := historico(t, a, "c1")
	if len(is) != 1 || is[0].Motivo != "dormindo" || is[0].N != 1 || f.chamadas.Load() != 0 {
		t.Fatalf("histórico = %+v, chamadas %d", is, f.chamadas.Load())
	}
	e, err := a.RodarAgora(context.Background(), bancoT, "c1", 0)
	if err != nil || strings.Contains(e.Em.String(), "m=") || strings.Contains(e.Ref.String(), "m=") {
		t.Errorf("manual: em %v ref %v, %v", e.Em, e.Ref, err)
	}
	if agoraDe(t1) != t1.Round(0) || strings.Contains(agoraDe(t1).String(), "m=") {
		t.Errorf("agoraDe = %v", agoraDe(t1))
	}
}

func TestPerdaQueFalhaNaoAvancaAberta(t *testing.T) {
	f := novoAmbiente()
	ag := map[string]any{"ligado": true, "freq": "diario", "a_cada": 1, "hora": "07:00"}
	f.por("c1", "Vendas", ag)
	f.por("c2", "Compras", ag)
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	chave := notas.Chave(bancoT)
	_ = a.o.Hist.MarcarAberta(chave, dia(25, 20, 0))
	destravar := travarAnexar(t, a.o.Hist, "c1")
	a.Passo(dia(26, 8, 0))
	a.esperarFila()
	if b, _, _ := a.o.Hist.Aberta(chave); !b.Equal(dia(25, 20, 0)) {
		t.Errorf("aberta avançou: %v", b)
	}
	if av, _ := a.Avisos(0); len(av) != 0 {
		t.Errorf("avisos = %+v", av)
	}
	destravar()
	a.Passo(dia(26, 8, 0).Add(30 * time.Second))
	a.esperarFila()
	for _, id := range []string{"c1", "c2"} {
		if is := historico(t, a, id); len(is) != 1 || is[0].Motivo != "fechada" {
			t.Errorf("%s: %+v", id, is)
		}
	}
	if b, _, _ := a.o.Hist.Aberta(chave); !b.Equal(dia(26, 8, 0).Add(30 * time.Second)) {
		t.Errorf("aberta = %v", b)
	}
	if av, _ := a.Avisos(0); len(av) != 1 || av[0].Texto != "2 execuções não rodaram enquanto a IA estava fechada" {
		t.Errorf("avisos = %+v", av)
	}
}

func TestRodarAgoraPerda(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", cada5)
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	chave := notas.Chave(bancoT)
	de, ate := dia(26, 7, 0), dia(26, 7, 55)
	p, err := a.o.Hist.Anexar(chave, "c1", Execucao{Tipo: "perda", Ref: ate, Em: dia(26, 8, 0), Motivo: "fechada", De: &de, Ate: &ate, N: 12})
	if err != nil {
		t.Fatal(err)
	}
	e, err := a.RodarAgora(context.Background(), bancoT, "c1", p.Seq)
	if err != nil {
		t.Fatal(err)
	}
	if e.Tipo != "manual" || !e.Ref.Equal(ate) || e.Resultado == nil || !e.Resultado.Ok {
		t.Fatalf("execução = %+v", e)
	}
	is := historico(t, a, "c1")
	if len(is) != 2 || is[1].Rodada == nil || !is[1].Rodada.Equal(e.Em) {
		t.Fatalf("histórico = %+v", is)
	}
	if av, _ := a.Avisos(0); len(av) != 0 {
		t.Errorf("avisos = %+v", av)
	}
	if _, err := a.RodarAgora(context.Background(), bancoT, "c1", e.Seq); err != ErrSemPerda {
		t.Errorf("seq de manual: %v", err)
	}
	if _, err := a.RodarAgora(context.Background(), bancoT, "nada", 0); err != ErrSemConsulta {
		t.Errorf("consulta que não existe: %v", err)
	}
	// Sem perda: ref = agora.
	e, err = a.RodarAgora(context.Background(), bancoT, "c1", 0)
	if err != nil || !e.Ref.Equal(a.o.Agora()) {
		t.Errorf("sem perda: %+v, %v", e, err)
	}
}

func TestRodarAgoraParado(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", nil)
	a := novoAgendador(t, f)
	ctx, cancel := context.WithCancel(context.Background())
	fim := make(chan struct{})
	go func() { a.Rodar(ctx); close(fim) }()
	cancel()
	<-fim
	if _, err := a.RodarAgora(context.Background(), bancoT, "c1", 0); err == nil {
		t.Error("rodou com o agendador parado")
	}
}

func TestAgendaDesligadaNaoRoda(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", map[string]any{"ligado": false, "freq": "minutos", "a_cada": 5})
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	_ = a.o.Hist.MarcarAberta(notas.Chave(bancoT), dia(25, 8, 0))
	a.Passo(dia(26, 10, 0))
	a.Passo(dia(26, 10, 5))
	a.esperarFila()
	if is := historico(t, a, "c1"); len(is) != 0 || f.chamadas.Load() != 0 {
		t.Fatalf("histórico = %+v", is)
	}
	if av, _ := a.Avisos(0); len(av) != 0 {
		t.Errorf("avisos = %+v", av)
	}
}

func TestSemBancoSoMarcaAberta(t *testing.T) {
	f := novoAmbiente()
	f.conectado = false
	f.por("c1", "Vendas", cada5)
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	a.Passo(dia(26, 10, 0))
	a.esperarFila()
	b, q, err := a.o.Hist.Aberta(notas.Chave(bancoT))
	if err != nil || !b.IsZero() || !q.Equal(dia(26, 10, 0)) || f.chamadas.Load() != 0 {
		t.Fatalf("banco %v qualquer %v err %v chamadas %d", b, q, err, f.chamadas.Load())
	}
}

func TestNovidadesEAvisos(t *testing.T) {
	f := novoAmbiente()
	ag := map[string]any{"ligado": true, "freq": "diario", "a_cada": 1, "hora": "14:00"}
	f.por("c1", "Vendas", ag)
	f.por("c2", "Compras", nil)
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	a.Passo(dia(26, 14, 0))
	a.esperarFila()
	if _, err := a.RodarAgora(context.Background(), bancoT, "c2", 0); err != nil {
		t.Fatal(err)
	}
	nv := a.Novidades(bancoT)
	if len(nv) != 1 || nv[0] != (Novidade{ConsultaId: "c1", Nome: "Vendas", N: 1}) {
		t.Errorf("novidades = %+v", nv)
	}
	for d := 27; d <= 31; d++ {
		a.Passo(dia(d, 14, 0))
		a.esperarFila()
	}
	av, ultimo := a.Avisos(0)
	if len(av) != 6 || ultimo != 6 {
		t.Fatalf("avisos = %+v, %d", av, ultimo)
	}
	if av, u := a.Avisos(4); len(av) != 2 || av[0].Seq != 5 || u != 6 {
		t.Errorf("desde 4: %+v %d", av, u)
	}
	if !strings.HasPrefix(av[0].Texto, "Rodou às 14:00 · ") {
		t.Errorf("texto = %q", av[0].Texto)
	}
}

func TestAvisosGuardaCem(t *testing.T) {
	a := NovoAgendador(Opcoes{})
	for i := 0; i < 150; i++ {
		a.avisar("c1", "t", "x")
	}
	av, ultimo := a.Avisos(0)
	if len(av) != 100 || av[0].Seq != 51 || ultimo != 150 {
		t.Errorf("len %d primeiro %d último %d", len(av), av[0].Seq, ultimo)
	}
}

var as14 = map[string]any{"ligado": true, "freq": "diario", "a_cada": 1, "hora": "14:00"}

func TestEsperaOBancoFicarPronto(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", as14)
	var n atomic.Int32
	f.pronto = func() bool { return n.Add(1) > 3 } // toolbox ainda subindo nas 3 primeiras
	a := novoAgendador(t, f)
	a.espera = time.Millisecond
	ligarExecutor(t, a)
	a.Passo(dia(26, 14, 0).Add(10 * time.Second))
	a.esperarFila()
	is := historico(t, a, "c1")
	if len(is) != 1 || is[0].Resultado == nil || !is[0].Resultado.Ok || f.chamadas.Load() != 1 || n.Load() < 4 {
		t.Fatalf("histórico = %+v, chamadas %d, pronto? %d vezes", is, f.chamadas.Load(), n.Load())
	}
}

func TestBancoNaoFicouPronto(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", as14)
	f.pronto = func() bool {
		f.relogio(func() time.Time { return dia(26, 14, 2) }) // passou a folga esperando
		return false
	}
	a := novoAgendador(t, f)
	a.espera = time.Millisecond
	ligarExecutor(t, a)
	a.Passo(dia(26, 14, 0).Add(10 * time.Second))
	a.esperarFila()
	is := historico(t, a, "c1")
	if len(is) != 1 || is[0].Tipo != "agendada" || is[0].Resultado == nil || is[0].Resultado.Ok ||
		is[0].Resultado.Erro != "O banco ainda não estava pronto." || f.chamadas.Load() != 0 {
		t.Fatalf("histórico = %+v, chamadas %d", is, f.chamadas.Load())
	}
}

func TestSemConexaoViraTextoSimples(t *testing.T) {
	f := novoAmbiente()
	f.consultar = func(context.Context, string) (string, error) {
		return "", fmt.Errorf("%w: dial tcp 127.0.0.1:6000: connect: connection refused", ErrSemConexao)
	}
	f.por("c1", "Vendas", as14)
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	a.Passo(dia(26, 14, 0))
	a.esperarFila()
	is := historico(t, a, "c1")
	if len(is) != 1 || is[0].Resultado == nil || is[0].Resultado.Erro != "Não consegui falar com o banco." {
		t.Fatalf("histórico = %+v", is)
	}
}

func TestErroPassageiroAoRelerAindaGrava(t *testing.T) {
	f := novoAmbiente()
	f.consultar = func(context.Context, string) (string, error) {
		f.mu.Lock()
		f.lerErr = errors.New("notas: arquivo ocupado")
		f.mu.Unlock()
		return linhasT, nil
	}
	f.por("c1", "Vendas", as14)
	a := novoAgendador(t, f)
	ligarExecutor(t, a)
	a.Passo(dia(26, 14, 0))
	a.esperarFila()
	f.lerErr = nil
	is := historico(t, a, "c1")
	if len(is) != 1 || is[0].Resultado == nil || !is[0].Resultado.Ok {
		t.Fatalf("histórico = %+v", is)
	}
	if av, _ := a.Avisos(0); len(av) != 1 || av[0].Titulo != "IA de Bolso · Vendas" {
		t.Errorf("avisos = %+v", av)
	}
}
