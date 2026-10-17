package icone

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
)

func decodificar(t *testing.T, b []byte) (int, int, func(x, y int) color.NRGBA) {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("PNG inválido: %v", err)
	}
	r := img.Bounds()
	return r.Dx(), r.Dy(), func(x, y int) color.NRGBA {
		return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	}
}

func hex(r, g, b uint8) color.NRGBA { return color.NRGBA{r, g, b, 255} }

var (
	cinza   = hex(0xa1, 0xa6, 0xae)
	amarelo = hex(0xdb, 0xff, 0x2a)
	vazio   = hex(0x1b, 0x20, 0x50)
)

func TestCerebroTamanhoETransparencia(t *testing.T) {
	l, a, px := decodificar(t, Cerebro("acordada", 2))
	if l != 20 || a != 16 {
		t.Fatalf("tamanho %dx%d, esperado 20x16", l, a)
	}
	if px(0, 0).A != 0 {
		t.Errorf("canto (0,0) devia ser transparente, veio %v", px(0, 0))
	}
	if px(4, 0) != cinza {
		t.Errorf("contorno (4,0) = %v, esperado %v", px(4, 0), cinza)
	}
	// Escala < 1 vira 1.
	if l, a, _ := decodificar(t, Cerebro("acordada", 0)); l != 10 || a != 8 {
		t.Errorf("escala 0 deu %dx%d, esperado 10x8", l, a)
	}
}

func TestCerebroCarregandoMeioVazio(t *testing.T) {
	_, _, px := decodificar(t, Cerebro("carregando", 1))
	if px(4, 1) != vazio {
		t.Errorf("linha 1 = %v, esperado vazio %v", px(4, 1), vazio)
	}
	if px(4, 6) != amarelo {
		t.Errorf("linha 6 = %v, esperado cheio %v", px(4, 6), amarelo)
	}
	// Rosto sono: olhos k em (2,3); acordada tem branco em (2,2).
	if px(2, 3) != hex(0x0d, 0x12, 0x33) {
		t.Errorf("olho do sono = %v", px(2, 3))
	}
	_, _, ac := decodificar(t, Cerebro("acordada", 1))
	if ac(4, 1) != amarelo || ac(2, 2) != hex(255, 255, 255) {
		t.Errorf("acordada: cheio=%v olho=%v", ac(4, 1), ac(2, 2))
	}
}

func grade(t *testing.T, texto, secao string) []string {
	t.Helper()
	var linhas []string
	dentro := false
	for _, l := range strings.Split(texto, "\n") {
		l = strings.TrimRight(l, "\r")
		if strings.HasPrefix(l, "[") {
			dentro = l == "["+secao+"]"
			continue
		}
		if dentro && l != "" && !strings.HasPrefix(l, "#") {
			linhas = append(linhas, l)
		}
	}
	return linhas
}

func TestPixelsIguaisAoRoboTxt(t *testing.T) {
	b, err := os.ReadFile("../../../core/robo.txt")
	if err != nil {
		t.Fatal(err)
	}
	for nome, no := range map[string][]string{"cerebro": cerebroGrade, "sono": sonoGrade, "acordado": acordadoGrade} {
		esperado := grade(t, string(b), nome)
		if strings.Join(esperado, "\n") != strings.Join(no, "\n") || len(no) == 0 {
			t.Errorf("[%s] diverge do robo.txt:\n%v\n%v", nome, no, esperado)
		}
	}
}

func TestICO(t *testing.T) {
	p := Cerebro("acordada", 2)
	ico := ICO(p)
	if !bytes.HasPrefix(ico, []byte{0, 0, 1, 0, 1, 0}) {
		t.Fatalf("cabeçalho: % x", ico[:6])
	}
	if ico[6] != 20 || ico[7] != 16 {
		t.Errorf("largura/altura %d x %d", ico[6], ico[7])
	}
	if n := binary.LittleEndian.Uint32(ico[14:]); int(n) != len(p) {
		t.Errorf("tamanho no diretório %d, PNG %d", n, len(p))
	}
	off := binary.LittleEndian.Uint32(ico[18:])
	if !bytes.Equal(ico[off:], p) {
		t.Error("PNG não está embutido inteiro")
	}
	g := ICO(Cerebro("acordada", 32)) // 320x256
	if g[6] != 0 || g[7] != 0 {
		t.Errorf(">=256 deve gravar 0, veio %d x %d", g[6], g[7])
	}
}
