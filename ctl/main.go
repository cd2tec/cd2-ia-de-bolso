// cd2-ctl: controle do banco de dados da IA de Bolso da CD2. Escuta só em 127.0.0.1:P+3000,
// é o único dono do MCP Toolbox (127.0.0.1:P+1000) e grava a conexão no db.env do cache.
//
// Uso: CD2_CTL_KEY=<64 hex> cd2-ctl --root <pen drive> --cache <pasta> --harness-port P [--parent-pid N]
//
// Com --parent-pid, o cd2-ctl confere a cada 2 s se o launcher ainda existe e, se ele sumiu sem
// limpar (console do Windows fechado no X, kill -9 no Unix), encerra como se tivesse recebido TERM.
package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/api"
	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/control"
	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/inst"
	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/notas"
	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/pasta"
	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/toolbox"
)

// parentPollInterval é de quanto em quanto tempo o --parent-pid é conferido.
var parentPollInterval = 2 * time.Second

// version vem do -ldflags "-X main.version=X.Y.Z" no release.
var version = "dev"

func main() {
	key := os.Getenv("CD2_CTL_KEY")
	_ = os.Unsetenv("CD2_CTL_KEY") // o toolbox (processo filho) não precisa dela
	stop := make(chan os.Signal, 1)
	// SIGHUP: terminal fechado no Mac/Linux. No Windows, fechar o console chega como SIGTERM.
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	os.Exit(run(os.Args[1:], key, stop, os.Stdout, os.Stderr))
}

func validKey(k string) bool {
	if len(k) != 64 {
		return false
	}
	_, err := hex.DecodeString(k)
	return err == nil
}

// run devolve o código de saída: 0 encerrou por sinal (ou --version), 2 erro de arranque.
// Mensagens de erro sem acento (vão para o terminal e para o ctl.log do launcher).
func run(args []string, key string, stop <-chan os.Signal, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("cd2-ctl", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", "", "raiz do pen drive")
	cache := fs.String("cache", "", "pasta de cache deste computador (db.env e toolbox.log)")
	hport := fs.Int("harness-port", 0, "porta do harness (P); toolbox em P+1000, controle em P+3000")
	parentPID := fs.Int("parent-pid", 0, "PID do launcher: quando ele some, o cd2-ctl encerra sozinho")
	showVersion := fs.Bool("version", false, "mostra a versao e sai")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, "cd2-ctl", version)
		return 0
	}
	if !validKey(key) {
		fmt.Fprintln(stderr, "cd2-ctl: CD2_CTL_KEY ausente ou invalida (64 caracteres hexadecimais)")
		return 2
	}
	if *root == "" || *cache == "" || *hport < 1 || *hport+3000 > 65535 || *parentPID < 0 {
		fmt.Fprintln(stderr, "uso: CD2_CTL_KEY=<chave> cd2-ctl --root <pen drive> --cache <pasta> --harness-port P [--parent-pid N]")
		return 2
	}
	if err := os.MkdirAll(*cache, 0o700); err != nil {
		fmt.Fprintf(stderr, "cd2-ctl: nao consegui criar %s: %v\n", *cache, err)
		return 2
	}
	ctlPort, tbPort := *hport+3000, *hport+1000
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", ctlPort))
	if err != nil {
		fmt.Fprintf(stderr, "cd2-ctl: nao consegui escutar em 127.0.0.1:%d: %v\n", ctlPort, err)
		return 2
	}
	exe := ""
	if runtime.GOOS == "windows" {
		exe = ".exe"
	}
	sup := toolbox.New(toolbox.Options{
		Source:       filepath.Join(*root, "bin", "toolbox", runtime.GOOS+"-"+runtime.GOARCH, "toolbox"+exe),
		CacheDir:     *cache,
		RecipeDir:    filepath.Join(*root, "core", "toolbox"),
		Port:         tbPort,
		HarnessPort:  *hport,
		ReadyTimeout: 15 * time.Second,
		StopTimeout:  5 * time.Second,
	})
	// toolbox.log começa vazio a cada abertura do harness
	if f, err := os.OpenFile(sup.LogPath(), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600); err == nil {
		f.Close()
	}
	ctl := control.New(filepath.Join(*cache, "db.env"), sup)
	ctl.BootAsync()
	instalacao := inst.New(*root, *cache)
	srv := &http.Server{
		Handler: api.Handler(api.Options{Key: key, HarnessPort: *hport, CtlPort: ctlPort, Instalacao: instalacao,
			Notas:         notas.New(*cache),
			Entendimento:  notas.NewEntendimento(*cache),
			PodeImportar:  func() bool { return instalacao.Info().Origem == "instalada" },
			EscolherPasta: pasta.Escolher,
		}, ctl),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() { _ = srv.Serve(ln) }()
	fmt.Fprintf(stdout, "cd2-ctl %s em 127.0.0.1:%d (toolbox em 127.0.0.1:%d)\n", version, ctlPort, tbPort)
	parentGone := make(chan struct{})
	quit := make(chan struct{})
	defer close(quit)
	if *parentPID > 0 {
		go watchParent(*parentPID, parentGone, quit)
	}
	select {
	case <-stop:
	case <-parentGone:
		fmt.Fprintf(stdout, "cd2-ctl: o launcher (PID %d) sumiu, encerrando\n", *parentPID)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	instalacao.Shutdown()
	ctl.Shutdown()
	fmt.Fprintln(stdout, "cd2-ctl encerrado")
	return 0
}

// watchParent fecha gone quando o processo pid deixa de existir (ou para quando quit fecha).
func watchParent(pid int, gone, quit chan struct{}) {
	t := time.NewTicker(parentPollInterval)
	defer t.Stop()
	for {
		select {
		case <-quit:
			return
		case <-t.C:
			if !processAlive(pid) {
				close(gone)
				return
			}
		}
	}
}
