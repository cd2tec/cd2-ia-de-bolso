// Package toolbox copia, sobe, confere e derruba o MCP Toolbox for Databases para o cd2-ctl.
package toolbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/dbconf"
	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/dbf"
	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/excel"
	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/secret"
)

// Mensagens que vão para a tela (campo erro de /status).
const (
	MsgDied      = "O toolbox não conectou ao banco. Confira host, porta, base, usuário e senha."
	MsgToolError = "O banco respondeu com erro ao testar a conexão. Confira usuário, senha e permissões."
)

// Options diz onde está cada coisa e quanto esperar.
type Options struct {
	Source       string        // binário no pen drive: <root>/bin/toolbox/<os>-<arch>/toolbox[.exe]
	CacheDir     string        // cópia executável e toolbox.log
	RecipeDir    string        // <root>/core/toolbox
	Port         int           // P+1000
	HarnessPort  int           // P
	ReadyTimeout time.Duration // 15 s
	StopTimeout  time.Duration // 5 s para a porta liberar
}

// StartError é uma subida que falhou: Msg vai para a tela, Log são as últimas linhas já filtradas.
type StartError struct {
	Msg string
	Log []string
}

func (e *StartError) Error() string { return e.Msg }

// Supervisor cuida de um toolbox por vez. Start, Reload e Stop não são concorrentes entre si
// (quem chama serializa); Running e Planilhas podem ser chamados a qualquer hora.
type Supervisor struct {
	opt    Options
	copied bool

	mu        sync.Mutex
	cmd       *exec.Cmd
	done      chan struct{}
	planilhas *excel.Relatorio // relatório da última importação Excel; nulo nos outros motores
}

// New aplica os prazos padrão (15 s e 5 s) quando vêm zerados.
func New(opt Options) *Supervisor {
	if opt.ReadyTimeout == 0 {
		opt.ReadyTimeout = 15 * time.Second
	}
	if opt.StopTimeout == 0 {
		opt.StopTimeout = 5 * time.Second
	}
	return &Supervisor{opt: opt}
}

// LogPath é o toolbox.log do cache (só linhas que passaram pelo filtro).
func (s *Supervisor) LogPath() string { return filepath.Join(s.opt.CacheDir, "toolbox.log") }

// Running diz se o toolbox da última subida continua vivo.
func (s *Supervisor) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done == nil {
		return false
	}
	select {
	case <-s.done:
		return false
	default:
		return true
	}
}

// Planilhas devolve o relatório da última importação de planilhas Excel (nulo nos outros motores,
// inclusive DBF). Fica guardado mesmo quando o Start falhou na importação, para a tela mostrar
// o que ficou de fora.
func (s *Supervisor) Planilhas() *excel.Relatorio {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.planilhas
}

// Start sobe o toolbox com cfg e só volta quando ele está pronto (nil) ou falhou (*StartError,
// já com o processo derrubado). Nos motores de arquivo, importa a pasta antes de subir.
func (s *Supervisor) Start(cfg dbconf.Config) error {
	if s.Running() {
		return &StartError{Msg: "O toolbox já está rodando."}
	}
	troca, err := s.importar(cfg)
	if err != nil {
		return err
	}
	if troca != nil {
		if err := troca(); err != nil {
			return err
		}
	}
	return s.subir(cfg)
}

// Reload relê a pasta de um motor de arquivo com o toolbox no ar. A nova cópia é montada ao
// lado (.novo); se a importação falhar, devolve *StartError e o toolbox segue com a cópia
// anterior. Só depois de a nova estar inteira derruba o toolbox, troca a cópia e sobe de novo.
func (s *Supervisor) Reload(cfg dbconf.Config) error {
	if !dbconf.EngineArquivo(cfg.Engine) {
		return &StartError{Msg: "Esta conexão não é de arquivos."}
	}
	troca, err := s.importar(cfg)
	if err != nil {
		return err
	}
	if err := s.Stop(); err != nil {
		removeSqlite(s.sqlitePath(cfg.Engine) + ".novo")
		return &StartError{Msg: "Não consegui parar o toolbox para trocar a cópia: " + err.Error()}
	}
	if err := troca(); err != nil {
		return err
	}
	return s.subir(cfg)
}

// importar grava a cópia SQLite de um motor de arquivo em <sqlite>.novo, guarda o relatório das
// planilhas (também quando a importação falha) e devolve a função que põe a cópia nova no
// lugar da anterior. Nos outros motores, zera o relatório e devolve troca nula.
func (s *Supervisor) importar(cfg dbconf.Config) (troca func() error, err error) {
	if !dbconf.EngineArquivo(cfg.Engine) {
		s.mu.Lock()
		s.planilhas = nil
		s.mu.Unlock()
		return nil, nil
	}
	destino := s.sqlitePath(cfg.Engine)
	novo := destino + ".novo"
	removeSqlite(novo)
	var rel *excel.Relatorio
	if cfg.Engine == "excel" {
		rel, err = excel.Import(cfg.Name, novo)
	} else {
		err = dbf.Import(cfg.Name, novo)
	}
	s.mu.Lock()
	s.planilhas = rel
	s.mu.Unlock()
	if err != nil {
		removeSqlite(novo)
		return nil, &StartError{Msg: err.Error()}
	}
	return func() error {
		// só troca a cópia anterior depois de a nova estar inteira
		_ = os.Remove(destino + "-wal")
		_ = os.Remove(destino + "-shm")
		if err := os.Rename(novo, destino); err != nil {
			removeSqlite(novo)
			return &StartError{Msg: "Não consegui gravar a cópia das planilhas: " + err.Error()}
		}
		return nil
	}, nil
}

// subir prepara o binário, abre o toolbox.log, inicia o processo e espera ficar pronto.
func (s *Supervisor) subir(cfg dbconf.Config) error {
	bin, err := s.prepareBinary()
	if err != nil {
		return &StartError{Msg: "Não consegui preparar o toolbox: " + err.Error()}
	}
	logf, err := os.OpenFile(s.LogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return &StartError{Msg: "Não consegui abrir o toolbox.log: " + err.Error()}
	}
	last := &tail{max: 15}
	scrub := secret.NewScrubber(io.MultiWriter(logf, last), cfg.Password)
	cmd := exec.Command(bin, s.args(cfg.Engine)...)
	cmd.Env = s.env(cfg)
	cmd.Stdout, cmd.Stderr = scrub, scrub
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Start(); err != nil {
		logf.Close()
		return &StartError{Msg: "Não consegui iniciar o toolbox: " + err.Error()}
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		_ = scrub.Close()
		_ = logf.Close()
		close(done)
	}()
	s.mu.Lock()
	s.cmd, s.done = cmd, done
	s.mu.Unlock()
	if msg := s.waitReady(done); msg != "" {
		_ = s.Stop()
		lines := last.lines()
		if n := scrub.Dropped(); n > 0 {
			lines = append(lines, fmt.Sprintf("(%d linha(s) omitida(s) por citar a senha)", n))
		}
		return &StartError{Msg: msg, Log: lines}
	}
	return nil
}

// Stop derruba o toolbox (TERM no Unix, Kill no Windows; Kill depois de 3 s) e espera a porta
// liberar por até StopTimeout.
func (s *Supervisor) Stop() error {
	s.mu.Lock()
	cmd, done := s.cmd, s.done
	s.cmd, s.done = nil, nil
	s.mu.Unlock()
	if cmd == nil {
		return nil
	}
	select {
	case <-done:
	default:
		terminate(cmd.Process)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
			}
		}
	}
	return waitPortFree(s.opt.Port, s.opt.StopTimeout)
}

func (s *Supervisor) args(engine string) []string {
	tb, h := strconv.Itoa(s.opt.Port), strconv.Itoa(s.opt.HarnessPort)
	return []string{
		"--config", filepath.Join(s.opt.RecipeDir, engine+".yaml"),
		"--address", "127.0.0.1", "--port", tb,
		"--allowed-hosts", "127.0.0.1:" + tb + ",localhost:" + tb,
		"--allowed-origins", "http://127.0.0.1:" + h + ",http://localhost:" + h,
		"--disable-reload",
	}
}

// env é o ambiente do cd2-ctl sem a chave da sessão e sem CD2_DB_* herdado, mais a conexão
// com a senha escapada para o escalar "..." das receitas.
func (s *Supervisor) env(cfg dbconf.Config) []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "CD2_DB_") || strings.HasPrefix(kv, "CD2_CTL_KEY=") {
			continue
		}
		env = append(env, kv)
	}
	c := cfg
	c.Password = secret.YamlDQ(cfg.Password)
	env = append(env, c.Env()...)
	if dbconf.EngineArquivo(cfg.Engine) {
		env = append(env, "CD2_DB_SQLITE="+s.sqlitePath(cfg.Engine))
	}
	return env
}

// sqlitePath é a cópia SQLite que o toolbox consulta: excel.sqlite ou dbf.sqlite no cache.
func (s *Supervisor) sqlitePath(engine string) string {
	return filepath.Join(s.opt.CacheDir, engine+".sqlite")
}

// removeSqlite apaga um SQLite e os arquivos -wal e -shm que ficaram ao lado dele.
func removeSqlite(path string) {
	for _, suf := range []string{"", "-wal", "-shm"} {
		_ = os.Remove(path + suf)
	}
}

// livenessTimeout limita só o GET / (o toolbox responde na hora quando está no ar). O initialize
// e o db_connection_status usam o prazo inteiro (ReadyTimeout): um Oracle ou SQL Server atrás de
// VPN pode levar vários segundos para responder ao primeiro teste de conexão.
const livenessTimeout = 2 * time.Second

// waitReady devolve "" quando pronto, ou a mensagem para a tela. Nunca passa do ReadyTimeout
// (mais os 200 ms de uma pausa entre tentativas).
func (s *Supervisor) waitReady(done <-chan struct{}) string {
	base := "http://127.0.0.1:" + strconv.Itoa(s.opt.Port)
	client := &http.Client{}
	deadline := time.Now().Add(s.opt.ReadyTimeout)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	// se o toolbox morrer no meio de uma chamada, ela é cancelada na hora
	go func() {
		select {
		case <-done:
			cancel()
		case <-ctx.Done():
		}
	}()
	for {
		select {
		case <-done:
			return MsgDied
		default:
		}
		err := probe(ctx, client, base)
		if err == nil {
			return ""
		}
		if errors.Is(err, errToolError) {
			return MsgToolError
		}
		select {
		case <-done:
			return MsgDied
		default:
		}
		if !time.Now().Before(deadline) {
			return fmt.Sprintf("O banco não respondeu em %d segundos. Confira host, porta e firewall.", int(s.opt.ReadyTimeout/time.Second))
		}
		select {
		case <-done:
			return MsgDied
		case <-ctx.Done():
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func waitPortFree(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			l.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("a porta %d continua ocupada", port)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// tail guarda as últimas max linhas que passaram pelo filtro (o Scrubber escreve uma por vez).
type tail struct {
	mu  sync.Mutex
	max int
	buf []string
}

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, l := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		t.buf = append(t.buf, l)
		if len(t.buf) > t.max {
			t.buf = t.buf[1:]
		}
	}
	return len(p), nil
}

func (t *tail) lines() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.buf...)
}
