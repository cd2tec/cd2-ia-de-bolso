package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/ambiente"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/chave"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/guarda"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/sistema"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/elevacao"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/gravar"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/volta"
)

// Frases do caminho de instalar no computador.
const (
	fraseBloqueio        = "Abra o instalador de novo sem ser como administrador (clique duplo normal) para instalar neste computador."
	fraseChaveIncompleta = "Esta chave não está completa. Confira se copiou ela inteira."
	fraseChaveDeOutro    = "Esta chave não é deste computador."
	fraseNaoAbriuIA      = "Não consegui abrir a IA de Bolso. Abra pelo ícone nos programas."
)

// tempoEspaco limita a conta de espaço, que baixa o manifesto.
const tempoEspaco = 30 * time.Second

// InfoInstalar é o que a tela de condições precisa saber.
type InfoInstalar struct {
	Codigo   string `json:"codigo"`
	TemChave bool   `json:"temChave"`
	Worker   string `json:"worker"`
	Sandbox  bool   `json:"sandbox"`
	Precisa  int64  `json:"precisa"`
	Livre    int64  `json:"livre"`
	Cabe     bool   `json:"cabe"`
	Bloqueio string `json:"bloqueio"`
}

// EventoChave é o evento "chave": a chave voltou do pagamento e foi (ou não)
// aceita para este computador.
type EventoChave struct {
	Ok   bool   `json:"ok"`
	Erro string `json:"erro"`
}

// pastaChavePadrao é a pasta cd2-harness, a da chave do lançador e do ctl: a
// mãe da pasta do stage (cd2-harness/usb).
func pastaChavePadrao() (string, error) {
	usb, err := gravar.PastaCache()
	if err != nil {
		return "", err
	}
	return filepath.Dir(usb), nil
}

// abrirSolto inicia o programa destacado, na pasta dele, sem esperar.
func abrirSolto(caminho string) error {
	c := exec.Command(caminho)
	c.Dir = filepath.Dir(caminho)
	elevacao.Destacar(c)
	if err := c.Start(); err != nil {
		return err
	}
	go func() { _ = c.Wait() }()
	return nil
}

// lancadorInstalado é o lançador da IA instalada neste computador.
func lancadorInstalado() string {
	home, _ := os.UserHomeDir()
	return sistema.Lancador(sistema.Destino(goruntime.GOOS, os.Getenv, home), goruntime.GOOS)
}

// urlCompra é a página do Worker que cria o pagamento deste computador.
func urlCompra(worker, codigo string, porta int) string {
	return worker + "/comprar?codigo=" + url.QueryEscape(codigo) +
		"&porta=" + strconv.Itoa(porta) + "&de=instalador"
}

// IniciouComInstalar diz se o instalador abriu com --instalar, ou seja, se
// foi reaberto sem administrador para seguir direto para a instalação.
func (a *App) IniciouComInstalar() bool { return a.comInstalar }

// iniciarComArgs lê os argumentos antes de a janela abrir. Com --instalar,
// tira o __COMPAT_LAYER herdado da reabertura (os filhos não precisam dele)
// e dá o sinal que a instância elevada está esperando para sair.
func (a *App) iniciarComArgs(args []string) {
	instalar, sinal := elevacao.LerArgs(args)
	a.comInstalar = instalar
	if !instalar {
		return
	}
	os.Unsetenv("__COMPAT_LAYER")
	if sinal != "" {
		if err := a.sinalizar(sinal); err != nil {
			log.Printf("sinal da reabertura: %v", err)
		}
	}
}

// EscolherInstalar roda quando a pessoa escolhe instalar neste computador.
// No Windows como administrador, reabre o instalador sem administrador com
// --instalar e um sinal, espera a instância nova dar o sinal e só então
// fecha esta. Se a reabertura falhar ou o sinal não vier, devolve o aviso de
// bloqueio. A instância que já veio com --instalar nunca reabre.
func (a *App) EscolherInstalar() string {
	if a.comInstalar || !a.elevado() {
		return ""
	}
	exe, err := os.Executable()
	var nonce string
	if err == nil {
		nonce, err = a.novoSinal()
	}
	if err == nil {
		err = a.reabrir(exe, []string{elevacao.ArgInstalar, elevacao.ArgSinal + nonce})
	}
	if err == nil && !a.esperarSinal(nonce) {
		err = errors.New("a instância reaberta não deu sinal no prazo")
	}
	if err != nil {
		log.Printf("reabrir sem administrador: %v", err)
		a.mu.Lock()
		a.bloqueio = fraseBloqueio
		a.mu.Unlock()
		return fraseBloqueio
	}
	a.sair(a.ctx)
	return ""
}

// InfoInstalar diz o código do computador, se já há chave valendo, para
// qual Worker vai a compra e se a instalação cabe no disco. Se a conta de
// espaço falhar (sem internet, por exemplo), Precisa e Livre ficam 0 e Cabe
// fica true: quem avisa da internet é a instalação.
func (a *App) InfoInstalar() InfoInstalar {
	cod, err := a.lerCodigo()
	if err != nil {
		log.Printf("código do computador: %v", err)
		cod = ""
	}
	a.mu.Lock()
	bloqueio := a.bloqueio
	a.mu.Unlock()
	info := InfoInstalar{
		Codigo:   cod,
		TemChave: a.temChave(),
		Worker:   ambiente.Worker,
		Sandbox:  ambiente.Sandbox,
		Cabe:     true,
		Bloqueio: bloqueio,
	}
	o, err := a.opcoesComputador(false, false)
	if err != nil {
		log.Printf("espaço: %v", err)
		return info
	}
	ctx, cancel := context.WithTimeout(a.ctx, tempoEspaco)
	defer cancel()
	precisa, livre, err := a.espaco(ctx, o)
	if err != nil {
		log.Printf("espaço: %v", err)
		return info
	}
	info.Precisa, info.Livre, info.Cabe = precisa, livre, livre >= precisa
	return info
}

// temChave diz se a chave guardada vale para este computador.
func (a *App) temChave() bool { return a.MinhaChave() != "" }

// MinhaChave devolve a chave guardada, normalizada, se ela vale para este
// computador. Sem chave, ou com a chave de outro computador, devolve "".
// A tela final e a de reinstalar a mostram com "Copiar a chave".
func (a *App) MinhaChave() string {
	dir, err := a.pastaChave()
	if err != nil {
		return ""
	}
	k, ok := chave.Normalizar(guarda.Ler(dir))
	if !ok {
		return ""
	}
	cod, err := a.lerCodigo()
	if err != nil || !a.conferir(k, cod) {
		return ""
	}
	return k
}

// ColarChave confere a chave com o código deste computador e, se valer, a
// guarda. Devolve "" quando deu certo, ou a frase do problema.
func (a *App) ColarChave(k string) string {
	norm, ok := chave.Normalizar(k)
	if !ok {
		return fraseChaveIncompleta
	}
	cod, err := a.lerCodigo()
	if err != nil {
		log.Printf("código do computador: %v", err)
		return fraseGenerica
	}
	if !a.conferir(norm, cod) {
		return fraseChaveDeOutro
	}
	dir, err := a.pastaChave()
	if err == nil {
		err = guarda.Gravar(dir, norm)
	}
	if err != nil {
		log.Printf("guardar a chave: %v", err)
		return fraseGenerica
	}
	return ""
}

// IrParaPagamento abre a página de pagamento no navegador e espera a chave
// voltar pela porta local. Quando ela chega, é conferida e guardada, e a
// tela recebe o evento "chave". Clicar de novo troca o servidor da volta:
// nunca ficam dois ligados.
func (a *App) IrParaPagamento() error {
	cod, err := a.lerCodigo()
	if err != nil {
		log.Printf("código do computador: %v", err)
		return errors.New(fraseGenerica)
	}
	a.mu.Lock()
	if a.pararVolta != nil {
		a.pararVolta()
		a.pararVolta = nil
	}
	ctx, cancel := context.WithCancel(a.ctx)
	s, err := volta.Ouvir(ctx)
	if err != nil {
		a.mu.Unlock()
		cancel()
		log.Printf("servidor da volta: %v", err)
		return errors.New(fraseGenerica)
	}
	parar := func() {
		cancel()
		s.Fechar()
	}
	a.pararVolta = parar
	a.mu.Unlock()
	go a.esperarChave(ctx, s, parar)
	a.abrirURL(a.ctx, urlCompra(ambiente.Worker, cod, s.Porta()))
	return nil
}

func (a *App) esperarChave(ctx context.Context, s *volta.Servidor, parar func()) {
	select {
	case k := <-s.Chaves():
		erro := a.ColarChave(k)
		a.emitir(a.ctx, "chave", EventoChave{Ok: erro == "", Erro: erro})
	case <-ctx.Done():
	}
	parar()
}

// encerrar roda quando a janela fecha: desliga o servidor da volta.
func (a *App) encerrar(context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pararVolta != nil {
		a.pararVolta()
		a.pararVolta = nil
	}
}

// opcoesComputador monta as opções da instalação com as pastas de verdade.
func (a *App) opcoesComputador(areaDeTrabalho, inicio bool) (gravar.OpcoesComputador, error) {
	cache, err := a.pastaCache()
	if err != nil {
		return gravar.OpcoesComputador{}, err
	}
	cacheChave, err := a.pastaChave()
	if err != nil {
		return gravar.OpcoesComputador{}, err
	}
	return gravar.OpcoesComputador{
		Cache:        cache,
		CacheChave:   cacheChave,
		URLManifesto: gravar.URLManifesto,
		// Sem timeout total: o download do modelo leva muitos minutos.
		HTTP:           &http.Client{},
		AreaDeTrabalho: areaDeTrabalho,
		Inicio:         inicio,
	}, nil
}

// InstalarComputador começa a instalação e devolve logo. O andamento chega
// pelo evento "progresso" e o resultado pelo evento "fim", como no Gravar.
// Sem chave valendo para este computador, não começa.
func (a *App) InstalarComputador(areaDeTrabalho, inicio bool) error {
	if !a.temChave() {
		return errors.New(instalacao.ErrSemChave.Error())
	}
	o, err := a.opcoesComputador(areaDeTrabalho, inicio)
	if err != nil {
		log.Printf("pastas da instalação: %v", err)
		return errors.New(fraseGenerica)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancelar != nil {
		return errors.New("Já tem uma instalação em andamento.")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancelar = cancel
	a.instalando = true
	go a.rodar(ctx, cancel, func(ctx context.Context, ev func(gravar.Evento)) error {
		return a.instalar(ctx, o, ev)
	})
	return nil
}

// AbrirIA abre a IA instalada, solta do instalador, e fecha o instalador.
func (a *App) AbrirIA() error {
	if err := a.abrirLancador(lancadorInstalado()); err != nil {
		log.Printf("abrir a IA: %v", err)
		return errors.New(fraseNaoAbriuIA)
	}
	a.sair(a.ctx)
	return nil
}
