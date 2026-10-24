package agenda

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// Espelha web/mcp.js: mesma versão de protocolo e mesmo clientInfo.
const (
	protocoloMCP = "2025-06-18"
	ferramentaBD = "db_read_only_query"
)

// ErrPrazo: o prazo do contexto de quem chamou estourou.
var ErrPrazo = errors.New("prazo da consulta estourou")

// ErrSemConexao: o pedido nem chegou ao toolbox (conexão recusada, erro de rede).
var ErrSemConexao = errors.New("sem conexão com o toolbox")

// errSessao: o toolbox não reconhece a sessão (404/400), provavelmente reiniciou.
type errSessao struct{ status int }

func (e errSessao) Error() string { return fmt.Sprintf("MCP respondeu HTTP %d", e.status) }

// Cliente fala MCP (Streamable HTTP) com o toolbox. Seguro para uso concorrente.
type Cliente struct {
	url  string
	http *http.Client

	mu  sync.Mutex
	sid string // Mcp-Session-Id, vazio se o servidor não mandou
	ok  bool   // initialize já feito
	ver string // versão de protocolo combinada no initialize; vai em todo pedido depois dele
	n   int
}

func NovoCliente(url string) *Cliente {
	return &Cliente{url: url, http: &http.Client{}}
}

// Consultar roda o SQL no db_read_only_query e devolve o texto do resultado.
// Erro do tool vira "ERRO: msg", como o web/mcp.js (o parseRows trata).
func (c *Cliente) Consultar(ctx context.Context, sql string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	texto, err := c.consultar(ctx, sql)
	var es errSessao
	if errors.As(err, &es) {
		c.reiniciar()
		texto, err = c.consultar(ctx, sql)
	}
	if err != nil && ctx.Err() != nil {
		return "", ErrPrazo
	}
	return texto, err
}

func (c *Cliente) reiniciar() { c.sid, c.ok, c.ver = "", false, "" }

func (c *Cliente) consultar(ctx context.Context, sql string) (string, error) {
	if !c.ok {
		if err := c.iniciar(ctx); err != nil {
			return "", err
		}
	}
	res, err := c.rpc(ctx, "tools/call", map[string]any{
		"name":      ferramentaBD,
		"arguments": map[string]any{"sql": sql},
	}, false)
	if err != nil {
		return "", err
	}
	var r struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(res, &r); err != nil {
		return "", fmt.Errorf("resposta MCP inválida: %w", err)
	}
	partes := make([]string, 0, len(r.Content))
	for _, ct := range r.Content {
		if ct.Type == "text" {
			partes = append(partes, ct.Text)
		} else {
			partes = append(partes, "["+ct.Type+"]")
		}
	}
	texto := strings.Join(partes, "\n")
	if r.IsError {
		texto = "ERRO: " + texto
	}
	return texto, nil
}

func (c *Cliente) iniciar(ctx context.Context) error {
	res, err := c.rpc(ctx, "initialize", map[string]any{
		"protocolVersion": protocoloMCP,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "cd2-ctl", "version": "0"},
	}, false)
	if err != nil {
		return err
	}
	var r struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(res, &r)
	if r.ProtocolVersion != protocoloMCP {
		return fmt.Errorf("o servidor MCP respondeu com a versão de protocolo %q; esperado %q", r.ProtocolVersion, protocoloMCP)
	}
	c.ver = r.ProtocolVersion // como o web/mcp.js: já o notifications/initialized leva o cabeçalho
	if _, err := c.rpc(ctx, "notifications/initialized", nil, true); err != nil {
		return err
	}
	c.ok = true
	return nil
}

// rpc manda uma mensagem JSON-RPC; com notif não espera resultado.
func (c *Cliente) rpc(ctx context.Context, metodo string, params any, notif bool) (json.RawMessage, error) {
	msg := map[string]any{"jsonrpc": "2.0", "method": metodo}
	if params != nil {
		msg["params"] = params
	}
	id := 0
	if !notif {
		c.n++
		id = c.n
		msg["id"] = id
	}
	corpo, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(corpo))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if c.sid != "" {
		req.Header.Set("Mcp-Session-Id", c.sid)
	}
	if c.ver != "" && metodo != "initialize" {
		req.Header.Set("MCP-Protocol-Version", c.ver)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSemConexao, err)
	}
	defer resp.Body.Close()
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		c.sid = sid
	}
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
		return nil, errSessao{resp.StatusCode}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("MCP respondeu HTTP %d", resp.StatusCode)
	}
	if notif {
		io.Copy(io.Discard, resp.Body)
		return nil, nil
	}
	var m struct {
		ID     *int            `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		if !lerSSE(resp.Body, id, &m) {
			return nil, errors.New("resposta MCP sem resultado")
		}
	} else if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("resposta MCP inválida: %w", err)
	}
	if m.Error != nil {
		if m.Error.Message == "" {
			return nil, errors.New("erro MCP")
		}
		return nil, errors.New(m.Error.Message)
	}
	return m.Result, nil
}

// lerSSE acha o evento `data:` cujo id é o esperado.
func lerSSE(r io.Reader, id int, alvo any) bool {
	var dados []string
	trata := func() bool {
		if len(dados) == 0 {
			return false
		}
		bruto := strings.Join(dados, "\n")
		dados = nil
		var cab struct {
			ID *int `json:"id"`
		}
		if json.Unmarshal([]byte(bruto), &cab) != nil || cab.ID == nil || *cab.ID != id {
			return false
		}
		return json.Unmarshal([]byte(bruto), alvo) == nil
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		l := strings.TrimRight(sc.Text(), "\r")
		if l == "" {
			if trata() {
				return true
			}
			continue
		}
		if strings.HasPrefix(l, "data:") {
			dados = append(dados, strings.TrimSpace(l[5:]))
		}
	}
	return trata()
}
