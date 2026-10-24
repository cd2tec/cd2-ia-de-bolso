package agenda

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const webDir = "../../../web"

func novoMotor(t *testing.T) *Motor {
	t.Helper()
	m, err := NovoMotor(webDir)
	if err != nil {
		t.Fatalf("NovoMotor: %v", err)
	}
	return m
}

// consultaComData monta uma consulta com um parâmetro de data no literal '2026-10-01'.
func consultaComData(t *testing.T, agenda any) json.RawMessage {
	t.Helper()
	sql := "SELECT loja, SUM(total) AS total FROM vendas WHERE dia >= '2026-10-01' GROUP BY loja"
	i := strings.Index(sql, "2026-10-01")
	c := map[string]any{
		"nome": "Vendas", "sql": sql, "visual": "barra", "x": "loja", "y": "total",
		"params": []any{map[string]any{"nome": "dia", "tipo": "data", "valor": "2026-10-01", "inicio": i, "fim": i + 10}},
	}
	if agenda != nil {
		c["agenda"] = agenda
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPrepararUsaPeriodo(t *testing.T) {
	m := novoMotor(t)
	ag := map[string]any{"ligado": true, "freq": "diario", "hora": "07:00",
		"params": []any{map[string]any{"modo": "periodo", "periodo": "hoje", "ponta": "inicio"}}}
	p, err := m.Preparar(consultaComData(t, ag), time.Date(2026, 10, 24, 7, 0, 0, 0, time.Local), "postgres")
	if err != nil {
		t.Fatal(err)
	}
	if p.Erro != "" || !strings.Contains(p.SQL, "'2026-10-24'") || strings.Contains(p.SQL, "2026-10-01") {
		t.Fatalf("Preparo = %+v", p)
	}
	if len(p.Valores) != 1 || p.Valores[0] != "2026-10-24" {
		t.Errorf("Valores = %v", p.Valores)
	}
}

func TestPrepararSemAgendaUsaValorSalvo(t *testing.T) {
	m := novoMotor(t)
	p, err := m.Preparar(consultaComData(t, nil), time.Date(2026, 10, 24, 7, 0, 0, 0, time.Local), "postgres")
	if err != nil {
		t.Fatal(err)
	}
	if p.Erro != "" || !strings.Contains(p.SQL, "'2026-10-01'") {
		t.Fatalf("Preparo = %+v", p)
	}
}

func TestPrepararSqlServerGanhaOffset(t *testing.T) {
	m := novoMotor(t)
	c := json.RawMessage(`{"sql":"SELECT a FROM t ORDER BY a","params":[]}`)
	p, err := m.Preparar(c, time.Now(), "sqlserver")
	if err != nil {
		t.Fatal(err)
	}
	if p.SQL != "SELECT a FROM t ORDER BY a OFFSET 0 ROWS" {
		t.Errorf("SQL = %q", p.SQL)
	}
}

func TestPrepararRecusaSqlNaoSelect(t *testing.T) {
	m := novoMotor(t)
	p, err := m.Preparar(json.RawMessage(`{"sql":"DELETE FROM x","params":[]}`), time.Now(), "postgres")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p.Erro, "Consulta recusada") || p.SQL != "" {
		t.Errorf("Preparo = %+v", p)
	}
}

func TestPrepararPeriodoInvalido(t *testing.T) {
	m := novoMotor(t)
	ag := map[string]any{"params": []any{map[string]any{"modo": "dia_semana", "formato": "sigla"}}}
	p, err := m.Preparar(consultaComData(t, ag), time.Now(), "postgres")
	if err != nil {
		t.Fatal(err)
	}
	if p.Erro != "dia: período inválido para este parâmetro." {
		t.Errorf("Erro = %q", p.Erro)
	}
}

func TestHorariosIgualAoNode(t *testing.T) {
	m := novoMotor(t)
	ag := map[string]any{"ligado": true, "freq": "horas", "a_cada": 1,
		"janela": map[string]any{"de": "08:00", "ate": "20:00"}, "dias": []int{1, 2, 3, 4, 5, 6}}
	de := time.Date(2026, 10, 24, 6, 0, 0, 0, time.Local) // sábado
	ate := time.Date(2026, 10, 26, 9, 0, 0, 0, time.Local)
	h, err := m.Horarios(consultaComData(t, ag), de, ate)
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 15 {
		t.Fatalf("len = %d, quero 15: %v", len(h), h)
	}
	if !h[0].Equal(time.Date(2026, 10, 24, 8, 0, 0, 0, time.Local)) || !h[14].Equal(ate) {
		t.Errorf("primeiro %v, último %v", h[0], h[14])
	}
}

func TestHorariosSemAgendaOuDesligada(t *testing.T) {
	m := novoMotor(t)
	de, ate := time.Now(), time.Now().Add(48*time.Hour)
	for _, ag := range []any{nil, map[string]any{"ligado": false, "freq": "horas", "a_cada": 1}} {
		h, err := m.Horarios(consultaComData(t, ag), de, ate)
		if err != nil || len(h) != 0 {
			t.Errorf("agenda %v: %v, %v", ag, h, err)
		}
	}
}

func TestResultadoFrase(t *testing.T) {
	m := novoMotor(t)
	r, err := m.Resultado(consultaComData(t, nil), `[{"loja":"A","total":1234.5},{"loja":"B","total":10}]`, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Mesma frase que o ask.js produz no Node para esta entrada.
	if !r.Ok || r.Visual != "barra" || r.Titulo != "Vendas" || r.Frase != "Maior: A, 1.234,5. Total: 1.244,5 em 2 categorias." {
		t.Fatalf("Resultado = %+v", r)
	}
	if len(r.Colunas) != 2 || r.Colunas[0] != "loja" || len(r.Todas) != 2 || r.Todas[0]["total"] != 1234.5 {
		t.Errorf("Resultado = %+v", r)
	}
}

func TestResultadoVazio(t *testing.T) {
	m := novoMotor(t)
	r, err := m.Resultado(consultaComData(t, nil), "[]", nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Ok || r.Motivo != "vazio" {
		t.Errorf("Resultado = %+v", r)
	}
}

func TestResultadoErroDoBanco(t *testing.T) {
	m := novoMotor(t)
	cases := map[string]string{
		"ERRO: relation \"x\" does not exist":                "sql",
		"ERRO: canceling statement due to statement timeout": "timeout",
		"isto não é json":                                    "sql",
	}
	for texto, motivo := range cases {
		r, err := m.Resultado(consultaComData(t, nil), texto, nil)
		if err != nil || r.Ok || r.Motivo != motivo {
			t.Errorf("%q: %+v, %v; quero motivo %q", texto, r, err, motivo)
		}
	}
}

func TestIntlIgualAoNode(t *testing.T) {
	m := novoMotor(t)
	// Saídas de new Intl.NumberFormat('pt-BR', { maximumFractionDigits:2 }).format(n) no Node 22.
	cases := []struct{ n, want string }{
		{"1234.5", "1.234,5"}, {"1244.5", "1.244,5"}, {"0.125", "0,13"},
		{"-1234567.891", "-1.234.567,89"}, {"1.005", "1,01"}, {"0", "0"},
		{"-0.001", "-0"}, {"1e21", "1.000.000.000.000.000.000.000"}, {"1234", "1.234"},
		{"12.3456", "12,35"}, {"999.999", "1.000"}, {"0.1", "0,1"},
	}
	for _, c := range cases {
		v, err := m.vm.RunString("new Intl.NumberFormat('pt-BR', { maximumFractionDigits:2 }).format(" + c.n + ")")
		if err != nil {
			t.Fatal(err)
		}
		if got := v.String(); got != c.want {
			t.Errorf("format(%s) = %q, quero %q", c.n, got, c.want)
		}
	}
}

func TestMotorSemWeb(t *testing.T) {
	if _, err := NovoMotor("/nao/existe"); err == nil {
		t.Fatal("NovoMotor sem a pasta web deveria falhar")
	}
}
