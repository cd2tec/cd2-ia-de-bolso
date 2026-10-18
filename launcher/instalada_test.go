package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/origem"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/sistema"
	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/navegador"
	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/sessao"
)

func TestLerArgs(t *testing.T) {
	casos := []struct {
		args []string
		quer Args
	}{
		{nil, Args{}},
		{[]string{"--inicio"}, Args{Inicio: true}},
		{[]string{"--assumir"}, Args{Assumir: true}},
		{[]string{"--desinstalar"}, Args{Desinstalar: true}},
		// O Mac pode mandar -psn_...; o desconhecido não atrapalha.
		{[]string{"-psn_0_12345", "--inicio", "qualquer"}, Args{Inicio: true}},
		{[]string{"--esperar", "4242"}, Args{Esperar: 4242}},
		{[]string{"--esperar"}, Args{}},
		{[]string{"--esperar", "x", "--assumir"}, Args{Assumir: true}},
		{[]string{"--esperar", "-3"}, Args{}},
	}
	for _, c := range casos {
		if got := lerArgs(c.args); got != c.quer {
			t.Errorf("lerArgs(%q) = %+v, quer %+v", c.args, got, c.quer)
		}
	}
}

func TestURLInstalarAntesDoHash(t *testing.T) {
	got := navegador.ComInstalar(navegador.URL(8080, "0.15.0", "abc"))
	if quer := "http://127.0.0.1:8080/?v=0.15.0&instalar=1#k=abc"; got != quer {
		t.Errorf("ComInstalar = %q, quer %q", got, quer)
	}
}

// O item "Instalar neste computador" só existe no pen drive e com a IA
// acordada, entre a nova janela e o dormir.
func TestItensBandejaPendrive(t *testing.T) {
	for _, f := range []sessao.Fase{sessao.Lendo, sessao.Subindo, sessao.Pronta, sessao.Dormindo, sessao.Dormiu, sessao.Erro} {
		for _, pendrive := range []bool{false, true} {
			itens := itensBandeja(sessao.Estado{Fase: f}, pendrive)
			textos := []string{}
			for _, it := range itens {
				textos = append(textos, it.Texto)
			}
			quer := []string{itemMostrar, itemNova, itemDormir}
			if pendrive && f == sessao.Pronta {
				quer = []string{itemMostrar, itemNova, itemInstalar, itemDormir}
			}
			if !reflect.DeepEqual(textos, quer) {
				t.Errorf("%s pendrive=%v: %q, quer %q", f, pendrive, textos, quer)
			}
			for _, it := range itens {
				if it.Texto == itemInstalar && !it.Ligado {
					t.Errorf("%s: o item de instalar veio desligado", f)
				}
			}
		}
	}
}

// A tela recebe "pendrive" para decidir o convite; a URL continua de fora.
func TestParaTelaPendrive(t *testing.T) {
	b, err := json.Marshal(paraTela(sessao.Estado{Fase: sessao.Pronta, Pct: 100, URL: "http://x/#k=segredo"}, true, true))
	if err != nil {
		t.Fatal(err)
	}
	if got, quer := string(b), `{"fase":"Pronta","pct":100,"aviso":"","erro":"","pendrive":true,"semNavegador":true}`; got != quer {
		t.Errorf("json = %s, quer %s", got, quer)
	}
}

func TestAppPendrive(t *testing.T) {
	for _, c := range []struct {
		o    origem.Origem
		quer bool
	}{{origem.Pendrive, true}, {origem.Instalada, false}, {origem.Desconhecida, false}, {"", false}} {
		a := &App{origem: c.o}
		if got := a.pendrive(); got != c.quer {
			t.Errorf("origem %q: pendrive = %v, quer %v", c.o, got, c.quer)
		}
	}
}

// trocaFalsa troca o que a troca usa na máquina de verdade.
func trocaFalsa(t *testing.T, existe bool) chan []string {
	t.Helper()
	ce, cl, ci := caminhoInstalada, existeArquivo, iniciarDestacado
	t.Cleanup(func() { caminhoInstalada, existeArquivo, iniciarDestacado = ce, cl, ci })
	caminhoInstalada = func() string { return "/inst/IA de Bolso" }
	existeArquivo = func(string) bool { return existe }
	rodou := make(chan []string, 2)
	iniciarDestacado = func(exe string, args ...string) error {
		rodou <- append([]string{exe}, args...)
		return nil
	}
	return rodou
}

// No pen drive, o --assumir da instalada põe a IA para dormir e, só depois
// do Dormiu, roda a instalada (sem --assumir, esperando este processo sair).
func TestAssumirDormeEDepoisAbre(t *testing.T) {
	rodou := trocaFalsa(t, true)
	a := &App{origem: origem.Pendrive}
	fases := make(chan sessao.Fase, 4)
	a.s = sessao.Nova(sessao.Deps{}, "t", func(e sessao.Estado) {
		fases <- e.Fase
		a.aoMudar(e)
	})
	a.segundaCopia([]string{"--assumir"})
	var vistas []sessao.Fase
	select {
	case cmd := <-rodou:
		for len(fases) > 0 {
			vistas = append(vistas, <-fases)
		}
		if !reflect.DeepEqual(vistas, []sessao.Fase{sessao.Dormindo, sessao.Dormiu}) {
			t.Errorf("rodou a instalada depois de %v, quer depois do Dormiu", vistas)
		}
		quer := []string{"/inst/IA de Bolso", "--esperar", strconv.Itoa(pidAtual())}
		if !reflect.DeepEqual(cmd, quer) {
			t.Errorf("rodou %q, quer %q", cmd, quer)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("não rodou a instalada")
	}
	// A saída vem logo depois de rodar a instalada, sem os 3 s do "Boa noite".
	for prazo := time.Now().Add(time.Second); ; time.Sleep(5 * time.Millisecond) {
		a.mu.Lock()
		pode := a.podeSair
		a.mu.Unlock()
		if pode {
			break
		}
		if time.Now().After(prazo) {
			t.Fatal("depois da troca o app devia sair na hora")
		}
	}
}

// A instalada que recebe --assumir (ou um pen drive sem a instalada no disco)
// só se mostra: nada de dormir.
func TestAssumirSemTroca(t *testing.T) {
	for _, c := range []struct {
		nome   string
		o      origem.Origem
		existe bool
	}{{"instalada", origem.Instalada, true}, {"sem a instalada", origem.Pendrive, false}} {
		rodou := trocaFalsa(t, c.existe)
		a := &App{origem: c.o, s: sessao.Nova(sessao.Deps{}, "t", nil)}
		a.segundaCopia([]string{"--assumir"})
		if f := a.s.Atual().Fase; f != sessao.Lendo {
			t.Errorf("%s: fase %s, quer que nada dormisse", c.nome, f)
		}
		if len(rodou) != 0 {
			t.Errorf("%s: rodou %q", c.nome, <-rodou)
		}
	}
}

// Uma segunda cópia comum só traz a janela.
func TestSegundaCopiaComumNaoTroca(t *testing.T) {
	rodou := trocaFalsa(t, true)
	a := &App{origem: origem.Pendrive, s: sessao.Nova(sessao.Deps{}, "t", nil)}
	a.segundaCopia(nil)
	if f := a.s.Atual().Fase; f != sessao.Lendo || len(rodou) != 0 {
		t.Errorf("fase %s, rodou %d: a segunda cópia comum não troca", f, len(rodou))
	}
}

func TestEsperarProcesso(t *testing.T) {
	vivo := 3
	esperarProcesso(77, time.Second, time.Millisecond, func(pid int) bool {
		if pid != 77 {
			t.Errorf("perguntou do pid %d", pid)
		}
		vivo--
		return vivo > 0
	})
	if vivo != 0 {
		t.Errorf("parou de esperar cedo: %d", vivo)
	}
	// Um processo que não sai não trava o lançador para sempre.
	inicio := time.Now()
	esperarProcesso(77, 20*time.Millisecond, time.Millisecond, func(int) bool { return true })
	if d := time.Since(inicio); d > time.Second {
		t.Errorf("esperou %v", d)
	}
}

// Com --inicio a notificação do primeiro fechar não sai.
func TestInicioNaoAvisa(t *testing.T) {
	a := novaApp(t.TempDir(), "t", Args{Inicio: true})
	if !a.avisou {
		t.Error("com --inicio o aviso do primeiro fechar devia já contar como dado")
	}
	if !a.semNavegador || !a.Estado().SemNavegador {
		t.Error("com --inicio a tela devia saber que a conversa não abriu")
	}
	if b := novaApp(t.TempDir(), "t", Args{}); b.avisou || b.semNavegador {
		t.Error("sem --inicio o aviso continua valendo")
	}
}

// Se a instalada não abre, nada de "Boa noite! Pode tirar o pen drive.": o
// Dormiu vira o aviso, visível e pelo tempo do aviso de falha.
func TestTrocaQueFalhaAvisa(t *testing.T) {
	trocaFalsa(t, true)
	iniciarDestacado = func(string, ...string) error { return errors.New("sem permissão") }
	a := &App{origem: origem.Pendrive, trocarPara: "/inst/IA de Bolso"}
	a.s = sessao.Nova(sessao.Deps{}, "t", nil)
	if got := a.trocar(""); got != naoAbriu {
		t.Fatalf("trocar = %v, quer naoAbriu", got)
	}
	e := a.comDormiu(sessao.Estado{Fase: sessao.Dormiu})
	if e.Erro != "Não consegui abrir a IA instalada. Abra pelo ícone nos programas." {
		t.Errorf("Erro = %q", e.Erro)
	}
	if !deveMostrar(e) || esperaAntesDeSair(e) != esperaSairFalha {
		t.Error("o aviso devia trazer a janela e ficar o tempo da falha")
	}
	// A tela que pede o estado depois (recarregou) também vê o aviso.
	a.s.Dormir()
	if got := a.Estado(); got.Fase != "Dormiu" || got.Erro != textoNaoAbriuInstalada {
		t.Errorf("Estado = %+v", got)
	}
}

// Sem pedido de troca, o Dormiu continua o "Boa noite" de sempre.
func TestDormiuSemTrocaNaoMuda(t *testing.T) {
	a := &App{}
	if got := a.trocar(""); got != semTroca {
		t.Errorf("trocar = %v", got)
	}
	if e := a.comDormiu(sessao.Estado{Fase: sessao.Dormiu}); e.Erro != "" {
		t.Errorf("Erro = %q", e.Erro)
	}
}

// A instalada roda de dentro da própria pasta, não da pasta do pen drive.
func TestComandoSoltoForaDoPendrive(t *testing.T) {
	exe := filepath.Join("inst", "IA de Bolso", "IA de Bolso")
	c := comandoSolto(exe, "--esperar", "7")
	if c.Dir != filepath.Dir(exe) {
		t.Errorf("Dir = %q, quer %q", c.Dir, filepath.Dir(exe))
	}
	if c.SysProcAttr == nil {
		t.Error("o comando devia sair solto deste processo")
	}
	if !reflect.DeepEqual(c.Args[1:], []string{"--esperar", "7"}) {
		t.Errorf("Args = %q", c.Args)
	}
}

// --assumir que chega com a IA já dormida não faz nada.
func TestAssumirDepoisDoDormiu(t *testing.T) {
	rodou := trocaFalsa(t, true)
	a := &App{origem: origem.Pendrive, s: sessao.Nova(sessao.Deps{}, "t", nil)}
	a.s.Dormir()
	a.segundaCopia([]string{"--assumir"})
	if a.trocarPara != "" || len(rodou) != 0 {
		t.Error("depois do Dormiu o --assumir não devia pedir troca")
	}
}

// sistemaFalso é o sistema da máquina, só com o início com o computador.
type sistemaFalso struct {
	ligado bool
	falha  error
	pedido []string
}

func (s *sistemaFalso) Atalhos(string, bool) error { return nil }
func (s *sistemaFalso) LigarInicio(lancador string, ligar bool) error {
	s.pedido = append(s.pedido, lancador+" "+strconv.FormatBool(ligar))
	if s.falha != nil {
		return s.falha
	}
	s.ligado = ligar
	return nil
}
func (s *sistemaFalso) InicioLigado() bool                       { return s.ligado }
func (s *sistemaFalso) Registrar(string, string) error           { return nil }
func (s *sistemaFalso) Remover() error                           { return nil }
func (s *sistemaFalso) PlanoRemocao(string, int) sistema.Comando { return sistema.Comando{} }

func sistemaDeTeste(t *testing.T, s *sistemaFalso) {
	t.Helper()
	sa, ci := sistemaAqui, caminhoInstalada
	t.Cleanup(func() { sistemaAqui, caminhoInstalada = sa, ci })
	sistemaAqui = func() sistema.Sistema { return s }
	caminhoInstalada = func() string { return "/inst/IA de Bolso" }
}

// Na instalada o menu segue o rascunho (tela 4): mostrar, nova janela,
// acordar com o computador (marcado conforme o sistema), desinstalar e
// dormir. Nada de instalar.
func TestItensBandejaInstalada(t *testing.T) {
	for _, ligado := range []bool{false, true} {
		itens := itensBandejaInstalada(sessao.Estado{Fase: sessao.Pronta}, ligado)
		textos := []string{}
		for _, it := range itens {
			textos = append(textos, it.Texto)
			if !it.Ligado {
				t.Errorf("Pronta: %q veio desligado", it.Texto)
			}
			if it.Marcado != (it.Texto == itemInicio && ligado) {
				t.Errorf("ligado=%v: %q marcado = %v", ligado, it.Texto, it.Marcado)
			}
		}
		quer := []string{itemMostrar, itemNova, itemInicio, itemDesinstalar, itemDormir}
		if !reflect.DeepEqual(textos, quer) {
			t.Errorf("itens %q, quer %q", textos, quer)
		}
	}
	if itemInicio != "Acordar quando o computador ligar" || itemDesinstalar != "Desinstalar..." {
		t.Errorf("textos: %q, %q", itemInicio, itemDesinstalar)
	}
	// Indo dormir não dá mais para pedir a desinstalação.
	for _, it := range itensBandejaInstalada(sessao.Estado{Fase: sessao.Dormindo}, true) {
		if it.Texto == itemDesinstalar && it.Ligado {
			t.Error("Dormindo: desinstalar devia vir desligado")
		}
	}
	// O menu da App lê o sistema e só tem esses itens na instalada.
	s := &sistemaFalso{ligado: true}
	sistemaDeTeste(t, s)
	a := &App{origem: origem.Instalada}
	a.lerInicio()
	if !a.inicioLigado {
		t.Error("lerInicio não leu o sistema")
	}
	if got := a.itensDoMenu(sessao.Estado{Fase: sessao.Pronta}); len(got) != 5 || !got[2].Marcado {
		t.Errorf("itens da instalada = %+v", got)
	}
	p := &App{origem: origem.Pendrive}
	for _, it := range p.itensDoMenu(sessao.Estado{Fase: sessao.Pronta}) {
		if it.Texto == itemInicio || it.Texto == itemDesinstalar {
			t.Errorf("pen drive com %q", it.Texto)
		}
	}
}

// O item liga e desliga na hora; se o sistema recusar, a marcação fica
// como estava e a janela diz o porquê.
func TestAlternarInicio(t *testing.T) {
	s := &sistemaFalso{}
	sistemaDeTeste(t, s)
	a := &App{origem: origem.Instalada, s: sessao.Nova(sessao.Deps{}, "t", nil)}
	a.lerInicio()
	a.alternarInicio()
	if !a.inicioLigado || !reflect.DeepEqual(s.pedido, []string{"/inst/IA de Bolso true"}) {
		t.Fatalf("ligado=%v pedido=%q", a.inicioLigado, s.pedido)
	}
	s.falha = errors.New("negado")
	a.alternarInicio()
	if !a.inicioLigado {
		t.Error("a falha devia manter a marcação de antes")
	}
	if got := a.Estado().Aviso; got != textoNaoMudouInicio {
		t.Errorf("Aviso = %q, quer %q", got, textoNaoMudouInicio)
	}
	s.falha = nil
	a.alternarInicio()
	if a.inicioLigado || a.Estado().Aviso != "" {
		t.Errorf("desligar: ligado=%v aviso=%q", a.inicioLigado, a.Estado().Aviso)
	}
}

// remocaoFalsa troca o que a desinstalação faz na máquina.
func remocaoFalsa(t *testing.T, c sistema.Comando, err error, ordem chan string) chan sistema.Comando {
	t.Helper()
	da, dr, pc, ed := desinstalarAgora, dispararRemocao, pastaCache, esperaDesinstalada
	t.Cleanup(func() { desinstalarAgora, dispararRemocao, pastaCache, esperaDesinstalada = da, dr, pc, ed })
	esperaDesinstalada = 10 * time.Millisecond
	pastaCache = func() string { return "/cache" }
	desinstalarAgora = func(cache string, pid int) (sistema.Comando, error) {
		ordem <- "desinstalou " + cache + " " + strconv.Itoa(pid)
		return c, err
	}
	disparou := make(chan sistema.Comando, 1)
	dispararRemocao = func(c sistema.Comando) error {
		ordem <- "disparou"
		disparou <- c
		return nil
	}
	return disparou
}

func podeSairLogo(a *App, prazo time.Duration) bool {
	for fim := time.Now().Add(prazo); time.Now().Before(fim); time.Sleep(5 * time.Millisecond) {
		a.mu.Lock()
		pode := a.podeSair
		a.mu.Unlock()
		if pode {
			return true
		}
	}
	return false
}

// Desinstalar põe a IA para dormir, e só depois do Dormiu desinstala,
// dispara a remoção da pasta e sai.
func TestDesinstalarDormeAntes(t *testing.T) {
	ordem := make(chan string, 8)
	cmd := sistema.Comando{Exe: "/bin/sh", Args: []string{"-c", "x"}, Env: []string{"A=b"}}
	disparou := remocaoFalsa(t, cmd, nil, ordem)
	a := &App{origem: origem.Instalada}
	a.s = sessao.Nova(sessao.Deps{}, "t", func(e sessao.Estado) {
		ordem <- string(e.Fase)
		a.aoMudar(e)
	})
	a.PedirDesinstalar()
	if got := a.Estado().Fase; got != faseDesinstalar {
		t.Fatalf("PedirDesinstalar: fase %q", got)
	}
	a.Desinstalar()
	select {
	case c := <-disparou:
		if !reflect.DeepEqual(c, cmd) {
			t.Errorf("disparou %+v", c)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("não disparou a remoção")
	}
	var vistas []string
	for len(ordem) > 0 {
		vistas = append(vistas, <-ordem)
	}
	quer := []string{string(sessao.Dormindo), string(sessao.Dormiu), "desinstalou /cache " + strconv.Itoa(pidAtual()), "disparou"}
	if !reflect.DeepEqual(vistas, quer) {
		t.Errorf("ordem %q, quer %q", vistas, quer)
	}
	if got := a.Estado().Fase; got != faseDesinstalada {
		t.Errorf("depois de disparar: fase %q", got)
	}
	if !podeSairLogo(a, time.Second) {
		t.Error("depois de desinstalar o app devia sair")
	}
}

// Se a desinstalação recusa (erro ou comando vazio), nada é disparado, o
// erro fica na tela e o app não sai sozinho; o Fechar sai.
func TestDesinstalarRecusado(t *testing.T) {
	for _, c := range []struct {
		nome string
		cmd  sistema.Comando
		err  error
	}{{"erro", sistema.Comando{}, errors.New("não é a pasta")}, {"vazio", sistema.Comando{}, nil}} {
		ordem := make(chan string, 8)
		disparou := remocaoFalsa(t, c.cmd, c.err, ordem)
		a := &App{origem: origem.Instalada}
		dormiu := make(chan struct{})
		a.s = sessao.Nova(sessao.Deps{}, "t", func(e sessao.Estado) {
			a.aoMudar(e)
			if e.Fase == sessao.Dormiu {
				close(dormiu)
			}
		})
		a.Desinstalar()
		<-dormiu
		if len(disparou) != 0 {
			t.Errorf("%s: disparou a remoção", c.nome)
		}
		if got := a.Estado(); got.Fase != "Erro" || got.Erro != textoNaoDesinstalou {
			t.Errorf("%s: Estado = %+v", c.nome, got)
		}
		if podeSairLogo(a, 50*time.Millisecond) {
			t.Errorf("%s: o app saiu sozinho", c.nome)
		}
		if got := a.itensDoMenu(a.s.Atual()); !reflect.DeepEqual(got, itensBandejaTrancada()) {
			t.Errorf("%s: menu %+v", c.nome, got)
		}
		a.Fechar()
		if !podeSairLogo(a, time.Second) {
			t.Errorf("%s: o Fechar devia sair", c.nome)
		}
	}
}

// Com --desinstalar a IA não acorda: a tela pergunta primeiro. "Melhor
// não" acorda a IA como sempre.
func TestArgDesinstalarNaoAcorda(t *testing.T) {
	a := novaApp(t.TempDir(), "t", Args{Desinstalar: true})
	acordou := 0
	a.acordar = func() { acordou++ }
	if tela, ok := a.telaEspecial(); !ok || tela.Fase != faseDesinstalar {
		t.Fatalf("telaEspecial = %+v, %v", tela, ok)
	}
	if a.Estado().Fase != faseDesinstalar || acordou != 0 {
		t.Fatal("com --desinstalar devia abrir na pergunta, sem acordar")
	}
	a.MelhorNao()
	if acordou != 1 {
		t.Errorf("Melhor não acordou %d vezes", acordou)
	}
	if got := a.Estado().Fase; got != "Lendo" {
		t.Errorf("depois do Melhor não: fase %q", got)
	}
	// Trancada, o Melhor não volta para a trava e não acorda.
	b := novaApp(t.TempDir(), "t", Args{Desinstalar: true})
	b.trancada = true
	b.acordar = func() { t.Error("trancada não acorda") }
	b.MelhorNao()
	if got := b.Estado().Fase; got != faseTrancada {
		t.Errorf("trancada depois do Melhor não: %q", got)
	}
	if _, ok := novaApp(t.TempDir(), "t", Args{}).telaEspecial(); ok {
		t.Error("sem --desinstalar nem trava não há tela especial")
	}
}

// O --desinstalar de uma segunda cópia leva a que está rodando para a
// pergunta, sem dormir nada.
func TestSegundaCopiaDesinstalar(t *testing.T) {
	trocaFalsa(t, true)
	a := &App{origem: origem.Instalada, s: sessao.Nova(sessao.Deps{}, "t", nil)}
	a.segundaCopia([]string{"--desinstalar"})
	if got := a.Estado().Fase; got != faseDesinstalar {
		t.Errorf("fase %q", got)
	}
	if f := a.s.Atual().Fase; f != sessao.Lendo {
		t.Errorf("a sessão foi para %s", f)
	}
}

// A remoção roda fora da pasta da instalação, solta, com o ambiente de
// agora mais o do comando.
func TestComandoRemocao(t *testing.T) {
	c := comandoRemocao(sistema.Comando{Exe: "/bin/sh", Args: []string{"-c", "x"}, Env: []string{"CD2_PASTA=/inst"}})
	if c.Dir != os.TempDir() {
		t.Errorf("Dir = %q", c.Dir)
	}
	if c.SysProcAttr == nil {
		t.Error("a remoção devia sair solta deste processo")
	}
	if len(c.Env) != len(os.Environ())+1 || c.Env[len(c.Env)-1] != "CD2_PASTA=/inst" {
		t.Errorf("Env termina em %q", c.Env[len(c.Env)-1])
	}
	if !reflect.DeepEqual(c.Args, []string{"/bin/sh", "-c", "x"}) {
		t.Errorf("Args = %q", c.Args)
	}
}
