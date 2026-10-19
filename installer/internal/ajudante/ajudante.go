// Package ajudante roda como administrador só o que precisa disso no Windows:
// listar, apagar e ejetar o pen drive. A janela do instalador fica com o
// usuário logado, porque com a senha de outra conta (a da TI, num computador
// de empresa) o WebView2 roda como essa outra conta e não abre ("O Microsoft
// Edge não pode ler e gravar no diretório de dados").
//
// A janela ouve numa porta local e abre o ajudante, que é o próprio exe com
// ArgAjudante e ArgCodigo, pedindo administrador. O ajudante liga de volta,
// mostra o código e atende os pedidos até a conexão fechar. Ele nunca ouve
// porta nenhuma, e só faz as três operações do discos.Sistema.
package ajudante

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/discos"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/elevacao"
)

// Argumentos com que o ajudante é aberto.
const (
	ArgAjudante = "--ajudante="
	ArgCodigo   = "--codigo="
)

// PausaRecusa é quanto o cliente espera para perguntar de novo depois de a
// pessoa dizer "Não" ao Windows: a tela busca discos a cada 1,5 s.
const PausaRecusa = 30 * time.Second

// prazoConexao é quanto a janela espera o ajudante ligar depois que o
// Windows liberou. O tempo da pergunta em si não conta: abrir só volta
// quando a pessoa responde.
const prazoConexao = 30 * time.Second

// prazoCodigo é quanto quem conecta tem para mostrar o código.
const prazoCodigo = 5 * time.Second

// ErrRecusou é a resposta quando a pessoa não deu a permissão de administrador.
var ErrRecusou = elevacao.ErrRecusou

type pedido struct {
	Op       string `json:"op"`
	ID       string `json:"id,omitempty"`
	Rotulo   string `json:"rotulo,omitempty"`
	Montagem string `json:"montagem,omitempty"`
}

type resposta struct {
	Brutos   []discos.Bruto `json:"brutos,omitempty"`
	Montagem string         `json:"montagem,omitempty"`
	Erro     string         `json:"erro,omitempty"`
}

type apresentacao struct {
	Codigo string `json:"codigo"`
}

// Args monta os argumentos do ajudante.
func Args(endereco, codigo string) []string {
	return []string{ArgAjudante + endereco, ArgCodigo + codigo}
}

// LerArgs devolve o endereço e o código, se args forem os do ajudante. O
// endereço tem de ser 127.0.0.1: o ajudante só liga para esta máquina.
func LerArgs(args []string) (endereco, codigo string, ok bool) {
	if len(args) != 2 || !strings.HasPrefix(args[0], ArgAjudante) || !strings.HasPrefix(args[1], ArgCodigo) {
		return "", "", false
	}
	endereco, codigo = args[0][len(ArgAjudante):], args[1][len(ArgCodigo):]
	host, _, err := net.SplitHostPort(endereco)
	if err != nil || host != "127.0.0.1" || codigo == "" {
		return "", "", false
	}
	return endereco, codigo, true
}

// Servir é o lado elevado: liga para a janela, mostra o código e atende os
// pedidos com s até a conexão fechar (a janela fechou ou caiu).
func Servir(endereco, codigo string, s discos.Sistema) error {
	conn, err := net.DialTimeout("tcp", endereco, prazoCodigo)
	if err != nil {
		return err
	}
	defer conn.Close()
	enc, dec := json.NewEncoder(conn), json.NewDecoder(bufio.NewReader(conn))
	if err := enc.Encode(apresentacao{Codigo: codigo}); err != nil {
		return err
	}
	ctx := context.Background()
	for {
		var p pedido
		if err := dec.Decode(&p); err != nil {
			return nil // a janela fechou
		}
		if err := enc.Encode(atender(ctx, s, p)); err != nil {
			return err
		}
	}
}

func atender(ctx context.Context, s discos.Sistema, p pedido) resposta {
	var r resposta
	var err error
	switch p.Op {
	case "listar":
		r.Brutos, err = s.Listar(ctx)
	case "apagar":
		r.Montagem, err = s.Apagar(ctx, p.ID, p.Rotulo)
	case "ejetar":
		err = s.Ejetar(ctx, p.ID, p.Montagem)
	default:
		err = fmt.Errorf("pedido desconhecido %q", p.Op)
	}
	if err != nil {
		r.Erro = err.Error()
	}
	return r
}

// Cliente é o discos.Sistema da janela sem administrador: manda cada
// operação para o ajudante, que abre na primeira vez que precisa.
type Cliente struct {
	abrir func(args []string) error
	agora func() time.Time

	mu        sync.Mutex
	conn      net.Conn
	enc       *json.Encoder
	dec       *json.Decoder
	recusouEm time.Time
}

// NovoCliente devolve o cliente. abrir roda o próprio exe com os argumentos
// como administrador; devolve ErrRecusou se a pessoa disser "Não".
func NovoCliente(abrir func(args []string) error) *Cliente {
	return &Cliente{abrir: abrir, agora: time.Now}
}

// Fechar desliga a conexão; o ajudante sai sozinho.
func (c *Cliente) Fechar() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.largar()
}

func (c *Cliente) largar() {
	if c.conn != nil {
		c.conn.Close()
	}
	c.conn, c.enc, c.dec = nil, nil, nil
}

func (c *Cliente) Listar(ctx context.Context) ([]discos.Bruto, error) {
	r, err := c.pedir(ctx, pedido{Op: "listar"})
	return r.Brutos, err
}

func (c *Cliente) Apagar(ctx context.Context, id, rotulo string) (string, error) {
	r, err := c.pedir(ctx, pedido{Op: "apagar", ID: id, Rotulo: rotulo})
	return r.Montagem, err
}

func (c *Cliente) Ejetar(ctx context.Context, id, montagem string) error {
	_, err := c.pedir(ctx, pedido{Op: "ejetar", ID: id, Montagem: montagem})
	return err
}

func (c *Cliente) pedir(ctx context.Context, p pedido) (resposta, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		if err := c.ligar(); err != nil {
			return resposta{}, err
		}
	}
	// Cancelar o contexto solta a espera; a conexão vai junto, e a próxima
	// chamada abre outro ajudante.
	parar := context.AfterFunc(ctx, func() { c.conn.SetDeadline(time.Now()) })
	defer parar()
	var r resposta
	err := c.enc.Encode(p)
	if err == nil {
		err = c.dec.Decode(&r)
	}
	if err != nil {
		c.largar()
		if ctx.Err() != nil {
			return resposta{}, ctx.Err()
		}
		return resposta{}, fmt.Errorf("ajudante de administrador: %w", err)
	}
	if r.Erro != "" {
		return r, errors.New(r.Erro)
	}
	return r, nil
}

// ligar abre o ajudante e espera ele ligar de volta com o código certo.
func (c *Cliente) ligar() error {
	if !c.recusouEm.IsZero() && c.agora().Sub(c.recusouEm) < PausaRecusa {
		return ErrRecusou
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer l.Close()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	codigo := hex.EncodeToString(b)
	if err := c.abrir(Args(l.Addr().String(), codigo)); err != nil {
		if errors.Is(err, ErrRecusou) {
			c.recusouEm = c.agora()
		}
		return err
	}
	c.recusouEm = time.Time{}
	l.(*net.TCPListener).SetDeadline(time.Now().Add(prazoConexao))
	for {
		conn, err := l.Accept()
		if err != nil {
			return fmt.Errorf("o ajudante de administrador não ligou: %w", err)
		}
		dec := json.NewDecoder(bufio.NewReader(conn))
		var a apresentacao
		conn.SetDeadline(time.Now().Add(prazoCodigo))
		if dec.Decode(&a) != nil || subtle.ConstantTimeCompare([]byte(a.Codigo), []byte(codigo)) != 1 {
			conn.Close()
			continue
		}
		conn.SetDeadline(time.Time{})
		c.conn, c.enc, c.dec = conn, json.NewEncoder(conn), dec
		return nil
	}
}
