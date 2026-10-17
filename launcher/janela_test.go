package main

import (
	"bytes"
	"encoding/json"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/sessao"
)

func TestRotuloBandeja(t *testing.T) {
	casos := []struct {
		e    sessao.Estado
		quer string
	}{
		{sessao.Estado{Fase: sessao.Lendo, Pct: 62}, "IA de Bolso · carregando 62%"},
		{sessao.Estado{Fase: sessao.Subindo, Pct: 95}, "IA de Bolso · carregando 95%"},
		{sessao.Estado{Fase: sessao.Pronta, Pct: 100}, "IA de Bolso · acordada"},
		{sessao.Estado{Fase: sessao.Dormindo}, "IA de Bolso"},
		{sessao.Estado{Fase: sessao.Erro, Erro: "x"}, "IA de Bolso · com problema"},
	}
	for _, c := range casos {
		if got := rotuloBandeja(c.e); got != c.quer {
			t.Errorf("rotuloBandeja(%v) = %q, quer %q", c.e, got, c.quer)
		}
	}
}

func TestItensBandeja(t *testing.T) {
	textos := []string{"Mostrar a IA de Bolso", "Abrir uma nova janela da conversa", "Colocar a IA para dormir"}
	casos := []struct {
		fase   sessao.Fase
		ligado []bool
	}{
		{sessao.Lendo, []bool{true, false, true}},
		{sessao.Subindo, []bool{true, false, true}},
		{sessao.Pronta, []bool{true, true, true}},
		{sessao.Erro, []bool{true, false, true}},
		// Já indo dormir: não há o que pedir de novo.
		{sessao.Dormindo, []bool{true, false, false}},
		{sessao.Dormiu, []bool{true, false, false}},
	}
	for _, c := range casos {
		itens := itensBandeja(sessao.Estado{Fase: c.fase})
		if len(itens) != len(textos) {
			t.Fatalf("%s: %d itens, quer %d", c.fase, len(itens), len(textos))
		}
		for i, it := range itens {
			if it.Texto != textos[i] {
				t.Errorf("%s: item %d = %q, quer %q", c.fase, i, it.Texto, textos[i])
			}
			if it.Ligado != c.ligado[i] {
				t.Errorf("%s: %q ligado = %v, quer %v", c.fase, it.Texto, it.Ligado, c.ligado[i])
			}
		}
	}
}

// A tela (frontend/dist/tela.js) compara a fase com "Lendo", "Pronta"...
// e lê as chaves em minúsculas.
func TestParaTela(t *testing.T) {
	casos := map[sessao.Fase]string{
		sessao.Lendo: "Lendo", sessao.Subindo: "Subindo", sessao.Pronta: "Pronta",
		sessao.Dormindo: "Dormindo", sessao.Dormiu: "Dormiu", sessao.Erro: "Erro",
	}
	for f, quer := range casos {
		if got := paraTela(sessao.Estado{Fase: f}).Fase; got != quer {
			t.Errorf("fase %q virou %q, quer %q", f, got, quer)
		}
	}
	b, err := json.Marshal(paraTela(sessao.Estado{Fase: sessao.Subindo, Pct: 91, Aviso: "a", Erro: "e", URL: "http://x/#k=segredo"}))
	if err != nil {
		t.Fatal(err)
	}
	if got, quer := string(b), `{"fase":"Subindo","pct":91,"aviso":"a","erro":"e"}`; got != quer {
		t.Errorf("json = %s, quer %s", got, quer)
	}
}

// Erro nunca fica escondido: a janela volta sozinha com o erro.
func TestDeveMostrar(t *testing.T) {
	casos := []struct {
		e    sessao.Estado
		quer bool
	}{
		{sessao.Estado{Fase: sessao.Lendo}, false},
		{sessao.Estado{Fase: sessao.Subindo}, false},
		{sessao.Estado{Fase: sessao.Pronta}, false},
		{sessao.Estado{Fase: sessao.Dormindo}, false},
		{sessao.Estado{Fase: sessao.Dormiu}, false},
		{sessao.Estado{Fase: sessao.Erro, Erro: "x"}, true},
	}
	for _, c := range casos {
		if got := deveMostrar(c.e); got != c.quer {
			t.Errorf("deveMostrar(%v) = %v, quer %v", c.e, got, c.quer)
		}
	}
}

// Com o erro na tela, fechar a janela é o botão Fechar (a IA dorme e o app
// sai); nas outras fases fechar só esconde.
func TestFecharVaiDormir(t *testing.T) {
	for _, f := range []sessao.Fase{sessao.Lendo, sessao.Subindo, sessao.Pronta, sessao.Dormindo, sessao.Dormiu} {
		if fecharVaiDormir(f) {
			t.Errorf("%s: fechar deveria só esconder", f)
		}
	}
	if !fecharVaiDormir(sessao.Erro) {
		t.Error("Erro: fechar deveria pôr a IA para dormir")
	}
}

func TestIconeDoEstado(t *testing.T) {
	for f, quer := range map[sessao.Fase]string{
		sessao.Lendo: "carregando", sessao.Subindo: "carregando", sessao.Pronta: "acordada",
		sessao.Erro: "carregando", sessao.Dormindo: "carregando", sessao.Dormiu: "carregando",
	} {
		if got := iconeDoEstado(sessao.Estado{Fase: f}); got != quer {
			t.Errorf("%s: ícone %q, quer %q", f, got, quer)
		}
	}
}

func TestDeveChamar(t *testing.T) {
	for _, f := range []sessao.Fase{sessao.Lendo, sessao.Subindo, sessao.Dormindo, sessao.Dormiu, sessao.Erro} {
		if deveChamar(f) {
			t.Errorf("%s não deveria emitir chamada", f)
		}
	}
	if !deveChamar(sessao.Pronta) {
		t.Error("Pronta deveria emitir chamada")
	}
}

func TestLerVersao(t *testing.T) {
	dir := t.TempDir()
	if got := lerVersao("1.2.3", dir); got != "1.2.3" {
		t.Errorf("com ldflags: %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("0.14.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := lerVersao("", dir); got != "0.14.0" {
		t.Errorf("do VERSION: %q", got)
	}
	if got := lerVersao("", t.TempDir()); got != "dev" {
		t.Errorf("sem VERSION: %q", got)
	}
}

// O NSStatusItem do fyne.io/systray força 16x16 pontos: o ícone tem de ser
// quadrado, senão o cérebro sai achatado.
func TestIconeBandejaQuadrado(t *testing.T) {
	for _, estado := range []string{"carregando", "acordada"} {
		cfg, err := png.DecodeConfig(bytes.NewReader(iconePNG(estado, ladoBandeja)))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Width != ladoBandeja || cfg.Height != ladoBandeja {
			t.Errorf("%s: %dx%d, quer %dx%d", estado, cfg.Width, cfg.Height, ladoBandeja, ladoBandeja)
		}
	}
}

// build/appicon.png é o cérebro acordado em 1024 px, de onde o wails build
// tira o ícone do .app e do .exe. Para gerar de novo:
// CD2_GERAR_APPICON=1 go test -run TestAppicon .
func TestAppicon(t *testing.T) {
	quer := iconePNG("acordada", 1024)
	caminho := filepath.Join("build", "appicon.png")
	if os.Getenv("CD2_GERAR_APPICON") == "1" {
		if err := os.WriteFile(caminho, quer, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	// Compara os pixels, não os bytes: o codificador PNG muda entre versões do Go.
	a, errA := png.Decode(bytes.NewReader(got))
	b, errB := png.Decode(bytes.NewReader(quer))
	if errA != nil || errB != nil {
		t.Fatal(errA, errB)
	}
	if a.Bounds() != b.Bounds() {
		t.Fatalf("%s tem %v, quer %v", caminho, a.Bounds(), b.Bounds())
	}
	for y := 0; y < b.Bounds().Dy(); y++ {
		for x := 0; x < b.Bounds().Dx(); x++ {
			if color.NRGBAModel.Convert(a.At(x, y)) != color.NRGBAModel.Convert(b.At(x, y)) {
				t.Fatalf("%s diferente do cérebro acordado em (%d,%d); gere de novo com CD2_GERAR_APPICON=1", caminho, x, y)
			}
		}
	}
}
