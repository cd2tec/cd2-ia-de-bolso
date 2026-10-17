package sessao

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/modelo"
	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/motor"
)

const chaveTeste = "abc123"

// procFalso faz o papel do motor ou do cd2-ctl: Parar fecha o Morreu e anota
// o nome na ordem compartilhada.
type procFalso struct {
	nome   string
	morreu chan struct{}
	once   sync.Once
	ordem  *registro
	falha  error // o que o Parar devolve (nil: parou bem)
}

func novoProc(nome string, r *registro) *procFalso {
	return &procFalso{nome: nome, morreu: make(chan struct{}), ordem: r}
}

func (p *procFalso) Pid() int                { return 1 }
func (p *procFalso) Morreu() <-chan struct{} { return p.morreu }
func (p *procFalso) Parar() error {
	p.once.Do(func() {
		p.ordem.anotar("parou " + p.nome)
		close(p.morreu)
	})
	return p.falha
}

// morrer simula o processo saindo sozinho, sem Parar.
func (p *procFalso) morrer() { p.once.Do(func() { close(p.morreu) }) }

func (p *procFalso) parado() bool {
	select {
	case <-p.morreu:
		return true
	default:
		return false
	}
}

// registro guarda, em ordem, chamadas e estados vindos de goroutines diferentes.
type registro struct {
	mu    sync.Mutex
	itens []string
}

func (r *registro) anotar(s string) {
	r.mu.Lock()
	r.itens = append(r.itens, s)
	r.mu.Unlock()
}

func (r *registro) lista() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.itens...)
}

// espiao recebe os estados do aoMudar e deixa o teste esperar por uma fase.
type espiao struct {
	mu      sync.Mutex
	estados []Estado
	mudou   chan struct{}
	ordem   *registro
}

func novoEspiao(r *registro) *espiao { return &espiao{mudou: make(chan struct{}, 1000), ordem: r} }

func (e *espiao) aoMudar(est Estado) {
	e.mu.Lock()
	e.estados = append(e.estados, est)
	e.mu.Unlock()
	e.ordem.anotar("fase " + string(est.Fase))
	e.mudou <- struct{}{}
}

func (e *espiao) todos() []Estado {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]Estado(nil), e.estados...)
}

func (e *espiao) ultimo() Estado {
	t := e.todos()
	if len(t) == 0 {
		return Estado{}
	}
	return t[len(t)-1]
}

// esperar volta quando algum estado já recebido satisfaz ok.
func (e *espiao) esperar(t *testing.T, oque string, ok func(Estado) bool) Estado {
	t.Helper()
	limite := time.After(5 * time.Second)
	for {
		for _, est := range e.todos() {
			if ok(est) {
				return est
			}
		}
		select {
		case <-e.mudou:
		case <-limite:
			t.Fatalf("não chegou %s; estados: %+v", oque, e.todos())
		}
	}
}

func fase(f Fase) func(Estado) bool { return func(e Estado) bool { return e.Fase == f } }

// cenario monta Deps falsos que dão certo; cada teste muda o que precisa.
type cenario struct {
	ordem    *registro
	motor    *procFalso
	ctl      *procFalso
	abriu    chan string
	cfgMotor motor.Config
	argsCtl  []string
	chaveCtl string
	portaCtl int
	mu       sync.Mutex
	subiuMot int
	deps     Deps
}

func novoCenario() *cenario {
	c := &cenario{ordem: &registro{}, abriu: make(chan string, 10)}
	c.motor = novoProc("motor", c.ordem)
	c.ctl = novoProc("ctl", c.ordem)
	c.deps = Deps{
		Raiz:         "/pen",
		GOOS:         "darwin",
		PID:          4242,
		Cache:        "/cache",
		Logs:         "/logs",
		PortaInicial: 8080,
		Ctx:          "8192",
		AcharModelo:  func() (string, error) { return "/pen/models/m.gguf", nil },
		PrepararCache: func() (string, string, error) {
			return "/cache/llamafile", "/cache/cd2-ctl", nil
		},
		PortaLivre: func(int) bool { return true },
		NovaChave:  func() (string, error) { return chaveTeste, nil },
		Ler: func(ctx context.Context, _ string, prog func(feitos, total int64)) error {
			for _, f := range []int64{0, 30, 60, 100, 120} {
				prog(f, 100)
			}
			return nil
		},
		SubirMotor: func(ctx context.Context, cfg motor.Config, aviso func(string)) (Processo, error) {
			c.mu.Lock()
			c.cfgMotor = cfg
			c.subiuMot++
			c.mu.Unlock()
			return c.motor, nil
		},
		SubirCtl: func(ctx context.Context, exe string, args []string, chave string, portaCtl int) (Processo, error) {
			c.mu.Lock()
			c.argsCtl, c.chaveCtl, c.portaCtl = args, chave, portaCtl
			c.mu.Unlock()
			return c.ctl, nil
		},
		AbrirNavegador: func(url string) error {
			c.ordem.anotar("abriu")
			c.abriu <- url
			return nil
		},
		Agora: time.Now,
	}
	return c
}

func (c *cenario) motorSubiu() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.subiuMot
}

// rodar sobe a sessão numa goroutine e devolve um canal que fecha quando o
// Rodar volta. O Cleanup garante que nada fica rodando depois do teste.
func rodar(t *testing.T, s *Sessao) <-chan struct{} {
	t.Helper()
	fim := make(chan struct{})
	go func() {
		s.Rodar(context.Background())
		close(fim)
	}()
	t.Cleanup(func() {
		s.Dormir()
		select {
		case <-fim:
		case <-time.After(5 * time.Second):
			t.Error("o Rodar não voltou")
		}
	})
	return fim
}

const urlTeste = "http://127.0.0.1:8080/?v=0.14.0#k=" + chaveTeste

func TestSessaoFeliz(t *testing.T) {
	c := novoCenario()
	esp := novoEspiao(c.ordem)
	s := Nova(c.deps, "0.14.0", esp.aoMudar)
	rodar(t, s)
	pronta := esp.esperar(t, "Pronta", fase(Pronta))

	if pronta.URL != urlTeste || pronta.Pct != 100 {
		t.Fatalf("Pronta = %+v", pronta)
	}
	if got := s.Atual(); got.Fase != Pronta || got.URL != urlTeste {
		t.Fatalf("Atual = %+v", got)
	}
	// Lendo com pct crescente até 90, depois Subindo, depois Pronta.
	ultimoPct, vistoSubindo := -1, false
	for _, e := range esp.todos() {
		if e.Fase != Pronta && e.URL != "" {
			t.Errorf("URL fora de Pronta: %+v", e)
		}
		switch e.Fase {
		case Lendo:
			if vistoSubindo || e.Pct < ultimoPct || e.Pct > 90 {
				t.Errorf("Lendo fora de ordem: %+v", esp.todos())
			}
			ultimoPct = e.Pct
		case Subindo:
			vistoSubindo = true
			if e.Pct < 90 || e.Pct > 99 {
				t.Errorf("Subindo com pct %d", e.Pct)
			}
		}
	}
	if ultimoPct != 90 || !vistoSubindo {
		t.Fatalf("faltou Lendo 90 ou Subindo: %+v", esp.todos())
	}
	if url := <-c.abriu; url != urlTeste {
		t.Fatalf("navegador abriu %q", url)
	}
	select {
	case u := <-c.abriu:
		t.Fatalf("navegador abriu de novo: %q", u)
	default:
	}
	esperado := motor.Config{Exe: "/cache/llamafile", Modelo: "/pen/models/m.gguf", Web: filepath.Join("/pen", "web"),
		LogDir: "/logs", Porta: 8080, Ctx: "8192", GOOS: "darwin"}
	if c.cfgMotor != esperado {
		t.Fatalf("config do motor = %+v", c.cfgMotor)
	}
	argsCtl := []string{"--root", "/pen", "--cache", "/cache", "--harness-port", "8080", "--parent-pid", "4242"}
	if !reflect.DeepEqual(c.argsCtl, argsCtl) || c.chaveCtl != chaveTeste || c.portaCtl != 11080 {
		t.Fatalf("ctl: args %q chave %q porta %d", c.argsCtl, c.chaveCtl, c.portaCtl)
	}
}

func TestSessaoPortaOcupadaPula(t *testing.T) {
	c := novoCenario()
	c.deps.PortaLivre = func(p int) bool { return p != 9080 } // 8080+1000 ocupada
	esp := novoEspiao(c.ordem)
	rodar(t, Nova(c.deps, "0.14.0", esp.aoMudar))
	if p := esp.esperar(t, "Pronta", fase(Pronta)); !strings.HasPrefix(p.URL, "http://127.0.0.1:8081/") {
		t.Fatalf("URL = %q", p.URL)
	}
}

func TestSessaoEsperaVaiDe90a99(t *testing.T) {
	tq := Tique
	Tique = time.Millisecond
	t.Cleanup(func() { Tique = tq })
	c := novoCenario()
	// O relógio anda 10 s a cada consulta: a barra chega ao teto rápido.
	var mu sync.Mutex
	agora := time.Unix(0, 0)
	c.deps.Agora = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		agora = agora.Add(10 * time.Second)
		return agora
	}
	var s *Sessao
	c.deps.SubirMotor = func(ctx context.Context, cfg motor.Config, aviso func(string)) (Processo, error) {
		for i := 0; i < 5000 && s.Atual().Pct < 99; i++ {
			time.Sleep(time.Millisecond)
		}
		return c.motor, nil
	}
	esp := novoEspiao(c.ordem)
	s = Nova(c.deps, "0.14.0", esp.aoMudar)
	rodar(t, s)
	esp.esperar(t, "Pronta", fase(Pronta))
	ultimo, vistos := 0, 0
	for _, e := range esp.todos() {
		if e.Fase != Subindo {
			continue
		}
		vistos++
		if e.Pct < ultimo || e.Pct < 90 || e.Pct > 99 {
			t.Fatalf("Subindo fora da faixa: %+v", esp.todos())
		}
		ultimo = e.Pct
	}
	if ultimo != 99 || vistos < 3 {
		t.Fatalf("a espera não andou até 99: %+v", esp.todos())
	}
}

func TestSessaoAvisoGPU(t *testing.T) {
	c := novoCenario()
	c.deps.SubirMotor = func(ctx context.Context, cfg motor.Config, aviso func(string)) (Processo, error) {
		aviso(motor.AvisoGPU)
		return c.motor, nil
	}
	esp := novoEspiao(c.ordem)
	rodar(t, Nova(c.deps, "0.14.0", esp.aoMudar))
	esp.esperar(t, "o aviso da GPU", func(e Estado) bool {
		return e.Aviso == "Aceleração por GPU indisponível, tentando só com CPU..."
	})
	esp.esperar(t, "Pronta", fase(Pronta))
}

func TestSessaoSemModelo(t *testing.T) {
	c := novoCenario()
	c.deps.AcharModelo = func() (string, error) { return "", modelo.ErrSemModelo }
	esp := novoEspiao(c.ordem)
	fim := rodar(t, Nova(c.deps, "0.14.0", esp.aoMudar))
	esp.esperar(t, "Erro", fase(Erro))
	<-fim
	esperado := Estado{Fase: Erro, Erro: "O modelo de IA não está neste pen drive. Grave o pen drive de novo."}
	if got := esp.ultimo(); got != esperado {
		t.Fatalf("último estado = %+v", got)
	}
	if c.motorSubiu() != 0 {
		t.Fatal("SubirMotor chamado sem modelo")
	}
}

func TestSessaoSemMotor(t *testing.T) {
	c := novoCenario()
	c.deps.PrepararCache = func() (string, string, error) { return "", "", ErrSemMotor }
	esp := novoEspiao(c.ordem)
	rodar(t, Nova(c.deps, "0.14.0", esp.aoMudar))
	e := esp.esperar(t, "Erro", fase(Erro))
	if e.Erro != "O motor da IA não está neste pen drive. Grave o pen drive de novo." {
		t.Fatalf("Erro = %q", e.Erro)
	}
	if c.motorSubiu() != 0 {
		t.Fatal("SubirMotor chamado sem motor")
	}
}

func TestSessaoMotorNaoSobe(t *testing.T) {
	c := novoCenario()
	c.deps.SubirMotor = func(context.Context, motor.Config, func(string)) (Processo, error) {
		return nil, motor.ErrNaoSubiu
	}
	esp := novoEspiao(c.ordem)
	fim := rodar(t, Nova(c.deps, "0.14.0", esp.aoMudar))
	e := esp.esperar(t, "Erro", fase(Erro))
	<-fim
	if e.Erro != "A IA não conseguiu acordar." {
		t.Fatalf("Erro = %q", e.Erro)
	}
	if len(c.abriu) != 0 {
		t.Fatal("abriu o navegador sem motor")
	}
}

func TestSessaoCtlFalhaSegue(t *testing.T) {
	c := novoCenario()
	c.deps.SubirCtl = func(context.Context, string, []string, string, int) (Processo, error) {
		return nil, errors.New("o controle do banco não subiu")
	}
	esp := novoEspiao(c.ordem)
	rodar(t, Nova(c.deps, "0.14.0", esp.aoMudar))
	p := esp.esperar(t, "Pronta", fase(Pronta))
	if p.Aviso != "O controle do banco não subiu. Seguindo sem banco de dados (Demonstração e Archeo continuam)." {
		t.Fatalf("Aviso = %q", p.Aviso)
	}
}

func TestSessaoSemCtlNoPenDrive(t *testing.T) {
	c := novoCenario()
	c.deps.PrepararCache = func() (string, string, error) { return "/cache/llamafile", "", nil }
	chamou := false
	c.deps.SubirCtl = func(context.Context, string, []string, string, int) (Processo, error) {
		chamou = true
		return c.ctl, nil
	}
	esp := novoEspiao(c.ordem)
	rodar(t, Nova(c.deps, "0.14.0", esp.aoMudar))
	p := esp.esperar(t, "Pronta", fase(Pronta))
	if chamou || !strings.HasPrefix(p.Aviso, "O controle do banco não subiu.") {
		t.Fatalf("chamou = %v, Aviso = %q", chamou, p.Aviso)
	}
}

func TestSessaoContinuaSemJanela(t *testing.T) {
	for nome, aoMudar := range map[string]func(Estado){"ignora": func(Estado) {}, "nil": nil} {
		t.Run(nome, func(t *testing.T) {
			c := novoCenario()
			s := Nova(c.deps, "0.14.0", aoMudar)
			rodar(t, s)
			select {
			case url := <-c.abriu:
				if url != urlTeste {
					t.Fatalf("abriu %q", url)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("o navegador não abriu")
			}
			for i := 0; i < 500 && s.Atual().Fase != Pronta; i++ {
				time.Sleep(10 * time.Millisecond)
			}
			if got := s.Atual(); got.Fase != Pronta {
				t.Fatalf("Atual = %+v", got)
			}
		})
	}
}

func TestDormirDuranteCarregamento(t *testing.T) {
	t.Run("lendo", func(t *testing.T) {
		c := novoCenario()
		entrou, saiu := make(chan struct{}), make(chan error, 1)
		c.deps.Ler = func(ctx context.Context, _ string, prog func(int64, int64)) error {
			prog(10, 100)
			close(entrou)
			<-ctx.Done()
			saiu <- ctx.Err()
			return ctx.Err()
		}
		esp := novoEspiao(c.ordem)
		s := Nova(c.deps, "0.14.0", esp.aoMudar)
		fim := rodar(t, s)
		<-entrou
		s.Dormir()
		if err := <-saiu; !errors.Is(err, context.Canceled) {
			t.Fatalf("Ler recebeu %v", err)
		}
		esp.esperar(t, "Dormiu", fase(Dormiu))
		<-fim
		if c.motorSubiu() != 0 || len(c.abriu) != 0 {
			t.Fatal("seguiu carregando depois do Dormir")
		}
		assertFimDormindo(t, esp)
	})
	t.Run("subindo o ctl", func(t *testing.T) {
		c := novoCenario()
		entrou := make(chan struct{})
		c.deps.SubirCtl = func(ctx context.Context, _ string, _ []string, _ string, _ int) (Processo, error) {
			close(entrou)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		esp := novoEspiao(c.ordem)
		s := Nova(c.deps, "0.14.0", esp.aoMudar)
		fim := rodar(t, s)
		<-entrou
		s.Dormir()
		esp.esperar(t, "Dormiu", fase(Dormiu))
		<-fim
		if !c.motor.parado() {
			t.Fatal("o motor ficou rodando")
		}
		if len(c.abriu) != 0 {
			t.Fatal("abriu o navegador depois do Dormir")
		}
		assertFimDormindo(t, esp)
		ordem := c.ordem.lista()
		if i, j := indice(ordem, "parou motor"), indice(ordem, "fase "+string(Dormiu)); i < 0 || j < i {
			t.Fatalf("ordem = %q", ordem)
		}
	})
}

func assertFimDormindo(t *testing.T, esp *espiao) {
	t.Helper()
	todos := esp.todos()
	n := len(todos)
	if n < 2 || todos[n-2].Fase != Dormindo || todos[n-1].Fase != Dormiu {
		t.Fatalf("as últimas fases não são Dormindo, Dormiu: %+v", todos)
	}
}

func indice(l []string, s string) int {
	for i, v := range l {
		if v == s {
			return i
		}
	}
	return -1
}

func TestDormirDepoisDePronta(t *testing.T) {
	c := novoCenario()
	esp := novoEspiao(c.ordem)
	s := Nova(c.deps, "0.14.0", esp.aoMudar)
	fim := rodar(t, s)
	esp.esperar(t, "Pronta", fase(Pronta))
	s.Dormir()
	<-fim
	esp.esperar(t, "Dormiu", fase(Dormiu))
	ordem := c.ordem.lista()
	ctl, mot, dormiu := indice(ordem, "parou ctl"), indice(ordem, "parou motor"), indice(ordem, "fase "+string(Dormiu))
	if ctl < 0 || mot < ctl || dormiu < mot {
		t.Fatalf("ordem = %q", ordem)
	}
	assertFimDormindo(t, esp)
	antes := len(esp.todos())
	s.Dormir()
	time.Sleep(20 * time.Millisecond)
	if depois := len(esp.todos()); depois != antes {
		t.Fatalf("a 2a chamada emitiu estados: %+v", esp.todos())
	}
	if got := s.Atual(); got.Fase != Dormiu {
		t.Fatalf("Atual = %+v", got)
	}
}

// Se algum processo não parou, o Dormiu não pode dizer "pode tirar o pen
// drive": carrega o aviso de reiniciar. O outro processo é parado assim mesmo.
func TestDormirComFalhaAoParar(t *testing.T) {
	for _, quem := range []string{"motor", "ctl"} {
		t.Run(quem, func(t *testing.T) {
			c := novoCenario()
			if quem == "motor" {
				c.motor.falha = errors.New("não saiu")
			} else {
				c.ctl.falha = errors.New("não saiu")
			}
			esp := novoEspiao(c.ordem)
			s := Nova(c.deps, "0.14.0", esp.aoMudar)
			fim := rodar(t, s)
			esp.esperar(t, "Pronta", fase(Pronta))
			s.Dormir()
			<-fim
			esp.esperar(t, "Dormiu", fase(Dormiu))
			if got := s.Atual(); got != (Estado{Fase: Dormiu, Erro: TextoNaoDesligou}) {
				t.Fatalf("Atual = %+v", got)
			}
			ordem := c.ordem.lista()
			if indice(ordem, "parou ctl") < 0 || indice(ordem, "parou motor") < 0 {
				t.Fatalf("os dois tinham de ser parados: %q", ordem)
			}
		})
	}
}

func TestDormirSemFalhaNaoAvisa(t *testing.T) {
	c := novoCenario()
	esp := novoEspiao(c.ordem)
	s := Nova(c.deps, "0.14.0", esp.aoMudar)
	fim := rodar(t, s)
	esp.esperar(t, "Pronta", fase(Pronta))
	s.Dormir()
	<-fim
	esp.esperar(t, "Dormiu", fase(Dormiu))
	if got := s.Atual(); got != (Estado{Fase: Dormiu}) {
		t.Fatalf("Atual = %+v", got)
	}
}

func TestDormirAntesDeRodar(t *testing.T) {
	c := novoCenario()
	esp := novoEspiao(c.ordem)
	s := Nova(c.deps, "0.14.0", esp.aoMudar)
	s.Dormir()
	esp.esperar(t, "Dormiu", fase(Dormiu))
	feito := make(chan struct{})
	go func() { s.Rodar(context.Background()); close(feito) }()
	select {
	case <-feito:
	case <-time.After(5 * time.Second):
		t.Fatal("o Rodar depois do Dormir travou")
	}
	if c.motorSubiu() != 0 {
		t.Fatal("subiu o motor depois do Dormir")
	}
}

func TestDormirDepoisDeErro(t *testing.T) {
	c := novoCenario()
	c.deps.AcharModelo = func() (string, error) { return "", modelo.ErrSemModelo }
	esp := novoEspiao(c.ordem)
	s := Nova(c.deps, "0.14.0", esp.aoMudar)
	fim := rodar(t, s)
	esp.esperar(t, "Erro", fase(Erro))
	<-fim
	s.Dormir()
	esp.esperar(t, "Dormiu", fase(Dormiu))
	assertFimDormindo(t, esp)
}

func TestMotorMorreDepoisDePronta(t *testing.T) {
	c := novoCenario()
	esp := novoEspiao(c.ordem)
	s := Nova(c.deps, "0.14.0", esp.aoMudar)
	fim := rodar(t, s)
	esp.esperar(t, "Pronta", fase(Pronta))
	c.motor.morrer()
	esp.esperar(t, "Erro", fase(Erro))
	<-fim
	if got := esp.ultimo(); got != (Estado{Fase: Erro, Erro: "A IA parou de responder."}) {
		t.Fatalf("último estado = %+v", got)
	}
	if !c.ctl.parado() {
		t.Fatal("o cd2-ctl ficou rodando")
	}
	if err := s.AbrirConversa(); err == nil {
		t.Fatal("AbrirConversa depois do Erro devia falhar")
	}
}

func TestSairDoRodarPoeParaDormir(t *testing.T) {
	c := novoCenario()
	esp := novoEspiao(c.ordem)
	s := Nova(c.deps, "0.14.0", esp.aoMudar)
	ctx, cancel := context.WithCancel(context.Background())
	fim := make(chan struct{})
	go func() { s.Rodar(ctx); close(fim) }()
	esp.esperar(t, "Pronta", fase(Pronta))
	cancel()
	<-fim
	esp.esperar(t, "Dormiu", fase(Dormiu))
	if !c.motor.parado() || !c.ctl.parado() {
		t.Fatal("ficou processo rodando")
	}
}

func TestAbrirConversaSoPronta(t *testing.T) {
	c := novoCenario()
	entrou, solta := make(chan struct{}), make(chan struct{})
	c.deps.Ler = func(ctx context.Context, _ string, prog func(int64, int64)) error {
		close(entrou)
		<-solta
		return nil
	}
	esp := novoEspiao(c.ordem)
	s := Nova(c.deps, "0.14.0", esp.aoMudar)
	rodar(t, s)
	<-entrou
	if err := s.AbrirConversa(); err == nil {
		t.Fatal("AbrirConversa em Lendo devia falhar")
	}
	close(solta)
	esp.esperar(t, "Pronta", fase(Pronta))
	<-c.abriu
	if err := s.AbrirConversa(); err != nil {
		t.Fatal(err)
	}
	if url := <-c.abriu; url != urlTeste {
		t.Fatalf("abriu %q", url)
	}
}

func TestCaminhosPorPlataforma(t *testing.T) {
	casos := []struct {
		raiz, goos, goarch, cache string
		esperado                  caminhosExe
	}{
		{`E:\`, "windows", "amd64", `C:\L\cd2-harness`, caminhosExe{
			MotorOrigem: `E:\bin\llamafile.exe`, Motor: `E:\bin\llamafile.exe`,
			CtlOrigem: `E:\bin\ctl\windows-amd64\cd2-ctl.exe`, Ctl: `E:\bin\ctl\windows-amd64\cd2-ctl.exe`}},
		{"/Volumes/CD2", "darwin", "arm64", "/Users/g/Library/Caches/cd2-harness", caminhosExe{
			MotorOrigem: "/Volumes/CD2/bin/llamafile", Motor: "/Users/g/Library/Caches/cd2-harness/llamafile",
			CtlOrigem: "/Volumes/CD2/bin/ctl/darwin-arm64/cd2-ctl", Ctl: "/Users/g/Library/Caches/cd2-harness/cd2-ctl"}},
	}
	for _, c := range casos {
		if got := caminhos(c.raiz, c.goos, c.goarch, c.cache); got != c.esperado {
			t.Errorf("caminhos(%q, %q) = %+v", c.raiz, c.goos, got)
		}
	}
}

func TestAmbiente(t *testing.T) {
	casos := []struct {
		env   map[string]string
		porta int
		ctx   string
	}{
		{map[string]string{}, 8080, "8192"},
		{map[string]string{"CD2_PORT": "9090", "CD2_CTX": "4096"}, 9090, "4096"},
		{map[string]string{"CD2_PORT": "abc"}, 8080, "8192"},
		{map[string]string{"CD2_PORT": " 8100 ", "CD2_CTX": " "}, 8100, "8192"},
	}
	for _, c := range casos {
		porta, ctx := ambiente(func(k string) string { return c.env[k] })
		if porta != c.porta || ctx != c.ctx {
			t.Errorf("ambiente(%v) = %d, %q", c.env, porta, ctx)
		}
	}
}
