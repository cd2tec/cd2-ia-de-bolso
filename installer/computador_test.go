package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/ambiente"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/chave"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/guarda"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/gravar"
)

// vetor lê o vetor de teste da chave: a pública, o código e a chave.
func vetor(t *testing.T) (ed25519.PublicKey, string, string) {
	t.Helper()
	b, err := os.ReadFile("../instalacao/testdata/vetor.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		PublicaBase64 string `json:"publicaBase64"`
		Codigo        string `json:"codigo"`
		Chave         string `json:"chave"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	p, err := base64.StdEncoding.DecodeString(v.PublicaBase64)
	if err != nil {
		t.Fatal(err)
	}
	return ed25519.PublicKey(p), v.Codigo, v.Chave
}

// maquina é o computador de mentira de um teste: o código, a pasta da chave,
// o espaço e quantas vezes a instalação foi chamada.
type maquina struct {
	pub           ed25519.PublicKey
	codigo        string
	pastaChave    string
	precisa       int64
	livre         int64
	mu            sync.Mutex
	instalou      int
	recebeu       gravar.OpcoesComputador
	urls          []string
	saiu          int
	chaves        chan EventoChave
	reabriu       []string
	erroReabrir   error
	elevado       bool
	abriuLancador string
}

func (m *maquina) app() *App {
	a := NovaApp()
	a.ctx = context.Background()
	a.emitir = func(_ context.Context, nome string, dados ...any) {
		if nome == "chave" {
			m.chaves <- dados[0].(EventoChave)
		}
	}
	a.pastaCache = func() (string, error) { return filepath.Join(m.pastaChave, "usb"), nil }
	a.pastaChave = func() (string, error) { return m.pastaChave, nil }
	a.lerCodigo = func() (string, error) { return m.codigo, nil }
	a.conferir = func(k, c string) bool { return chave.ConferirCom(m.pub, k, c) }
	a.espaco = func(context.Context, gravar.OpcoesComputador) (int64, int64, error) {
		return m.precisa, m.livre, nil
	}
	a.instalar = func(_ context.Context, o gravar.OpcoesComputador, ev func(gravar.Evento)) error {
		m.mu.Lock()
		m.instalou++
		m.recebeu = o
		m.mu.Unlock()
		ev(gravar.Evento{Etapa: "pronto", Frase: "Pronto. A IA de Bolso está instalada neste computador.", Fracao: 1})
		return nil
	}
	a.abrirURL = func(_ context.Context, u string) {
		m.mu.Lock()
		m.urls = append(m.urls, u)
		m.mu.Unlock()
	}
	a.sair = func(context.Context) {
		m.mu.Lock()
		m.saiu++
		m.mu.Unlock()
	}
	a.elevado = func() bool { return m.elevado }
	a.reabrir = func(exe string, args []string) error {
		m.reabriu = append([]string{exe}, args...)
		return m.erroReabrir
	}
	a.abrirLancador = func(caminho string) error {
		m.abriuLancador = caminho
		return nil
	}
	return a
}

func novaMaquina(t *testing.T) *maquina {
	pub, cod, _ := vetor(t)
	return &maquina{
		pub: pub, codigo: cod,
		pastaChave: filepath.Join(t.TempDir(), "cd2-harness"),
		precisa:    7e9, livre: 50e9,
		chaves: make(chan EventoChave, 4),
	}
}

func TestColarChaveDeOutroComputadorNaoBaixa(t *testing.T) {
	m := novaMaquina(t)
	m.codigo = "ZZZZ-0X06-DV8S" // outro computador
	_, _, chv := vetor(t)
	a := m.app()
	if got := a.ColarChave(chv); got != "Esta chave não é deste computador." {
		t.Fatalf("ColarChave = %q", got)
	}
	if _, err := os.Stat(filepath.Join(m.pastaChave, "instalacao.chave")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("gravou a chave de outro computador: %v", err)
	}
	if a.InfoInstalar().TemChave {
		t.Fatal("TemChave sem chave valendo")
	}
	if err := a.InstalarComputador(true, true); err == nil || err.Error() != "Falta a chave deste computador." {
		t.Fatalf("InstalarComputador = %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.instalou != 0 {
		t.Fatal("chamou a instalação sem chave deste computador")
	}
}

func TestColarChaveIncompleta(t *testing.T) {
	m := novaMaquina(t)
	_, _, chv := vetor(t)
	a := m.app()
	for _, k := range []string{"", "abc", chv[:len(chv)-5]} {
		if got := a.ColarChave(k); got != "Esta chave não está completa. Confira se copiou ela inteira." {
			t.Fatalf("ColarChave(%q) = %q", k, got)
		}
	}
	if _, err := os.Stat(filepath.Join(m.pastaChave, "instalacao.chave")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("gravou chave incompleta: %v", err)
	}
}

func TestColarChaveDepoisDeReabrir(t *testing.T) {
	m := novaMaquina(t)
	_, _, chv := vetor(t)
	// O instalador do pagamento sumiu: uma App nova recebe a chave colada.
	a := m.app()
	if a.InfoInstalar().TemChave {
		t.Fatal("TemChave antes de colar")
	}
	suja := "  " + strings.ToLower(chv[:30]) + "\n" + chv[30:] + " "
	if got := a.ColarChave(suja); got != "" {
		t.Fatalf("ColarChave = %q", got)
	}
	quer, _ := chave.Normalizar(chv)
	if got := guarda.Ler(m.pastaChave); got != quer {
		t.Fatalf("guardou %q, quer %q", got, quer)
	}
	if !a.InfoInstalar().TemChave {
		t.Fatal("TemChave continua false depois de colar")
	}
	// E outra App, aberta depois, já acha a chave e instala sem pagar.
	b := m.app()
	if !b.InfoInstalar().TemChave {
		t.Fatal("a App reaberta não achou a chave")
	}
	fim := make(chan Fim, 1)
	b.emitir = func(_ context.Context, nome string, dados ...any) {
		if nome == "fim" {
			fim <- dados[0].(Fim)
		}
	}
	if err := b.InstalarComputador(false, true); err != nil {
		t.Fatal(err)
	}
	select {
	case f := <-fim:
		if !f.Ok || f.Frase != "Pronto. A IA de Bolso está instalada neste computador." {
			t.Fatalf("fim = %+v", f)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("o evento fim não chegou")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	o := m.recebeu
	if m.instalou != 1 || o.CacheChave != m.pastaChave || o.Cache != filepath.Join(m.pastaChave, "usb") ||
		o.AreaDeTrabalho || !o.Inicio || o.URLManifesto != gravar.URLManifesto || o.HTTP == nil || o.HTTP.Timeout != 0 {
		t.Fatalf("instalou %d vezes com %+v", m.instalou, o)
	}
}

func TestInfoSemEspacoBloqueiaPagamento(t *testing.T) {
	m := novaMaquina(t)
	m.precisa, m.livre = 7_000_000_000, 6_999_999_999
	info := m.app().InfoInstalar()
	if info.Cabe || info.Precisa != 7_000_000_000 || info.Livre != 6_999_999_999 {
		t.Fatalf("info = %+v", info)
	}
	if info.Codigo != m.codigo || info.Worker != ambiente.Worker || info.Sandbox != ambiente.Sandbox || info.Bloqueio != "" {
		t.Fatalf("info = %+v", info)
	}
	m.livre = 7_000_000_000
	if !m.app().InfoInstalar().Cabe {
		t.Fatal("cabe certinho e deu Cabe false")
	}
}

func TestInfoJSONEmMinusculas(t *testing.T) {
	b, err := json.Marshal(InfoInstalar{})
	if err != nil {
		t.Fatal(err)
	}
	quer := `{"codigo":"","temChave":false,"worker":"","sandbox":false,"precisa":0,"livre":0,"cabe":false,"bloqueio":""}`
	if string(b) != quer {
		t.Fatalf("json = %s", b)
	}
}

func TestURLDeCompra(t *testing.T) {
	got := urlCompra(ambiente.Worker, "BCNK-0X06-DV8S", 5000)
	quer := ambiente.Worker + "/comprar?codigo=BCNK-0X06-DV8S&porta=5000&de=instalador"
	if got != quer {
		t.Fatalf("url = %q, quer %q", got, quer)
	}
}

// voltar faz o que o botão da página da chave faz: chama a porta local.
func voltar(t *testing.T, porta int, k string) int {
	t.Helper()
	r, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/?chave=%s", porta, k))
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	return r.StatusCode
}

// portaDa tira a porta da URL de compra.
func portaDa(t *testing.T, u string) int {
	t.Helper()
	var p int
	i := strings.Index(u, "porta=")
	if _, err := fmt.Sscanf(u[i:], "porta=%d", &p); err != nil {
		t.Fatalf("sem porta em %q", u)
	}
	return p
}

func TestIrParaPagamentoRecebeAChave(t *testing.T) {
	m := novaMaquina(t)
	_, _, chv := vetor(t)
	a := m.app()
	if err := a.IrParaPagamento(); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	u := m.urls[0]
	m.mu.Unlock()
	if !strings.HasPrefix(u, ambiente.Worker+"/comprar?codigo="+m.codigo+"&porta=") || !strings.HasSuffix(u, "&de=instalador") {
		t.Fatalf("url = %q", u)
	}
	if st := voltar(t, portaDa(t, u), chv); st != 200 {
		t.Fatalf("status %d", st)
	}
	select {
	case e := <-m.chaves:
		if !e.Ok || e.Erro != "" {
			t.Fatalf("evento chave = %+v", e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("o evento chave não chegou")
	}
	if !a.InfoInstalar().TemChave {
		t.Fatal("a chave que voltou não foi guardada")
	}
}

func TestIrParaPagamentoChaveDeOutroComputador(t *testing.T) {
	m := novaMaquina(t)
	m.codigo = "ZZZZ-0X06-DV8S"
	_, _, chv := vetor(t)
	a := m.app()
	if err := a.IrParaPagamento(); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	u := m.urls[0]
	m.mu.Unlock()
	voltar(t, portaDa(t, u), chv)
	select {
	case e := <-m.chaves:
		if e.Ok || e.Erro != "Esta chave não é deste computador." {
			t.Fatalf("evento chave = %+v", e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("o evento chave não chegou")
	}
}

func TestIrParaPagamentoDuasVezesDeixaUmServidor(t *testing.T) {
	m := novaMaquina(t)
	_, _, chv := vetor(t)
	a := m.app()
	if err := a.IrParaPagamento(); err != nil {
		t.Fatal(err)
	}
	if err := a.IrParaPagamento(); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	u1, u2 := m.urls[0], m.urls[1]
	m.mu.Unlock()
	p1, p2 := portaDa(t, u1), portaDa(t, u2)
	if p1 == p2 {
		t.Fatalf("a mesma porta duas vezes: %d", p1)
	}
	// O primeiro servidor fecha (a conexão é recusada).
	prazo := time.Now().Add(2 * time.Second)
	for {
		r, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", p1))
		if err != nil {
			break
		}
		r.Body.Close()
		if time.Now().After(prazo) {
			t.Fatal("o primeiro servidor continua vivo")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if st := voltar(t, p2, chv); st != 200 {
		t.Fatalf("status %d", st)
	}
	select {
	case e := <-m.chaves:
		if !e.Ok {
			t.Fatalf("evento chave = %+v", e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("o evento chave não chegou")
	}
	a.encerrar(context.Background())
	if _, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", p2)); err == nil {
		t.Fatal("o servidor continua vivo depois de encerrar")
	}
}

func TestEscolherInstalar(t *testing.T) {
	// Sem administrador (ou fora do Windows) não reabre nada.
	m := novaMaquina(t)
	a := m.app()
	if got := a.EscolherInstalar(); got != "" || m.reabriu != nil || m.saiu != 0 {
		t.Fatalf("EscolherInstalar = %q, reabriu %q, saiu %d", got, m.reabriu, m.saiu)
	}

	// Como administrador, reabre com --instalar e sai.
	m.elevado = true
	a = m.app()
	if got := a.EscolherInstalar(); got != "" {
		t.Fatalf("EscolherInstalar = %q", got)
	}
	if len(m.reabriu) != 2 || m.reabriu[0] == "" || m.reabriu[1] != "--instalar" || m.saiu != 1 {
		t.Fatalf("reabriu %q, saiu %d", m.reabriu, m.saiu)
	}

	// Se não deu para reabrir, bloqueia o caminho de instalar e não sai.
	m.erroReabrir = errors.New("sem Explorer")
	a = m.app()
	quer := "Abra o instalador de novo sem ser como administrador (clique duplo normal) para instalar neste computador."
	if got := a.EscolherInstalar(); got != quer {
		t.Fatalf("EscolherInstalar = %q", got)
	}
	if m.saiu != 1 {
		t.Fatal("saiu sem ter reaberto")
	}
	if got := a.InfoInstalar().Bloqueio; got != quer {
		t.Fatalf("Bloqueio = %q", got)
	}
}

func TestAbrirIA(t *testing.T) {
	m := novaMaquina(t)
	a := m.app()
	if err := a.AbrirIA(); err != nil {
		t.Fatal(err)
	}
	if m.abriuLancador == "" || m.saiu != 1 {
		t.Fatalf("abriu %q, saiu %d", m.abriuLancador, m.saiu)
	}
	m.saiu = 0
	a.abrirLancador = func(string) error { return errors.New("não existe") }
	if err := a.AbrirIA(); err == nil || m.saiu != 0 {
		t.Fatalf("err = %v, saiu %d", err, m.saiu)
	}
}

func TestIniciouComInstalar(t *testing.T) {
	a := NovaApp()
	if a.IniciouComInstalar() {
		t.Fatal("sem --instalar")
	}
	a.comInstalar = true
	if !a.IniciouComInstalar() {
		t.Fatal("com --instalar")
	}
}
