// Package control é o estado do cd2-ctl: qual conexão vale, se o toolbox está de pé e a troca
// com rollback pedida pela tela.
package control

import (
	"errors"
	"io/fs"
	"strconv"
	"sync"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/dbconf"
	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/excel"
	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/toolbox"
)

// Estados de /status.
const (
	SemBanco = "sem_banco"
	Subindo  = "subindo"
	Rodando  = "rodando"
	Falhou   = "falhou"
)

// MsgClosed é a resposta de Apply, Clear e Reler depois do Shutdown.
const MsgClosed = "O cd2-ctl está encerrando."

// MsgStopped aparece quando o toolbox morre sozinho depois de pronto.
const MsgStopped = "O toolbox parou sozinho. Clique em Salvar e conectar para tentar de novo."

// ErrNaoArquivo é a resposta de Reler quando não há conexão ou ela não é de um motor de arquivo.
var ErrNaoArquivo = errors.New("Esta conexão não é de arquivos.")

// Runner sobe e derruba o toolbox (*toolbox.Supervisor; nos testes, um falso).
type Runner interface {
	Start(dbconf.Config) error
	Stop() error
	Running() bool
	// Reload relê a pasta de um motor de arquivo; se a importação falhar, o toolbox fica como estava.
	Reload(dbconf.Config) error
	// Planilhas é o relatório da última importação de planilhas Excel (nulo nos outros motores).
	Planilhas() *excel.Relatorio
}

// Status é a resposta de GET /status (e de POST e DELETE /config). Nunca leva a senha.
type Status struct {
	Estado     string   `json:"estado"`
	Motor      string   `json:"motor"`
	Host       string   `json:"host"`
	Porta      int      `json:"porta,omitempty"`
	Base       string   `json:"base"`
	Usuario    string   `json:"usuario"`
	Schema     string   `json:"schema"`
	SenhaSalva bool     `json:"senha_salva"`
	Erro       string   `json:"erro,omitempty"`
	Log        []string `json:"log,omitempty"`
	// Planilhas é o relatório da última leitura da pasta de planilhas (só no motor excel).
	Planilhas *excel.Relatorio `json:"planilhas,omitempty"`
}

// Controller guarda a conexão em vigor e atende um pedido de configuração por vez.
type Controller struct {
	envPath string
	run     Runner

	op     sync.Mutex // um pedido de configuração (ou o arranque) por vez
	closed bool       // true depois de Shutdown; lido e escrito só com op preso

	mu     sync.Mutex // protege os campos abaixo
	cur    *dbconf.Config
	estado string
	erro   string
	log    []string
}

// New começa em sem_banco; chame BootAsync para ler o db.env.
func New(envPath string, run Runner) *Controller {
	return &Controller{envPath: envPath, run: run, estado: SemBanco}
}

func (c *Controller) set(cur *dbconf.Config, estado, erro string, log []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cur, c.estado, c.erro, c.log = cur, estado, erro, log
}

func (c *Controller) current() *dbconf.Config {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cur
}

// Status monta a resposta. Se o toolbox morreu depois de pronto, diz falhou.
func (c *Controller) Status() Status {
	c.mu.Lock()
	cur, estado, erro, log := c.cur, c.estado, c.erro, c.log
	c.mu.Unlock()
	if estado == Rodando && !c.run.Running() {
		estado, erro, log = Falhou, MsgStopped, nil
	}
	st := Status{Estado: estado, Erro: erro, Log: log, Planilhas: c.run.Planilhas()}
	if cur != nil {
		st.Motor, st.Host, st.Base, st.Usuario, st.Schema = cur.Engine, cur.Host, cur.Name, cur.User, cur.Schema
		st.Porta, _ = strconv.Atoi(cur.Port)
		st.SenhaSalva = cur.Password != ""
	}
	return st
}

// BootAsync lê o db.env e, se válido, sobe o toolbox em segundo plano. Pega a vez dos pedidos
// antes de voltar, para um POST /config não atropelar a subida. O canal fecha no fim.
func (c *Controller) BootAsync() <-chan struct{} {
	c.op.Lock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer c.op.Unlock()
		c.boot()
	}()
	return done
}

func (c *Controller) boot() {
	if c.closed {
		return
	}
	cfg, err := dbconf.Load(c.envPath)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err == nil {
		err = cfg.Check()
	}
	if err != nil {
		c.set(nil, SemBanco, "O db.env deste computador está inválido ("+err.Error()+"). Configure o banco de novo.", nil)
		return
	}
	c.set(&cfg, Subindo, "", nil)
	if err := c.run.Start(cfg); err != nil {
		msg, log := describe(err)
		c.set(&cfg, Falhou, msg, log)
		return
	}
	c.set(&cfg, Rodando, "", nil)
}

func describe(err error) (string, []string) {
	var se *toolbox.StartError
	if errors.As(err, &se) {
		return se.Msg, se.Log
	}
	return err.Error(), nil
}

// Apply valida e troca a conexão. Campo inválido devolve *dbconf.FieldError sem mexer em nada.
// Falha na subida não grava nada e volta para a conexão anterior, se havia.
func (c *Controller) Apply(in dbconf.Input) (Status, error) {
	c.op.Lock()
	defer c.op.Unlock()
	if c.closed {
		return c.closedStatus(), nil
	}
	prev := c.current()
	cand, err := dbconf.Validate(in, prev)
	if err != nil {
		return Status{}, err
	}
	c.set(prev, Subindo, "", nil)
	if err := c.run.Stop(); err != nil {
		c.rollback(prev, err)
		return c.Status(), nil
	}
	if err := c.run.Start(cand); err != nil {
		c.rollback(prev, err)
		return c.Status(), nil
	}
	if err := dbconf.Save(c.envPath, cand); err != nil {
		_ = c.run.Stop()
		c.rollback(prev, errors.New("Não consegui gravar a conexão neste computador: "+err.Error()))
		return c.Status(), nil
	}
	c.set(&cand, Rodando, "", nil)
	return c.Status(), nil
}

// Reler relê a pasta da conexão em vigor (excel ou dbf) sem derrubar a cópia que está no ar
// quando a importação falha. Sem conexão, ou com motor que não é de arquivo, devolve
// ErrNaoArquivo sem mexer em nada.
func (c *Controller) Reler() (Status, error) {
	c.op.Lock()
	defer c.op.Unlock()
	if c.closed {
		return c.closedStatus(), nil
	}
	cur := c.current()
	if cur == nil || !dbconf.EngineArquivo(cur.Engine) {
		return Status{}, ErrNaoArquivo
	}
	c.set(cur, Subindo, "", nil)
	err := c.run.Reload(*cur)
	switch {
	case err == nil:
		c.set(cur, Rodando, "", nil)
	case c.run.Running():
		// a importação falhou antes de mexer no toolbox: a cópia antiga continua no ar
		msg, log := describe(err)
		c.set(cur, Rodando, msg, log)
	default:
		// o toolbox caiu no meio da troca: sobe de novo a mesma conexão (o Start relê a pasta)
		c.rollback(cur, err)
	}
	return c.Status(), nil
}

// rollback sobe de novo a conexão anterior e guarda erro e log da tentativa que falhou.
func (c *Controller) rollback(prev *dbconf.Config, failure error) {
	msg, log := describe(failure)
	if prev == nil {
		c.set(nil, SemBanco, msg, log)
		return
	}
	if err := c.run.Start(*prev); err != nil {
		// As duas falhas vão no Erro. O Log é o da volta à anterior, porque é a conexão
		// que a tela mostra agora (os campos são os de prev).
		rmsg, rlog := describe(err)
		c.set(prev, Falhou, "A nova conexão falhou: "+msg+" A anterior também não voltou: "+rmsg, rlog)
		return
	}
	c.set(prev, Rodando, msg, log)
}

// Clear derruba o toolbox e apaga o db.env.
func (c *Controller) Clear() Status {
	c.op.Lock()
	defer c.op.Unlock()
	if c.closed {
		return c.closedStatus()
	}
	_ = c.run.Stop()
	if err := dbconf.Remove(c.envPath); err != nil {
		c.set(c.current(), Falhou, "Não consegui apagar o db.env: "+err.Error(), nil)
		return c.Status()
	}
	c.set(nil, SemBanco, "", nil)
	return c.Status()
}

// Shutdown derruba o toolbox no encerramento (espera um pedido em andamento terminar).
func (c *Controller) Shutdown() {
	c.op.Lock()
	defer c.op.Unlock()
	c.closed = true
	_ = c.run.Stop()
}

// closedStatus é a resposta de Apply, Clear e Reler depois do Shutdown: nada é iniciado nem apagado.
func (c *Controller) closedStatus() Status {
	st := c.Status()
	st.Erro, st.Log = MsgClosed, nil
	return st
}
