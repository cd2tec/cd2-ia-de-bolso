package agenda

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// limite2000: soma de total > 2000. linhasCom(2500) dispara; o linhasT (1244,5) não.
var limite2000 = map[string]any{"coluna": "total", "agregacao": "soma", "operador": "maior", "valor": 2000}

// comResumo liga o Resumir no agendador com a resposta dada e conta as chamadas.
func comResumo(t *testing.T, a *Agendador, resp func(ctx context.Context, p PedidoResumo) (string, error)) *atomic.Int32 {
	t.Helper()
	n := &atomic.Int32{}
	a.o.Resumir = func(ctx context.Context, p PedidoResumo) (string, error) {
		n.Add(1)
		return resp(ctx, p)
	}
	return n
}

func respostaModelo(frases ...string) string {
	b, _ := json.Marshal(map[string]any{"linhas": frases})
	return string(b)
}

func TestResumoQuandoDispara(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", agendaCom(limite2000))
	f.responder(linhasCom(2500), nil)
	a := novoAgendador(t, f)
	var pedido PedidoResumo
	n := comResumo(t, a, func(_ context.Context, p PedidoResumo) (string, error) {
		pedido = p
		return respostaModelo("A soma de total foi 2.500, acima de 2.000.", "Só a loja A vendeu."), nil
	})
	ligarExecutor(t, a)

	a.Passo(dia(26, 8, 0))
	a.esperarFila()
	if n.Load() != 1 {
		t.Fatalf("chamadas ao modelo = %d", n.Load())
	}
	if len(pedido.Mensagens) != 2 || !strings.Contains(pedido.Mensagens[1]["content"], "Alerta: A soma de total = 2.500, acima de 2.000.") || len(pedido.Schema) == 0 {
		t.Fatalf("pedido = %+v", pedido)
	}
	is := historico(t, a, "c1")
	r := is[0].Resumo
	if r == nil || r.Falha != "" || len(r.Linhas) != 2 {
		t.Fatalf("resumo = %+v", r)
	}
	av, _ := a.Avisos(0)
	if len(av) != 1 || av[0].Texto != "Rodou às 08:00 · A soma de total foi 2.500, acima de 2.000.\nSó a loja A vendeu." {
		t.Fatalf("avisos = %+v", av)
	}
}

func TestResumoNumeroInventadoCaiNoAvisoDeSempre(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", agendaCom(limite2000))
	f.responder(linhasCom(2500), nil)
	a := novoAgendador(t, f)
	comResumo(t, a, func(context.Context, PedidoResumo) (string, error) {
		return respostaModelo("O total subiu 37% em relação a ontem."), nil
	})
	ligarExecutor(t, a)

	a.Passo(dia(26, 8, 0))
	a.esperarFila()
	r := historico(t, a, "c1")[0].Resumo
	if r == nil || r.Falha != "numero" || len(r.Linhas) != 0 || !strings.Contains(r.Detalhe, "37") {
		t.Fatalf("resumo = %+v", r)
	}
	av, _ := a.Avisos(0)
	if len(av) != 1 || av[0].Texto != "Rodou às 08:00 · A soma de total = 2.500, acima de 2.000." {
		t.Fatalf("avisos = %+v", av)
	}
}

func TestResumoFalhaDoModeloNaoBloqueiaOAviso(t *testing.T) {
	casos := map[string]struct {
		resp  func(ctx context.Context, p PedidoResumo) (string, error)
		falha string
	}{
		"erro":    {func(context.Context, PedidoResumo) (string, error) { return "", errors.New("conexão recusada") }, "modelo"},
		"formato": {func(context.Context, PedidoResumo) (string, error) { return "Claro, aqui vai o resumo.", nil }, "formato"},
		"prazo": {func(ctx context.Context, _ PedidoResumo) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		}, "prazo"},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			f := novoAmbiente()
			f.por("c1", "Vendas", agendaCom(limite2000))
			f.responder(linhasCom(2500), nil)
			a := novoAgendador(t, f)
			comResumo(t, a, c.resp)
			a.o.PrazoResumo = 50 * time.Millisecond
			ligarExecutor(t, a)

			a.Passo(dia(26, 8, 0))
			a.esperarFila()
			is := historico(t, a, "c1")
			if len(is) != 1 || is[0].Resumo == nil || is[0].Resumo.Falha != c.falha {
				t.Fatalf("histórico = %+v resumo = %+v", is, is[0].Resumo)
			}
			av, _ := a.Avisos(0)
			if len(av) != 1 || !strings.HasSuffix(av[0].Texto, "A soma de total = 2.500, acima de 2.000.") {
				t.Fatalf("avisos = %+v", av)
			}
		})
	}
}

func TestResumoSoQuandoDispara(t *testing.T) {
	// Não disparou: sem chamada. Sem condição: avisa como antes, sem chamada.
	f := novoAmbiente()
	f.por("c1", "Vendas", agendaCom(limite2000))
	f.por("c2", "Sem regra", map[string]any{"ligado": true, "freq": "minutos", "a_cada": 5})
	a := novoAgendador(t, f)
	n := comResumo(t, a, func(context.Context, PedidoResumo) (string, error) { return respostaModelo("x"), nil })
	ligarExecutor(t, a)

	a.Passo(dia(26, 8, 0))
	a.esperarFila()
	if n.Load() != 0 {
		t.Fatalf("chamou o modelo %d vez(es) sem disparar", n.Load())
	}
	if r := historico(t, a, "c1")[0].Resumo; r != nil {
		t.Fatalf("resumo sem disparo: %+v", r)
	}
	if av, _ := a.Avisos(0); len(av) != 1 || av[0].ConsultaId != "c2" {
		t.Fatalf("avisos = %+v", av)
	}
}

func TestResumoRodarAgoraNaoChama(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", agendaCom(limite2000))
	f.responder(linhasCom(2500), nil)
	a := novoAgendador(t, f)
	n := comResumo(t, a, func(context.Context, PedidoResumo) (string, error) { return respostaModelo("x"), nil })
	ligarExecutor(t, a)

	if _, err := a.RodarAgora(context.Background(), bancoT, "c1", 0); err != nil {
		t.Fatal(err)
	}
	if n.Load() != 0 {
		t.Fatalf("Rodar agora chamou o modelo")
	}
}

func TestNovoResumidor(t *testing.T) {
	var corpo map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		json.NewDecoder(r.Body).Decode(&corpo)
		w.Write([]byte(`{"choices":[{"message":{"content":"{\"linhas\":[\"ok\"]}"}}]}`))
	}))
	defer srv.Close()
	res := NovoResumidor(srv.URL + "/v1/chat/completions")
	p := PedidoResumo{Mensagens: []map[string]string{{"role": "user", "content": "oi"}}, Schema: json.RawMessage(`{"type":"object"}`), MaxTokens: 300}
	got, err := res(context.Background(), p)
	if err != nil || got != `{"linhas":["ok"]}` {
		t.Fatalf("got %q, %v", got, err)
	}
	rf, _ := corpo["response_format"].(map[string]any)
	kw, _ := corpo["chat_template_kwargs"].(map[string]any)
	if kw["enable_thinking"] != false {
		t.Fatalf("thinking não foi desligado: %+v", corpo["chat_template_kwargs"])
	}
	if corpo["temperature"] != float64(0) || corpo["max_tokens"] != float64(300) || rf["type"] != "json_schema" {
		t.Fatalf("corpo = %+v", corpo)
	}

	ruim := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "x", 503) }))
	defer ruim.Close()
	if _, err := NovoResumidor(ruim.URL)(context.Background(), p); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("err = %v", err)
	}
}

func TestResumoSegundaTentativaComOMotivo(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", agendaCom(limite2000))
	f.responder(linhasCom(2500), nil)
	a := novoAgendador(t, f)
	var pedidos []PedidoResumo
	n := comResumo(t, a, func(_ context.Context, p PedidoResumo) (string, error) {
		pedidos = append(pedidos, p)
		if len(pedidos) == 1 {
			return respostaModelo("O total subiu 37%."), nil
		}
		return respostaModelo("A soma de total foi 2.500, acima de 2.000."), nil
	})
	ligarExecutor(t, a)

	a.Passo(dia(26, 8, 0))
	a.esperarFila()
	if n.Load() != 2 {
		t.Fatalf("chamadas = %d, quero 2", n.Load())
	}
	seg := pedidos[1].Mensagens
	if len(seg) != len(pedidos[0].Mensagens)+2 || seg[len(seg)-2]["role"] != "assistant" ||
		!strings.Contains(seg[len(seg)-1]["content"], "número 37 não está nos dados") {
		t.Fatalf("segunda tentativa = %+v", seg)
	}
	r := historico(t, a, "c1")[0].Resumo
	if r == nil || r.Falha != "" || r.Tentativas != 2 || len(r.Linhas) != 1 {
		t.Fatalf("resumo = %+v", r)
	}
	if av, _ := a.Avisos(0); len(av) != 1 || av[0].Texto != "Rodou às 08:00 · A soma de total foi 2.500, acima de 2.000." {
		t.Fatalf("avisos = %+v", av)
	}
}

func TestResumoLimiteDeTentativas(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", agendaCom(limite2000))
	f.responder(linhasCom(2500), nil)
	a := novoAgendador(t, f)
	n := comResumo(t, a, func(context.Context, PedidoResumo) (string, error) { return "não é json", nil })
	ligarExecutor(t, a)

	a.Passo(dia(26, 8, 0))
	a.esperarFila()
	if n.Load() != 2 {
		t.Fatalf("chamadas = %d, quero 2 (o limite)", n.Load())
	}
	r := historico(t, a, "c1")[0].Resumo
	if r == nil || r.Falha != "formato" || r.Tentativas != 2 {
		t.Fatalf("resumo = %+v", r)
	}
	if av, _ := a.Avisos(0); len(av) != 1 || av[0].Texto != "Rodou às 08:00 · A soma de total = 2.500, acima de 2.000." {
		t.Fatalf("avisos = %+v", av)
	}
}

func TestResumoErroDoModeloNaoTentaDeNovo(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", agendaCom(limite2000))
	f.responder(linhasCom(2500), nil)
	a := novoAgendador(t, f)
	n := comResumo(t, a, func(context.Context, PedidoResumo) (string, error) { return "", errors.New("conexão recusada") })
	ligarExecutor(t, a)

	a.Passo(dia(26, 8, 0))
	a.esperarFila()
	if r := historico(t, a, "c1")[0].Resumo; n.Load() != 1 || r == nil || r.Falha != "modelo" || r.Tentativas != 1 {
		t.Fatalf("chamadas = %d, resumo = %+v", n.Load(), r)
	}
}

func TestResumoPrazoValeParaTodasAsTentativas(t *testing.T) {
	f := novoAmbiente()
	f.por("c1", "Vendas", agendaCom(limite2000))
	f.responder(linhasCom(2500), nil)
	a := novoAgendador(t, f)
	a.o.PrazoResumo = 80 * time.Millisecond
	n := comResumo(t, a, func(ctx context.Context, _ PedidoResumo) (string, error) {
		select {
		case <-time.After(50 * time.Millisecond): // cada tentativa cabe, as duas juntas não
			return "não é json", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})
	ligarExecutor(t, a)

	a.Passo(dia(26, 8, 0))
	a.esperarFila()
	r := historico(t, a, "c1")[0].Resumo
	if r == nil || r.Falha != "prazo" || n.Load() != 2 {
		t.Fatalf("chamadas = %d, resumo = %+v", n.Load(), r)
	}
	if av, _ := a.Avisos(0); len(av) != 1 || !strings.HasSuffix(av[0].Texto, "acima de 2.000.") {
		t.Fatalf("avisos = %+v", av)
	}
}
