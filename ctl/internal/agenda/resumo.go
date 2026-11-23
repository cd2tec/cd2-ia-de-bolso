package agenda

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// PrazoResumo é quanto o aviso espera o resumo do modelo. Passou disso, o aviso sai como antes
// (o texto da condição). O modelo atende um pedido por vez: se a pessoa estiver no chat, o resumo
// espera na fila e pode estourar o prazo.
const PrazoResumo = 25 * time.Second

// Resumo é o que ficou do resumo de uma execução que disparou. Linhas quando o modelo respondeu e
// a conferência passou; senão Falha: "prazo" | "modelo" | "formato" | "numero". Ms: quanto levou.
type Resumo struct {
	Linhas  []string `json:"linhas,omitempty"`
	Falha   string   `json:"falha,omitempty"`
	Detalhe string   `json:"detalhe,omitempty"`
	Ms      int64    `json:"ms"`
}

// NovoResumidor chama o modelo do harness (llamafile, API compatível com a da OpenAI) em url
// (http://127.0.0.1:P/v1/chat/completions) com a saída presa ao JSON schema do pedido.
func NovoResumidor(url string) func(ctx context.Context, p PedidoResumo) (string, error) {
	cli := &http.Client{}
	return func(ctx context.Context, p PedidoResumo) (string, error) {
		corpo, err := json.Marshal(map[string]any{
			"model": "cd2", "messages": p.Mensagens, "stream": false, "temperature": 0, "max_tokens": p.MaxTokens,
			"response_format": map[string]any{"type": "json_schema",
				"json_schema": map[string]any{"name": "resumo", "strict": true, "schema": p.Schema}},
		})
		if err != nil {
			return "", err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(corpo))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := cli.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return "", err
		}
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("modelo respondeu HTTP %d", resp.StatusCode)
		}
		var r struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(b, &r); err != nil || len(r.Choices) == 0 {
			return "", errors.New("resposta do modelo ilegível")
		}
		return r.Choices[0].Message.Content, nil
	}
}

// resumir pede e confere o resumo de uma execução cuja condição disparou. nil quando não há o que
// resumir (sem Resumir configurado, ou a condição não disparou) ou quando o ctl está fechando.
func (a *Agendador) resumir(ctx context.Context, c consulta, res Resultado, j *Julgamento) *Resumo {
	if a.o.Resumir == nil || j == nil || !j.Disparou || !j.Aviso {
		return nil
	}
	p, err := a.o.Motor.ResumoPedido(c.raw, res, j)
	if err != nil || p == nil {
		return nil
	}
	prazo := a.o.PrazoResumo
	if prazo <= 0 {
		prazo = PrazoResumo
	}
	ini := time.Now()
	rctx, cancel := context.WithTimeout(ctx, prazo)
	texto, err := a.o.Resumir(rctx, *p)
	estourou := errors.Is(rctx.Err(), context.DeadlineExceeded) // antes do cancel, que também marca Err
	cancel()
	r := &Resumo{Ms: time.Since(ini).Milliseconds()}
	switch {
	case ctx.Err() != nil:
		return nil
	case err != nil && estourou:
		r.Falha = "prazo"
		return r
	case err != nil:
		r.Falha, r.Detalhe = "modelo", err.Error()
		return r
	}
	conf, err := a.o.Motor.ResumoConferir(texto, c.raw, res, j)
	switch {
	case err != nil:
		r.Falha, r.Detalhe = "formato", err.Error()
	case !conf.Ok:
		r.Falha, r.Detalhe = conf.Motivo, conf.Detalhe
	default:
		r.Linhas = conf.Linhas
	}
	return r
}
