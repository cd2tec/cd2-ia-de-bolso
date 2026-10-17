package discos

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"unicode/utf16"
)

// decodificar desfaz o -EncodedCommand: base64 de UTF-16LE.
func decodificar(t *testing.T, b64 string) string {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(b)%2 != 0 {
		t.Fatalf("base64 inválido: %v", err)
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	return string(utf16.Decode(u))
}

func TestWindowsUmDiscoObjeto(t *testing.T) {
	got, err := lerDiscosWindows(fixture(t, "windows_um_disco.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "2" || got[0].Nome != "SanDisk Ultra USB 3.0" || got[0].Bytes != 32015679488 {
		t.Fatalf("veio %+v", got)
	}
	// Com um disco só, DriveLetters pode vir como texto solto.
	if len(got[0].Montagens) != 1 || got[0].Montagens[0] != `E:\` {
		t.Fatalf("montagens %v", got[0].Montagens)
	}
}

func TestWindowsLista(t *testing.T) {
	got, err := lerDiscosWindows(fixture(t, "windows_lista.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("veio %+v", got)
	}
	if got[0].ID != "2" || got[0].Boot || len(got[0].Montagens) != 1 || got[0].Montagens[0] != `E:\` {
		t.Errorf("disco 2: %+v", got[0])
	}
	if got[1].ID != "3" || !got[1].Boot || len(got[1].Montagens) != 0 {
		t.Errorf("disco 3: %+v", got[1])
	}
	if f := Filtrar(got); len(f) != 1 || f[0].ID != "2" {
		t.Errorf("filtro: %+v", f)
	}
}

// Disco com Size ou Number nulo (leitor de cartão sem cartão, por exemplo) é
// pulado; os outros seguem.
func TestWindowsPulaDiscoSemTamanho(t *testing.T) {
	js := `[{"Number": 2, "FriendlyName": "SanDisk", "Size": 32015679488, "DriveLetters": ["E"]},
		{"Number": 3, "FriendlyName": "Leitor", "Size": null, "DriveLetters": []},
		{"Number": null, "FriendlyName": "Fantasma", "Size": 8000000000}]`
	got, err := lerDiscosWindows([]byte(js))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "2" {
		t.Fatalf("veio %+v", got)
	}
	um, err := lerDiscosWindows([]byte(`{"Number": 3, "FriendlyName": "Leitor", "Size": null}`))
	if err != nil || len(um) != 0 {
		t.Fatalf("objeto único sem Size: %+v %v", um, err)
	}
}

func TestWindowsVazio(t *testing.T) {
	for _, entrada := range []string{"", "  \r\n", "[]", "\ufeff[]"} {
		got, err := lerDiscosWindows([]byte(entrada))
		if err != nil || len(got) != 0 {
			t.Errorf("%q: %+v %v", entrada, got, err)
		}
	}
	if _, err := lerDiscosWindows([]byte("isto não é json")); err == nil {
		t.Error("esperava erro")
	}
}

func TestWindowsListarChamaPowerShell(t *testing.T) {
	var args []string
	x := func(_ context.Context, nome string, a ...string) ([]byte, error) {
		args = append([]string{nome}, a...)
		return fixture(t, "windows_lista.json"), nil
	}
	got, err := NovoWindows(x).Listar(context.Background())
	if err != nil || len(got) != 2 {
		t.Fatalf("%+v %v", got, err)
	}
	if !strings.Contains(decodificar(t, args[len(args)-1]), "Get-Disk") {
		t.Fatalf("o script de listar devia estar no -Command: %v", args)
	}
}

func TestWindowsApagarChamaPowerShell(t *testing.T) {
	var args []string
	x := func(_ context.Context, nome string, a ...string) ([]byte, error) {
		args = append([]string{nome}, a...)
		return []byte("E\r\n"), nil
	}
	mnt, err := NovoWindows(x).Apagar(context.Background(), "2", "CD2")
	if err != nil {
		t.Fatal(err)
	}
	if mnt != `E:\` {
		t.Fatalf("montagem %q", mnt)
	}
	if len(args) != 7 || args[0] != "powershell" || args[1] != "-NoProfile" || args[2] != "-NonInteractive" ||
		args[3] != "-ExecutionPolicy" || args[4] != "Bypass" || args[5] != "-EncodedCommand" {
		t.Fatalf("args: %q", args)
	}
	cmd := decodificar(t, args[6])
	// O script embutido vai inteiro, dentro de & { ... }, seguido dos parâmetros.
	if !strings.HasPrefix(cmd, "& { ") || !strings.Contains(cmd, strings.TrimSpace(apagarPS1)) ||
		!strings.HasSuffix(cmd, "} -DiskNumber 2 -Label 'CD2'") {
		t.Fatalf("comando inesperado:\n%s", cmd)
	}
}

func TestWindowsApagarRecusaIDERotuloInvalidos(t *testing.T) {
	chamou := false
	x := func(context.Context, string, ...string) ([]byte, error) { chamou = true; return []byte("E"), nil }
	for _, id := range []string{"2; calc", "", "-1", "abc"} {
		if _, err := NovoWindows(x).Apagar(context.Background(), id, "CD2"); err == nil || chamou {
			t.Fatalf("ID %q tem de ser recusado sem chamar o PowerShell", id)
		}
	}
	for _, r := range []string{"", "cd2", "O'NEIL", "NOME MUITO LONGO", "A;B"} {
		if _, err := NovoWindows(x).Apagar(context.Background(), "2", r); err == nil || chamou {
			t.Fatalf("rótulo %q tem de ser recusado sem chamar o PowerShell", r)
		}
	}
}

func TestWindowsApagarSoAceitaLetraExata(t *testing.T) {
	for _, saida := range []string{"E:", "EF", "1", "Formatando...\r\nE\r\n", "E\r\naviso", "\r\n"} {
		x := func(context.Context, string, ...string) ([]byte, error) { return []byte(saida), nil }
		mnt, err := NovoWindows(x).Apagar(context.Background(), "2", "CD2")
		if saida == "Formatando...\r\nE\r\n" {
			if err != nil || mnt != `E:\` {
				t.Errorf("%q: %q %v", saida, mnt, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("saída %q deveria dar erro, veio %q", saida, mnt)
		}
	}
}

func TestWindowsDiscoSemNumberOuSizeEhPulado(t *testing.T) {
	for _, js := range []string{
		`{"FriendlyName":"X","Size":32015679488,"IsBoot":false}`,
		`[{"Number":2,"FriendlyName":"X","IsBoot":false}]`,
	} {
		if got, err := lerDiscosWindows([]byte(js)); err != nil || len(got) != 0 {
			t.Errorf("%s: deveria ser pulado, veio %+v %v", js, got, err)
		}
	}
}

func TestWindowsApagarSemLetraDaErro(t *testing.T) {
	x := func(context.Context, string, ...string) ([]byte, error) { return []byte("\r\n"), nil }
	if _, err := NovoWindows(x).Apagar(context.Background(), "2", "CD2"); err == nil {
		t.Fatal("saída sem letra deve dar erro")
	}
	e := func(context.Context, string, ...string) ([]byte, error) { return nil, fmt.Errorf("falhou") }
	if _, err := NovoWindows(e).Apagar(context.Background(), "2", "CD2"); err == nil {
		t.Fatal("erro do executor deve subir")
	}
}

func TestWindowsEjetar(t *testing.T) {
	var cmd string
	x := func(_ context.Context, _ string, a ...string) ([]byte, error) {
		cmd = decodificar(t, a[len(a)-1])
		return nil, nil
	}
	if err := NovoWindows(x).Ejetar(context.Background(), "2", `E:\`); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cmd, "InvokeVerb('Eject')") || !strings.HasSuffix(cmd, "} -Letter 'E'") {
		t.Fatalf("-Command: %s", cmd)
	}
	if err := NovoWindows(x).Ejetar(context.Background(), "2", ""); err == nil {
		t.Fatal("montagem vazia deve dar erro")
	}
}
