package discos

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFiltrarEscondeBootSistemaTimeMachine(t *testing.T) {
	entrada := []Bruto{
		{ID: "ok", Bytes: 32_000_000_000},
		{ID: "boot", Bytes: 32_000_000_000, Boot: true},
		{ID: "sistema", Bytes: 32_000_000_000, Sistema: true},
		{ID: "tm", Bytes: 32_000_000_000, TimeMachine: true},
	}
	saida := Filtrar(entrada)
	if len(saida) != 1 || saida[0].ID != "ok" {
		t.Fatalf("esperava só o disco ok, veio %+v", saida)
	}
}

func TestFiltrarLimite(t *testing.T) {
	saida := Filtrar([]Bruto{
		{ID: "no-limite", Bytes: 260_000_000_000},
		{ID: "acima", Bytes: 260_000_000_001},
	})
	if len(saida) != 1 || saida[0].ID != "no-limite" {
		t.Fatalf("esperava só no-limite, veio %+v", saida)
	}
}

func TestDescreverSituacao(t *testing.T) {
	casos := []struct {
		bytes int64
		quer  Situacao
	}{
		{3_999_999_999, Pequeno},
		{4_000_000_000, Aviso},
		{15_999_999_999, Aviso},
		{16_000_000_000, Ok},
	}
	for _, c := range casos {
		d := Descrever(Bruto{ID: "x", Bytes: c.bytes}, func([]string) (int, bool, bool) { return 0, false, false })
		if d.Situacao != c.quer {
			t.Errorf("%d bytes: situação %q, queria %q", c.bytes, d.Situacao, c.quer)
		}
	}
}

func TestDescreverCopiaCampos(t *testing.T) {
	d := Descrever(Bruto{ID: "7", Nome: "SanDisk", Bytes: 32_000_000_000, Montagens: []string{"E:\\"}},
		func(m []string) (int, bool, bool) { return 12, true, false })
	if d.ID != "7" || d.Nome != "SanDisk" || d.Bytes != 32_000_000_000 || d.Arquivos != 12 || !d.MaisDe {
		t.Fatalf("campos errados: %+v", d)
	}
}

func TestRotulo(t *testing.T) {
	d := Descrever(Bruto{Nome: "SanDisk Ultra", Bytes: 32_017_047_552}, func([]string) (int, bool, bool) { return 0, false, false })
	if d.Rotulo != "SanDisk Ultra 32 GB" {
		t.Errorf("rótulo %q", d.Rotulo)
	}
	d = Descrever(Bruto{Bytes: 32_017_047_552}, func([]string) (int, bool, bool) { return 0, false, false })
	if d.Rotulo != "Pen drive 32 GB" {
		t.Errorf("rótulo %q", d.Rotulo)
	}
}

func TestContarArquivosPara(t *testing.T) {
	vazia := t.TempDir()
	if n, mais, sem := ContarArquivos([]string{vazia}); n != 0 || mais || sem {
		t.Fatalf("pasta vazia: %d, %v", n, mais)
	}
	cheia := t.TempDir()
	for i := 0; i < 10_005; i++ {
		if err := os.WriteFile(filepath.Join(cheia, fmt.Sprintf("a%d", i)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if n, mais, _ := ContarArquivos([]string{cheia}); n != 10_000 || !mais {
		t.Fatalf("pasta cheia: %d, %v", n, mais)
	}
}

func TestContarArquivosSomaMontagensERecursivo(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(a, "sub", "fundo"), 0o755)
	os.WriteFile(filepath.Join(a, "1"), nil, 0o644)
	os.WriteFile(filepath.Join(a, "sub", "2"), nil, 0o644)
	os.WriteFile(filepath.Join(a, "sub", "fundo", "3"), nil, 0o644)
	os.WriteFile(filepath.Join(b, "4"), nil, 0o644)
	if n, mais, sem := ContarArquivos([]string{a, b, filepath.Join(a, "nao-existe")}); n != 4 || mais || sem {
		t.Fatalf("esperava 4, veio %d, %v", n, mais)
	}
}

func TestMesmo(t *testing.T) {
	a := Disco{ID: "1", Nome: "X", Bytes: 100}
	if !Mesmo(a, Disco{ID: "1", Nome: "X", Bytes: 100, Arquivos: 5}) {
		t.Error("deveria ser o mesmo")
	}
	if Mesmo(a, Disco{ID: "1", Nome: "X", Bytes: 101}) {
		t.Error("Bytes diferentes deveria ser false")
	}
	if Mesmo(a, Disco{ID: "1", Nome: "Y", Bytes: 100}) || Mesmo(a, Disco{ID: "2", Nome: "X", Bytes: 100}) {
		t.Error("Nome ou ID diferentes deveria ser false")
	}
}

// Sem permissão na raiz da montagem (o macOS nega o volume removível sem a
// autorização), a contagem não pode virar "está vazio".
func TestContarArquivosSemAcessoNaRaiz(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("chmod não tira a leitura no Windows nem para root")
	}
	fechada := t.TempDir()
	os.WriteFile(filepath.Join(fechada, "segredo"), nil, 0o644)
	if err := os.Chmod(fechada, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(fechada, 0o755) })
	aberta := t.TempDir()
	os.WriteFile(filepath.Join(aberta, "1"), nil, 0o644)
	n, mais, sem := ContarArquivos([]string{aberta, fechada})
	if !sem || n != 1 || mais {
		t.Fatalf("n=%d mais=%v semAcesso=%v", n, mais, sem)
	}
	// Subpasta ilegível continua só sendo pulada.
	raiz := t.TempDir()
	sub := filepath.Join(raiz, "sub")
	os.Mkdir(sub, 0o755)
	os.Chmod(sub, 0o000)
	t.Cleanup(func() { os.Chmod(sub, 0o755) })
	if _, _, sem := ContarArquivos([]string{raiz}); sem {
		t.Fatal("subpasta ilegível não é falta de acesso ao pen drive")
	}
	d := Descrever(Bruto{ID: "x", Bytes: 32_000_000_000, Montagens: []string{fechada}}, ContarArquivos)
	if !d.SemAcesso || d.Arquivos != 0 {
		t.Fatalf("Descrever: %+v", d)
	}
}

func TestNasMontagens(t *testing.T) {
	m := t.TempDir()
	exe := filepath.Join(m, "CD2 IA de Bolso.app", "Contents", "MacOS", "instalador")
	if !NasMontagens([]string{"/nao/existe", m}, exe) {
		t.Error("o instalador dentro da montagem deveria ser achado")
	}
	if NasMontagens([]string{m}, filepath.Join(filepath.Dir(m), "outra", "instalador")) {
		t.Error("fora da montagem não conta")
	}
	if NasMontagens([]string{m}, m+"-irma"+string(filepath.Separator)+"x") {
		t.Error("pasta irmã com o mesmo começo de nome não conta")
	}
	if NasMontagens(nil, exe) || NasMontagens([]string{m}, "") {
		t.Error("sem montagem ou sem caminho não conta")
	}
}
