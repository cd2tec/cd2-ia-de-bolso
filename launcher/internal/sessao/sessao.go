// Package sessao é o maestro do lançador: lê o modelo, sobe o motor e o
// cd2-ctl, abre a conversa, vigia o motor e põe a IA para dormir. Segue a
// ordem de core/start-unix.sh e core/start-windows.ps1.
package sessao

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/cache"
	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/chave"
	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/ctl"
	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/modelo"
	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/motor"
	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/navegador"
	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/portas"
	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/proc"
	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/sobra"
)

// Tique é o intervalo em que a barra anda enquanto o motor sobe. Variável
// para o teste encurtar.
var Tique = 500 * time.Millisecond

// Espera é o tempo em que a barra vai de 90 a 99 esperando o motor. Não é
// prazo: o motor tem o dele; a barra só para em 99 até ele responder.
var Espera = 60 * time.Second

// Processo é o que a sessão usa de um filho; *proc.Processo serve.
type Processo interface {
	Pid() int
	Morreu() <-chan struct{}
	Parar() error
}

// Deps reúne a configuração e os efeitos da sessão, para o teste trocar por
// falsos. DepsReais liga os pacotes de verdade.
type Deps struct {
	Raiz         string
	GOOS         string
	PID          int    // do lançador, para o cd2-ctl sair junto
	Cache, Logs  string // pastas
	PortaInicial int    // CD2_PORT
	Ctx          string // CD2_CTX

	AcharModelo func() (string, error)
	// PrepararCache devolve os executáveis do motor e do cd2-ctl. No Mac
	// copia para o cache; ctl vazio quer dizer que não há cd2-ctl.
	PrepararCache  func() (motorExe, ctlExe string, err error)
	PortaLivre     func(int) bool
	NovaChave      func() (string, error)
	Ler            func(ctx context.Context, caminho string, prog func(feitos, total int64)) error
	SubirMotor     func(ctx context.Context, c motor.Config, aviso func(string)) (Processo, error)
	SubirCtl       func(ctx context.Context, exe string, args []string, chave string, portaCtl int) (Processo, error)
	AbrirNavegador func(url string) error
	Agora          func() time.Time
	// SemNavegador é o --inicio: a IA acorda com o computador e fica só no
	// ícone, sem abrir a conversa. A URL vale do mesmo jeito em Pronta.
	SemNavegador bool
}

// Sessao é uma IA acordada (ou acordando) neste computador.
type Sessao struct {
	d       Deps
	versao  string
	aoMudar func(Estado)

	mu         sync.Mutex
	estado     Estado
	fila       []Estado
	sinal      chan struct{}
	rodou      bool
	dormindo   bool
	cancelar   context.CancelFunc
	carregando chan struct{} // fecha quando o Rodar sai do carregamento
	motor, ctl Processo

	dormir sync.Once
}

// Nova prepara a sessão. aoMudar recebe cada estado em ordem, numa goroutine
// própria e fora do lock: pode chamar Atual, AbrirConversa e Dormir. Pode ser
// nil (janelinha fechada).
func Nova(d Deps, versao string, aoMudar func(Estado)) *Sessao {
	if aoMudar == nil {
		aoMudar = func(Estado) {}
	}
	s := &Sessao{d: d, versao: versao, aoMudar: aoMudar, estado: Estado{Fase: Lendo}, sinal: make(chan struct{}, 1)}
	go s.entregar()
	return s
}

// Atual devolve o último estado.
func (s *Sessao) Atual() Estado {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.estado
}

// publicar troca o estado e o põe na fila de entrega. Exige s.mu.
func (s *Sessao) publicar(e Estado) {
	s.estado = e
	s.fila = append(s.fila, e)
	select {
	case s.sinal <- struct{}{}:
	default:
	}
}

// entregar chama o aoMudar na ordem da fila. Uma goroutine só garante a
// ordem sem segurar o lock; termina depois de entregar Dormiu.
func (s *Sessao) entregar() {
	for range s.sinal {
		s.mu.Lock()
		fila := s.fila
		s.fila = nil
		s.mu.Unlock()
		for _, e := range fila {
			s.aoMudar(e)
			if e.Fase == Dormiu {
				return
			}
		}
	}
}

// mudar aplica f ao estado e publica, a não ser que a IA já esteja indo
// dormir: dali em diante só o Dormir anuncia.
func (s *Sessao) mudar(f func(*Estado)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dormindo {
		return
	}
	e := s.estado
	f(&e)
	if e != s.estado {
		s.publicar(e)
	}
}

func (s *Sessao) falhar(texto string) {
	s.mudar(func(e *Estado) { *e = Estado{Fase: Erro, Erro: texto} })
}

// Rodar carrega a IA e depois vigia o motor até ele parar ou a IA dormir.
// Se o ctx acabar, a IA dorme.
func (s *Sessao) Rodar(ctx context.Context) {
	s.mu.Lock()
	if s.dormindo || s.rodou {
		s.mu.Unlock()
		return
	}
	s.rodou = true
	ctx, cancelar := context.WithCancel(ctx)
	defer cancelar()
	s.cancelar = cancelar
	carregando := make(chan struct{})
	s.carregando = carregando
	s.publicar(Estado{Fase: Lendo})
	s.mu.Unlock()

	m, ok := s.carregar(ctx)
	close(carregando)
	if ok {
		s.vigiar(ctx, m)
	}
	// Cancelado sem Dormir: quem chamou desistiu (o app está fechando).
	if ctx.Err() != nil {
		s.Dormir()
	}
}

// carregar faz a sequência dos scripts até Pronta. Devolve o motor e se
// chegou lá; em falha já anunciou o Erro (menos quando foi o ctx).
func (s *Sessao) carregar(ctx context.Context) (Processo, bool) {
	d := s.d
	caminho, err := d.AcharModelo()
	if err != nil {
		s.falhar(TextoSemModelo)
		return nil, false
	}
	motorExe, ctlExe, err := d.PrepararCache()
	if err != nil {
		if errors.Is(err, ErrSemMotor) {
			s.falhar(TextoSemMotor)
		} else {
			s.falhar(TextoNaoAcordou)
		}
		return nil, false
	}
	porta, err := portas.Achar(d.PortaInicial, d.PortaLivre)
	if err != nil {
		s.falhar(TextoNaoAcordou)
		return nil, false
	}
	k, err := d.NovaChave()
	if err != nil {
		s.falhar(TextoNaoAcordou)
		return nil, false
	}

	// Como o robo_ler: se a leitura falhar, o motor sobe assim mesmo (só
	// fica sem o cache de arquivos aquecido).
	_ = d.Ler(ctx, caminho, func(feitos, total int64) {
		pct := pctFimLeitura
		if total > 0 {
			pct = int(feitos * pctFimLeitura / total)
		}
		pct = limitar(pct, 0, pctFimLeitura)
		s.mudar(func(e *Estado) {
			if e.Fase == Lendo && pct > e.Pct {
				e.Pct = pct
			}
		})
	})
	if ctx.Err() != nil {
		return nil, false
	}

	s.mudar(func(e *Estado) { *e = Estado{Fase: Subindo, Pct: pctFimLeitura} })
	cfg := motor.Config{Exe: motorExe, Modelo: caminho, Web: filepath.Join(d.Raiz, "web"), LogDir: d.Logs,
		Porta: porta, Ctx: d.Ctx, GOOS: d.GOOS}
	parar := s.andarEspera()
	m, err := d.SubirMotor(ctx, cfg, func(txt string) { s.mudar(func(e *Estado) { e.Aviso = txt }) })
	parar()
	if err != nil {
		if errors.Is(err, sobra.ErrAindaFechando) {
			s.falhar(TextoAindaFechando)
		} else if ctx.Err() == nil {
			s.falhar(TextoNaoAcordou)
		}
		return nil, false
	}
	s.mu.Lock()
	s.motor = m
	s.mu.Unlock()
	if ctx.Err() != nil {
		return nil, false
	}

	// O aviso da GPU já cumpriu o papel: o motor está de pé.
	aviso := ""
	if ctlExe == "" {
		aviso = TextoSemBanco
	} else {
		args := ctl.Args(d.Raiz, d.Cache, porta, d.PID)
		c, err := d.SubirCtl(ctx, ctlExe, args, k, porta+3000)
		if ctx.Err() != nil {
			if err == nil {
				s.guardarCtl(c)
			}
			return nil, false
		}
		if err != nil {
			aviso = TextoSemBanco
		} else {
			s.guardarCtl(c)
		}
	}

	url := navegador.URL(porta, s.versao, k)
	// Sem navegador a IA segue acordada: o botão de abrir outra janela tenta de novo.
	if !d.SemNavegador {
		_ = d.AbrirNavegador(url)
	}
	if ctx.Err() != nil {
		return nil, false
	}
	s.mudar(func(e *Estado) { *e = Estado{Fase: Pronta, Pct: pctPronta, Aviso: aviso, URL: url} })
	return m, true
}

func (s *Sessao) guardarCtl(c Processo) {
	s.mu.Lock()
	s.ctl = c
	s.mu.Unlock()
}

// andarEspera faz a barra ir de 90 a 99 enquanto o motor sobe. A função
// devolvida para o andamento e só volta depois que ele parou.
func (s *Sessao) andarEspera() func() {
	inicio := s.d.Agora()
	parar, parou := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(parou)
		t := time.NewTicker(Tique)
		defer t.Stop()
		for {
			select {
			case <-parar:
				return
			case <-t.C:
			}
			passou := s.d.Agora().Sub(inicio)
			pct := pctFimLeitura + int(int64(pctTetoEspera-pctFimLeitura)*int64(passou)/int64(Espera))
			pct = limitar(pct, pctFimLeitura, pctTetoEspera)
			s.mudar(func(e *Estado) {
				if e.Fase == Subindo && pct > e.Pct {
					e.Pct = pct
				}
			})
		}
	}()
	return func() {
		close(parar)
		<-parou
	}
}

// vigiar espera o motor morrer ou a IA dormir. Motor que morre sozinho
// derruba o cd2-ctl e vira Erro.
func (s *Sessao) vigiar(ctx context.Context, m Processo) {
	select {
	case <-ctx.Done():
		return
	case <-m.Morreu():
	}
	s.mu.Lock()
	dormindo, c := s.dormindo, s.ctl
	s.mu.Unlock()
	if dormindo {
		return
	}
	if c != nil {
		_ = c.Parar()
	}
	s.falhar(TextoParou)
}

// AbrirConversa abre outra janela com a mesma URL. Só vale em Pronta.
func (s *Sessao) AbrirConversa() error {
	e := s.Atual()
	if e.Fase != Pronta {
		return ErrNaoPronta
	}
	return s.d.AbrirNavegador(e.URL)
}

// AbrirInstalar abre a conversa já com o modal de instalação. Só vale em
// Pronta, como o AbrirConversa.
func (s *Sessao) AbrirInstalar() error {
	e := s.Atual()
	if e.Fase != Pronta {
		return ErrNaoPronta
	}
	return s.d.AbrirNavegador(navegador.ComInstalar(e.URL))
}

// Dormir cancela o carregamento, para o cd2-ctl e depois o motor, e anuncia
// Dormindo e, com tudo parado, Dormiu. Se algum não parou, o Dormiu leva
// TextoNaoDesligou no Erro. Pode ser chamado mais de uma vez e antes do
// Rodar; volta só quando tudo parou (ou desistiu de parar).
func (s *Sessao) Dormir() {
	s.dormir.Do(func() {
		s.mu.Lock()
		s.dormindo = true
		if s.cancelar != nil {
			s.cancelar()
		}
		carregando := s.carregando
		s.publicar(Estado{Fase: Dormindo})
		s.mu.Unlock()

		// O carregamento cancelado ainda pode estar guardando um processo
		// recém-subido: espera ele sair para não deixar nada para trás.
		if carregando != nil {
			<-carregando
		}
		s.mu.Lock()
		m, c := s.motor, s.ctl
		s.mu.Unlock()
		// O cd2-ctl antes: ele derruba o toolbox e não fica falando com um motor morto.
		// Um que não pare não impede de tentar o outro.
		falhou := false
		for _, p := range []Processo{c, m} {
			if p != nil && p.Parar() != nil {
				falhou = true
			}
		}
		fim := Estado{Fase: Dormiu}
		if falhou {
			// Algo pode estar segurando o pen drive: nada de "pode tirar".
			fim.Erro = TextoNaoDesligou
		}
		s.mu.Lock()
		s.publicar(fim)
		s.mu.Unlock()
	})
}

func limitar(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// caminhosExe são os executáveis no pen drive (origem) e de onde rodam.
type caminhosExe struct {
	MotorOrigem, Motor string
	CtlOrigem, Ctl     string
}

// caminhos escolhe os executáveis por plataforma, com o separador do goos
// informado. No Mac o motor e o cd2-ctl rodam de uma cópia no cache (pen
// drive exFAT não guarda permissão de execução); no Windows, do pen drive.
func caminhos(raiz, goos, goarch, pastaCache string) caminhosExe {
	ctlOrigem := ctl.Caminho(raiz, goos, goarch)
	if goos == "windows" {
		m := strings.TrimRight(raiz, `\/`) + `\bin\llamafile.exe`
		return caminhosExe{MotorOrigem: m, Motor: m, CtlOrigem: ctlOrigem, Ctl: ctlOrigem}
	}
	return caminhosExe{
		MotorOrigem: strings.TrimRight(raiz, "/") + "/bin/llamafile",
		Motor:       pastaCache + "/llamafile",
		CtlOrigem:   ctlOrigem,
		Ctl:         pastaCache + "/cd2-ctl",
	}
}

// ambiente lê CD2_PORT (padrão 8080) e CD2_CTX (padrão "8192").
func ambiente(env func(string) string) (porta int, ctx string) {
	porta = 8080
	if p, err := strconv.Atoi(strings.TrimSpace(env("CD2_PORT"))); err == nil && p > 0 {
		porta = p
	}
	ctx = strings.TrimSpace(env("CD2_CTX"))
	if ctx == "" {
		ctx = "8192"
	}
	return porta, ctx
}

// DepsReais liga a sessão aos pacotes de verdade, com a raiz do pen drive.
func DepsReais(raiz string) Deps {
	goos := runtime.GOOS
	home, _ := os.UserHomeDir()
	pastaCache := cache.Pasta(goos, os.Getenv, home)
	logs := cache.PastaLogs(goos, os.Getenv, home)
	porta, ctx := ambiente(os.Getenv)
	exes := caminhos(raiz, goos, runtime.GOARCH, pastaCache)

	// Prazo curto por pedido: um motor travado não pode segurar a conferência.
	cli := &http.Client{Timeout: 2 * time.Second}
	get := func(url string) (int, error) {
		r, err := cli.Get(url)
		if err != nil {
			return 0, err
		}
		_, _ = io.Copy(io.Discard, r.Body)
		r.Body.Close()
		return r.StatusCode, nil
	}
	existe := func(p string) bool { st, err := os.Stat(p); return err == nil && !st.IsDir() }

	return Deps{
		Raiz: raiz, GOOS: goos, PID: os.Getpid(), Cache: pastaCache, Logs: logs,
		PortaInicial: porta, Ctx: ctx,
		AcharModelo: func() (string, error) { return modelo.Achar(raiz) },
		PrepararCache: func() (string, string, error) {
			if !existe(exes.MotorOrigem) {
				return "", "", ErrSemMotor
			}
			ctlExe := exes.Ctl
			if !existe(exes.CtlOrigem) {
				ctlExe = ""
			}
			if goos != "darwin" {
				return exes.Motor, ctlExe, nil
			}
			if _, err := cache.Preparar(exes.MotorOrigem, exes.Motor); err != nil {
				return "", "", err
			}
			if ctlExe != "" {
				// Sem cópia do cd2-ctl a IA segue sem banco, como no script.
				if _, err := cache.Preparar(exes.CtlOrigem, exes.Ctl); err != nil {
					ctlExe = ""
				}
			}
			return exes.Motor, ctlExe, nil
		},
		PortaLivre: portas.Livre,
		NovaChave:  chave.Nova,
		Ler: func(ctx context.Context, caminho string, prog func(feitos, total int64)) error {
			return modelo.Ler(ctx, caminho, modelo.Bloco, prog)
		},
		SubirMotor: func(ctx context.Context, c motor.Config, aviso func(string)) (Processo, error) {
			// Um llamafile deste pen drive que sobrou da abertura anterior
			// segura o log no Windows; encerra só ele antes de subir.
			logs := []string{filepath.Join(c.LogDir, "server.log"), filepath.Join(c.LogDir, "server.err.log")}
			if err := sobra.Encerrar(c.Exe, logs); err != nil {
				return nil, err
			}
			return processo(motor.Subir(ctx, c, get, aviso))
		},
		SubirCtl: func(ctx context.Context, exe string, args []string, k string, portaCtl int) (Processo, error) {
			return processo(ctl.Subir(ctx, exe, args, k, portaCtl, logs, goos, get))
		},
		AbrirNavegador: navegador.Abrir,
		Agora:          time.Now,
	}
}

// processo evita que um *proc.Processo nil vire uma interface não nil.
func processo(p *proc.Processo, err error) (Processo, error) {
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New("o processo não foi criado")
	}
	return p, nil
}
