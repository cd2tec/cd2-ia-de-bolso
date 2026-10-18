package main

import (
	"errors"
	"testing"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/chave"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/origem"
	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/sessao"
)

// Só tranca a cópia instalada sem chave: o pen drive e a dúvida sobem livres.
func TestDeveTrancar(t *testing.T) {
	casos := []struct {
		o      origem.Origem
		valida bool
		quer   bool
	}{
		{origem.Pendrive, false, false},
		{origem.Pendrive, true, false},
		{origem.Instalada, false, true},
		{origem.Instalada, true, false},
		{origem.Desconhecida, false, false},
		{origem.Desconhecida, true, false},
	}
	for _, c := range casos {
		leu := false
		valida := func() bool { leu = true; return c.valida }
		if got := deveTrancar(c.o, valida); got != c.quer {
			t.Errorf("deveTrancar(%s, %v) = %v, quer %v", c.o, c.valida, got, c.quer)
		}
		// Fora da instalada a chave nem é lida.
		if leu != (c.o == origem.Instalada) {
			t.Errorf("deveTrancar(%s): leu a chave = %v", c.o, leu)
		}
	}
}

// Trancada, a bandeja só mostra a janela ou fecha: não há IA para dormir.
func TestItensBandejaTrancada(t *testing.T) {
	quer := []item{{"Mostrar a IA de Bolso", true}, {"Fechar", true}}
	got := itensBandejaTrancada()
	if len(got) != len(quer) {
		t.Fatalf("%d itens, quer %d", len(got), len(quer))
	}
	for i := range quer {
		if got[i] != quer[i] {
			t.Errorf("item %d = %v, quer %v", i, got[i], quer[i])
		}
	}
}

// appTrancada monta um App trancado com o código do computador, a conferência
// e a gravação falsos. Devolve onde ficam a chave gravada e se a IA subiu.
func appTrancada(t *testing.T, confere bool, errGravar error) (*App, *string, *bool) {
	t.Helper()
	gravada, subiu := "", false
	velhos := []func(){}
	lc, cf, gv, pc := lerCodigo, conferirChave, gravarChave, pastaCache
	velhos = append(velhos, func() { lerCodigo, conferirChave, gravarChave, pastaCache = lc, cf, gv, pc })
	t.Cleanup(func() {
		for _, v := range velhos {
			v()
		}
	})
	lerCodigo = func() (string, error) { return "7F3K-Q9WD-2MXA", nil }
	conferirChave = func(k, cod string) bool { return confere && cod == "7F3K-Q9WD-2MXA" }
	gravarChave = func(cache, k string) error {
		if errGravar != nil {
			return errGravar
		}
		if cache != "/cache" {
			t.Errorf("gravou em %q, quer /cache", cache)
		}
		gravada = k
		return nil
	}
	pastaCache = func() string { return "/cache" }
	a := &App{s: sessao.Nova(sessao.Deps{}, "t", nil), trancada: true}
	a.acordar = func() { subiu = true }
	return a, &gravada, &subiu
}

// Uma chave qualquer com o formato certo (a assinatura de zeros não confere).
func chaveDeFormato() string { return chave.Formatar(make([]byte, 64)) }

func TestColarChaveIncompleta(t *testing.T) {
	a, gravada, subiu := appTrancada(t, true, nil)
	if got := a.ColarChave("CD2-ABC"); got != textoChaveIncompleta {
		t.Errorf("ColarChave = %q, quer %q", got, textoChaveIncompleta)
	}
	if *gravada != "" || *subiu || !a.estaTrancada() {
		t.Errorf("chave incompleta não pode gravar (%q), subir (%v) nem destrancar", *gravada, *subiu)
	}
}

func TestColarChaveDeOutroComputador(t *testing.T) {
	a, gravada, subiu := appTrancada(t, false, nil)
	if got := a.ColarChave(chaveDeFormato()); got != "Esta chave não é deste computador." {
		t.Errorf("ColarChave = %q", got)
	}
	if *gravada != "" || *subiu || !a.estaTrancada() {
		t.Errorf("chave de outro computador não pode gravar (%q), subir (%v) nem destrancar", *gravada, *subiu)
	}
}

func TestColarChaveAceita(t *testing.T) {
	a, gravada, subiu := appTrancada(t, true, nil)
	k := chaveDeFormato()
	if got := a.ColarChave("  " + k + "\n"); got != "" {
		t.Fatalf("ColarChave = %q, quer aceitar", got)
	}
	norm, _ := chave.Normalizar(k)
	if *gravada != norm {
		t.Errorf("gravou %q, quer %q", *gravada, norm)
	}
	if !*subiu || a.estaTrancada() {
		t.Errorf("aceita: subiu = %v, trancada = %v", *subiu, a.estaTrancada())
	}
	if got := a.Estado().Fase; got == faseTrancada {
		t.Errorf("depois de aceitar, Estado ainda é %q", got)
	}
}

func TestColarChaveSemGravar(t *testing.T) {
	a, _, subiu := appTrancada(t, true, errors.New("disco cheio"))
	if got := a.ColarChave(chaveDeFormato()); got != textoNaoGravou {
		t.Errorf("ColarChave = %q, quer %q", got, textoNaoGravou)
	}
	if *subiu || !a.estaTrancada() {
		t.Error("sem gravar a chave a IA não sobe")
	}
}

func TestColarChaveSemCodigo(t *testing.T) {
	a, _, subiu := appTrancada(t, true, nil)
	lerCodigo = func() (string, error) { return "", errors.New("sem ioreg") }
	if got := a.ColarChave(chaveDeFormato()); got != textoSemCodigo {
		t.Errorf("ColarChave = %q, quer %q", got, textoSemCodigo)
	}
	if *subiu {
		t.Error("sem o código a IA não sobe")
	}
}

// Trancada, a tela pede o estado e recebe a fase Trancada.
func TestEstadoTrancada(t *testing.T) {
	a, _, _ := appTrancada(t, true, nil)
	if got := a.Estado(); got != (estadoTela{Fase: "Trancada"}) {
		t.Errorf("Estado = %+v, quer a fase Trancada", got)
	}
}

// Fechar a janela trancada sai: não há IA acordada para ficar no ícone.
func TestFecharSai(t *testing.T) {
	a, _, _ := appTrancada(t, true, nil)
	if !a.fecharSai() {
		t.Error("trancada: fechar deveria sair")
	}
	a.trancada = false
	if a.fecharSai() {
		t.Error("destrancada, em Lendo: fechar deveria só esconder")
	}
}

// O Dormir antes de o Rodar começar (o app trancado que sai) não trava e
// chega ao Dormiu.
func TestDormirSemRodar(t *testing.T) {
	fases := make(chan sessao.Fase, 4)
	s := sessao.Nova(sessao.Deps{}, "t", func(e sessao.Estado) { fases <- e.Fase })
	s.Dormir()
	if got := s.Atual().Fase; got != sessao.Dormiu {
		t.Errorf("depois do Dormir: %s, quer Dormiu", got)
	}
	if f := <-fases; f != sessao.Dormindo {
		t.Errorf("primeiro estado %s, quer Dormindo", f)
	}
	if f := <-fases; f != sessao.Dormiu {
		t.Errorf("segundo estado %s, quer Dormiu", f)
	}
}
