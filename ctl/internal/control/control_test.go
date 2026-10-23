package control

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/dbconf"
	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/excel"
	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/faketoolbox"
	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/toolbox"
)

func TestMain(m *testing.M) {
	if os.Getenv(faketoolbox.EnvVar) == "1" {
		faketoolbox.Main()
		return
	}
	os.Exit(m.Run())
}

// fakeRunner falha pela senha e conta subidas simultâneas.
type fakeRunner struct {
	mu        sync.Mutex
	running   *dbconf.Config
	fail      map[string]error
	starts    []string
	stops     int
	active    int
	maxActive int
	delay     time.Duration
	reloads   []string
	reloadErr error
	planilhas *excel.Relatorio
	// relPorSenha, quando preenchido, troca planilhas a cada Start pela senha da config (nulo se não
	// houver), com sucesso ou falha: é o supervisor real, que guarda o relatório da última tentativa.
	relPorSenha map[string]*excel.Relatorio
}

func (f *fakeRunner) Start(cfg dbconf.Config) error {
	f.mu.Lock()
	f.active++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	f.starts = append(f.starts, cfg.Password)
	err, d := f.fail[cfg.Password], f.delay
	f.mu.Unlock()
	time.Sleep(d)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.active--
	if f.relPorSenha != nil {
		f.planilhas = f.relPorSenha[cfg.Password]
	}
	if err != nil {
		return err
	}
	c := cfg
	f.running = &c
	return nil
}

func (f *fakeRunner) Stop() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stops++
	f.running = nil
	return nil
}

func (f *fakeRunner) Running() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.running != nil
}

// Reload registra a pasta relida; quando falha, o toolbox fica como estava.
func (f *fakeRunner) Reload(cfg dbconf.Config) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reloads = append(f.reloads, cfg.Name)
	return f.reloadErr
}

func (f *fakeRunner) Planilhas() *excel.Relatorio {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.planilhas
}

func (f *fakeRunner) kill() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.running = nil
}

func str(s string) *string { return &s }

func input(pw string) dbconf.Input {
	return dbconf.Input{Motor: "postgres", Host: "db.local", Porta: json.RawMessage(`5432`), Base: "cd2", Usuario: "leitor", Senha: str(pw)}
}

func saved(pw string) dbconf.Config {
	return dbconf.Config{Engine: "postgres", Host: "db.local", Port: "5432", Name: "cd2", User: "leitor", Password: pw, Schema: "public"}
}

var boom = &toolbox.StartError{Msg: toolbox.MsgDied, Log: []string{"Error: toolbox failed to initialize"}}

func newCtl(t *testing.T, run Runner) (*Controller, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "db.env")
	return New(path, run), path
}

func TestBootSemDbEnv(t *testing.T) {
	run := &fakeRunner{}
	c, _ := newCtl(t, run)
	<-c.BootAsync()
	st := c.Status()
	if st.Estado != SemBanco || st.Motor != "" || st.SenhaSalva || st.Erro != "" || len(run.starts) != 0 {
		t.Errorf("status %+v, subidas %v", st, run.starts)
	}
}

func TestBootComDbEnvDaV050(t *testing.T) {
	run := &fakeRunner{}
	c, path := newCtl(t, run)
	b, err := os.ReadFile(filepath.Join("..", "dbconf", "testdata", "v050-6.env"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	<-c.BootAsync()
	st := c.Status()
	want := Status{Estado: Rodando, Motor: "postgres", Host: "db.local", Porta: 5432, Base: "cd2", Usuario: "leitor", Schema: "public", SenhaSalva: true}
	if !reflect.DeepEqual(st, want) {
		t.Errorf("status %+v", st)
	}
	if !reflect.DeepEqual(run.starts, []string{`$x y'z#`}) {
		t.Errorf("subiu com %q", run.starts)
	}
}

func TestBootQueFalhaFicaFalhou(t *testing.T) {
	run := &fakeRunner{fail: map[string]error{"ruim": boom}}
	c, path := newCtl(t, run)
	if err := dbconf.Save(path, saved("ruim")); err != nil {
		t.Fatal(err)
	}
	<-c.BootAsync()
	st := c.Status()
	if st.Estado != Falhou || st.Erro != toolbox.MsgDied || len(st.Log) != 1 || st.Motor != "postgres" {
		t.Errorf("status %+v", st)
	}
}

func TestBootComDbEnvInvalido(t *testing.T) {
	run := &fakeRunner{}
	c, path := newCtl(t, run)
	if err := os.WriteFile(path, []byte("lixo sem motor\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	<-c.BootAsync()
	st := c.Status()
	if st.Estado != SemBanco || !strings.Contains(st.Erro, "inválido") || len(run.starts) != 0 {
		t.Errorf("status %+v", st)
	}
}

func TestApplyPrimeiraVez(t *testing.T) {
	run := &fakeRunner{}
	c, path := newCtl(t, run)
	<-c.BootAsync()
	st, err := c.Apply(input("s1"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Estado != Rodando || st.Schema != "public" || st.Porta != 5432 || st.Erro != "" || !st.SenhaSalva {
		t.Errorf("status %+v", st)
	}
	got, err := dbconf.Load(path)
	if err != nil || got != saved("s1") {
		t.Errorf("db.env %+v, %v", got, err)
	}
}

func TestApplyRuimVoltaParaAnteriorENaoGrava(t *testing.T) {
	run := &fakeRunner{fail: map[string]error{"ruim": boom}}
	c, path := newCtl(t, run)
	if err := dbconf.Save(path, saved("s1")); err != nil {
		t.Fatal(err)
	}
	<-c.BootAsync()
	before, _ := os.ReadFile(path)
	st, err := c.Apply(input("ruim"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Estado != Rodando || st.Erro != toolbox.MsgDied || !reflect.DeepEqual(st.Log, boom.Log) {
		t.Errorf("status %+v", st)
	}
	if !reflect.DeepEqual(run.starts, []string{"s1", "ruim", "s1"}) {
		t.Errorf("subidas %q", run.starts)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Error("db.env mudou depois da falha")
	}
	if !run.Running() || run.running.Password != "s1" {
		t.Error("a conexão anterior não voltou")
	}
}

func TestApplyRuimSemAnterior(t *testing.T) {
	run := &fakeRunner{fail: map[string]error{"ruim": boom}}
	c, path := newCtl(t, run)
	<-c.BootAsync()
	st, err := c.Apply(input("ruim"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Estado != SemBanco || st.Erro != toolbox.MsgDied || st.Motor != "" {
		t.Errorf("status %+v", st)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("gravou db.env de uma conexão que falhou")
	}
}

func TestApplySenhaVaziaMantemASalva(t *testing.T) {
	run := &fakeRunner{}
	c, path := newCtl(t, run)
	if err := dbconf.Save(path, saved("s1")); err != nil {
		t.Fatal(err)
	}
	<-c.BootAsync()
	for _, s := range []*string{nil, str("")} {
		in := input("")
		in.Senha = s
		if _, err := c.Apply(in); err != nil {
			t.Fatal(err)
		}
		got, _ := dbconf.Load(path)
		if got.Password != "s1" {
			t.Errorf("senha virou %q", got.Password)
		}
	}
}

func TestApplyCampoInvalidoNaoMexeEmNada(t *testing.T) {
	run := &fakeRunner{}
	c, path := newCtl(t, run)
	if err := dbconf.Save(path, saved("s1")); err != nil {
		t.Fatal(err)
	}
	<-c.BootAsync()
	in := input("x")
	in.Host = ""
	_, err := c.Apply(in)
	var fe *dbconf.FieldError
	if !errors.As(err, &fe) || err.Error() != "host: obrigatório" {
		t.Fatalf("erro %v", err)
	}
	if run.stops != 0 || len(run.starts) != 1 || c.Status().Estado != Rodando {
		t.Errorf("mexeu: stops %d starts %v", run.stops, run.starts)
	}
}

func TestClear(t *testing.T) {
	run := &fakeRunner{}
	c, path := newCtl(t, run)
	if err := dbconf.Save(path, saved("s1")); err != nil {
		t.Fatal(err)
	}
	<-c.BootAsync()
	st := c.Clear()
	if st.Estado != SemBanco || st.Motor != "" || st.SenhaSalva {
		t.Errorf("status %+v", st)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("db.env continua lá")
	}
	if run.Running() {
		t.Error("toolbox continua de pé")
	}
}

func TestToolboxQueMorreViraFalhou(t *testing.T) {
	run := &fakeRunner{}
	c, _ := newCtl(t, run)
	<-c.BootAsync()
	if _, err := c.Apply(input("s1")); err != nil {
		t.Fatal(err)
	}
	run.kill()
	st := c.Status()
	if st.Estado != Falhou || st.Erro != MsgStopped {
		t.Errorf("status %+v", st)
	}
}

func TestPedidosUmDeCadaVez(t *testing.T) {
	run := &fakeRunner{delay: 50 * time.Millisecond}
	c, _ := newCtl(t, run)
	<-c.BootAsync()
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = c.Apply(input(fmt.Sprint("s", i)))
		}(i)
	}
	wg.Wait()
	if run.maxActive != 1 {
		t.Errorf("%d subidas ao mesmo tempo", run.maxActive)
	}
}

func TestArranqueSeguraOPrimeiroPedido(t *testing.T) {
	run := &fakeRunner{delay: 100 * time.Millisecond}
	c, path := newCtl(t, run)
	if err := dbconf.Save(path, saved("s1")); err != nil {
		t.Fatal(err)
	}
	done := c.BootAsync()
	if _, err := c.Apply(input("s2")); err != nil {
		t.Fatal(err)
	}
	<-done
	if !reflect.DeepEqual(run.starts, []string{"s1", "s2"}) || run.running.Password != "s2" {
		t.Errorf("subidas %q", run.starts)
	}
}

func TestStatusNuncaTemSenha(t *testing.T) {
	run := &fakeRunner{fail: map[string]error{"senha-ruim": boom}}
	c, _ := newCtl(t, run)
	<-c.BootAsync()
	st1, _ := c.Apply(input("senha-boa"))
	st2, _ := c.Apply(input("senha-ruim"))
	for _, st := range []Status{st1, st2, c.Status()} {
		b, _ := json.Marshal(st)
		if strings.Contains(string(b), "senha-boa") || strings.Contains(string(b), "senha-ruim") {
			t.Errorf("senha no JSON: %s", b)
		}
	}
}

// Rollback com o supervisor de verdade e o toolbox falso.
func TestRollbackComToolboxFalso(t *testing.T) {
	t.Setenv(faketoolbox.EnvVar, "1")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	sup := toolbox.New(toolbox.Options{Source: self, CacheDir: t.TempDir(), RecipeDir: "receitas", Port: port, HarnessPort: 18080, ReadyTimeout: 10 * time.Second})
	c, path := newCtl(t, sup)
	t.Cleanup(c.Shutdown)
	<-c.BootAsync()
	if st, _ := c.Apply(input("boa1")); st.Estado != Rodando {
		t.Fatalf("primeira: %+v", st)
	}
	st, err := c.Apply(input("morre-x"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Estado != Rodando || st.Erro != toolbox.MsgDied || len(st.Log) == 0 {
		t.Errorf("status %+v", st)
	}
	got, _ := dbconf.Load(path)
	if got.Password != "boa1" {
		t.Errorf("db.env ficou com %q", got.Password)
	}
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	if err != nil {
		t.Fatalf("toolbox anterior não voltou: %v", err)
	}
	resp.Body.Close()
}

func TestRollbackQueTambemFalha(t *testing.T) {
	prevErr := &toolbox.StartError{Msg: "anterior morreu", Log: []string{"log anterior"}}
	run := &fakeRunner{fail: map[string]error{"ruim": boom}}
	c, path := newCtl(t, run)
	if err := dbconf.Save(path, saved("s1")); err != nil {
		t.Fatal(err)
	}
	<-c.BootAsync()
	run.mu.Lock()
	run.fail["s1"] = prevErr
	run.mu.Unlock()
	st, err := c.Apply(input("ruim"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Estado != Falhou || st.Host != "db.local" || st.Usuario != "leitor" || !st.SenhaSalva {
		t.Errorf("status %+v", st)
	}
	if !strings.Contains(st.Erro, toolbox.MsgDied) || !strings.Contains(st.Erro, "anterior morreu") {
		t.Errorf("erro %q", st.Erro)
	}
	if !reflect.DeepEqual(st.Log, prevErr.Log) {
		t.Errorf("log %v", st.Log)
	}
}

func TestShutdownEFinal(t *testing.T) {
	run := &fakeRunner{}
	c, _ := newCtl(t, run)
	<-c.BootAsync()
	c.Shutdown()
	if _, err := c.Apply(input("s1")); err != nil {
		t.Fatal(err)
	}
	st := c.Clear()
	<-c.BootAsync()
	if len(run.starts) != 0 || st.Erro != MsgClosed || run.Running() {
		t.Errorf("subidas %v, status %+v", run.starts, st)
	}
}

// inputArquivo é a tela de um motor de arquivo: só o motor e a pasta.
func inputArquivo(motor, pasta string) dbconf.Input {
	return dbconf.Input{Motor: motor, Base: pasta}
}

func TestRelerComExcel(t *testing.T) {
	run := &fakeRunner{}
	c, _ := newCtl(t, run)
	<-c.BootAsync()
	if st, err := c.Apply(inputArquivo("excel", "/planilhas")); err != nil || st.Estado != Rodando {
		t.Fatalf("Apply: %+v %v", st, err)
	}
	st, err := c.Reler()
	if err != nil {
		t.Fatal(err)
	}
	if st.Estado != Rodando || st.Erro != "" || st.Motor != "excel" || st.Base != "/planilhas" {
		t.Errorf("status %+v", st)
	}
	if !reflect.DeepEqual(run.reloads, []string{"/planilhas"}) {
		t.Errorf("releituras %q", run.reloads)
	}
}

func TestRelerComDbf(t *testing.T) {
	run := &fakeRunner{}
	c, _ := newCtl(t, run)
	<-c.BootAsync()
	if st, err := c.Apply(inputArquivo("dbf", "/dados")); err != nil || st.Estado != Rodando {
		t.Fatalf("Apply: %+v %v", st, err)
	}
	st, err := c.Reler()
	if err != nil {
		t.Fatal(err)
	}
	if st.Estado != Rodando || st.Erro != "" || st.Motor != "dbf" {
		t.Errorf("status %+v", st)
	}
	if !reflect.DeepEqual(run.reloads, []string{"/dados"}) {
		t.Errorf("releituras %q", run.reloads)
	}
}

func TestRelerFalhaNaImportacaoMantemRodando(t *testing.T) {
	run := &fakeRunner{reloadErr: &toolbox.StartError{Msg: "x", Log: []string{"linha"}}}
	c, _ := newCtl(t, run)
	<-c.BootAsync()
	if _, err := c.Apply(inputArquivo("excel", "/planilhas")); err != nil {
		t.Fatal(err)
	}
	stops, starts := run.stops, len(run.starts)
	st, err := c.Reler()
	if err != nil {
		t.Fatal(err)
	}
	if st.Estado != Rodando || st.Erro != "x" || !reflect.DeepEqual(st.Log, []string{"linha"}) || st.Motor != "excel" {
		t.Errorf("status %+v", st)
	}
	if run.stops != stops || len(run.starts) != starts {
		t.Errorf("mexeu no toolbox: stops %d→%d, subidas %d→%d", stops, run.stops, starts, len(run.starts))
	}
}

// Se o Reload falhou com o toolbox já derrubado, a mesma conexão sobe de novo (rollback).
func TestRelerFalhaComToolboxParadoSobeDeNovo(t *testing.T) {
	run := &fakeRunner{reloadErr: &toolbox.StartError{Msg: "x"}}
	c, _ := newCtl(t, run)
	<-c.BootAsync()
	if _, err := c.Apply(inputArquivo("excel", "/planilhas")); err != nil {
		t.Fatal(err)
	}
	run.kill()
	st, err := c.Reler()
	if err != nil {
		t.Fatal(err)
	}
	if st.Estado != Rodando || st.Erro != "x" || st.Motor != "excel" {
		t.Errorf("status %+v", st)
	}
	if len(run.starts) != 2 || !run.Running() {
		t.Errorf("não subiu de novo: subidas %q", run.starts)
	}
}

func TestRelerSemConexaoENaoArquivo(t *testing.T) {
	run := &fakeRunner{}
	c, _ := newCtl(t, run)
	<-c.BootAsync()
	if _, err := c.Reler(); !errors.Is(err, ErrNaoArquivo) {
		t.Errorf("sem conexão: erro %v", err)
	}
	if _, err := c.Apply(input("s1")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Reler(); !errors.Is(err, ErrNaoArquivo) {
		t.Errorf("postgres: erro %v", err)
	}
	if len(run.reloads) != 0 {
		t.Errorf("releu sem ser de arquivo: %q", run.reloads)
	}
	if st := c.Status(); st.Estado != Rodando || st.Erro != "" {
		t.Errorf("status mudou: %+v", st)
	}
}

func TestRelerDepoisDoShutdown(t *testing.T) {
	run := &fakeRunner{}
	c, _ := newCtl(t, run)
	<-c.BootAsync()
	if _, err := c.Apply(inputArquivo("excel", "/planilhas")); err != nil {
		t.Fatal(err)
	}
	c.Shutdown()
	st, err := c.Reler()
	if err != nil {
		t.Fatal(err)
	}
	if st.Erro != MsgClosed || len(run.reloads) != 0 {
		t.Errorf("status %+v, releituras %q", st, run.reloads)
	}
}

func TestStatusLevaPlanilhas(t *testing.T) {
	rel := &excel.Relatorio{Arquivos: 1, Tabelas: 1}
	run := &fakeRunner{planilhas: rel}
	c, _ := newCtl(t, run)
	<-c.BootAsync()
	if _, err := c.Apply(inputArquivo("excel", "/planilhas")); err != nil {
		t.Fatal(err)
	}
	if got := c.Status().Planilhas; got != rel {
		t.Errorf("planilhas %p, quero %p", got, rel)
	}
	run.mu.Lock()
	run.planilhas = nil
	run.mu.Unlock()
	if _, err := c.Apply(input("s1")); err != nil {
		t.Fatal(err)
	}
	st := c.Status()
	if st.Planilhas != nil {
		t.Errorf("postgres com planilhas: %+v", st.Planilhas)
	}
	if b, _ := json.Marshal(st); strings.Contains(string(b), "planilhas") {
		t.Errorf("JSON sem relatório não deveria ter a chave: %s", b)
	}
}

// Primeira conexão que falha: o rollback zera a conexão, mas o relatório da tentativa fica.
func TestApplyFalhaSemAnteriorGuardaPlanilhas(t *testing.T) {
	rel := &excel.Relatorio{Arquivos: 1, Ignoradas: []excel.Ignorada{{Arquivo: "a.xlsx", Motivo: "aba vazia"}}}
	run := &fakeRunner{planilhas: rel, fail: map[string]error{"": boom}}
	c, _ := newCtl(t, run)
	<-c.BootAsync()
	st, err := c.Apply(inputArquivo("excel", "/planilhas"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Estado != SemBanco || st.Erro == "" || st.Planilhas != rel {
		t.Errorf("status %+v", st)
	}
	if got := c.Status().Planilhas; got != rel {
		t.Errorf("planilhas %p, quero %p", got, rel)
	}
}

// Com conexão anterior, o Start(prev) do rollback troca o relatório do runner; o da tentativa vale.
func TestApplyFalhaComAnteriorGuardaPlanilhasDaTentativa(t *testing.T) {
	rel := &excel.Relatorio{Arquivos: 2, Ignoradas: []excel.Ignorada{{Arquivo: "b.xlsx", Motivo: "o arquivo não abriu"}}}
	run := &fakeRunner{
		fail:        map[string]error{"ruim": boom},
		relPorSenha: map[string]*excel.Relatorio{"ruim": rel},
	}
	c, _ := newCtl(t, run)
	<-c.BootAsync()
	if _, err := c.Apply(input("s1")); err != nil {
		t.Fatal(err)
	}
	if c.Status().Planilhas != nil {
		t.Fatalf("postgres com planilhas: %+v", c.Status().Planilhas)
	}
	st, err := c.Apply(input("ruim"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Estado != Rodando || st.Erro == "" || st.Planilhas != rel {
		t.Errorf("status %+v", st)
	}
	if run.Planilhas() != nil {
		t.Errorf("o fake deveria ter trocado para nulo no rollback")
	}
}

func TestClearZeraPlanilhas(t *testing.T) {
	rel := &excel.Relatorio{Arquivos: 1, Tabelas: 1}
	run := &fakeRunner{planilhas: rel}
	c, _ := newCtl(t, run)
	<-c.BootAsync()
	if _, err := c.Apply(inputArquivo("excel", "/planilhas")); err != nil {
		t.Fatal(err)
	}
	if c.Status().Planilhas != rel {
		t.Fatalf("Apply não guardou o relatório")
	}
	if st := c.Clear(); st.Planilhas != nil {
		t.Errorf("Clear deixou planilhas: %+v", st.Planilhas)
	}
	if c.Status().Planilhas != nil {
		t.Errorf("Status depois do Clear com planilhas")
	}
}

func TestRelerFalhaGuardaPlanilhasDaTentativa(t *testing.T) {
	rel := &excel.Relatorio{Arquivos: 3, Ignoradas: []excel.Ignorada{{Arquivo: "c.xlsx", Motivo: "aba vazia"}}}
	run := &fakeRunner{reloadErr: &toolbox.StartError{Msg: "x"}}
	c, _ := newCtl(t, run)
	<-c.BootAsync()
	if _, err := c.Apply(inputArquivo("excel", "/planilhas")); err != nil {
		t.Fatal(err)
	}
	run.mu.Lock()
	run.planilhas = rel // o Reload que falha deixa o relatório da tentativa no runner
	run.mu.Unlock()
	st, err := c.Reler()
	if err != nil {
		t.Fatal(err)
	}
	if st.Estado != Rodando || st.Erro != "x" || st.Planilhas != rel {
		t.Errorf("status %+v", st)
	}
}
