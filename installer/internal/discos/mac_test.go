package discos

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixture(t *testing.T, nome string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", nome))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// falso é um Executor que registra as chamadas e responde pelo texto do comando.
type falso struct {
	respostas map[string][]byte
	erros     map[string]error
	seq       map[string][][]byte // respostas em sequência, uma por chamada
	chamadas  []string
}

func (f *falso) exec(_ context.Context, nome string, args ...string) ([]byte, error) {
	linha := strings.Join(append([]string{nome}, args...), " ")
	f.chamadas = append(f.chamadas, linha)
	if err, ok := f.erros[linha]; ok {
		return nil, err
	}
	if q := f.seq[linha]; len(q) > 0 {
		f.seq[linha] = q[1:]
		return q[0], nil
	}
	if r, ok := f.respostas[linha]; ok {
		return r, nil
	}
	return nil, fmt.Errorf("comando inesperado: %s", linha)
}

func macFalso(t *testing.T) *falso {
	return &falso{respostas: map[string][]byte{
		"diskutil list -plist external physical": fixture(t, "diskutil_list_externos.plist"),
		"diskutil info -plist /":                 fixture(t, "diskutil_info_raiz.plist"),
		"diskutil apfs list -plist":              fixture(t, "diskutil_apfs.plist"),
		"diskutil info -plist /dev/disk4":        fixture(t, "diskutil_info_disk4.plist"),
		"diskutil info -plist /dev/disk5":        fixture(t, "diskutil_info_disk5.plist"),
		"diskutil info -plist /dev/disk7":        fixture(t, "diskutil_info_disk7.plist"),
	}}
}

func TestMacListaWholeDisks(t *testing.T) {
	got, err := lerListaMac(fixture(t, "diskutil_list_externos.plist"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"disk4", "disk5", "disk7"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("veio %v, esperava %v", got, want)
	}
}

func TestMacPapeisAPFS(t *testing.T) {
	m, err := lerApfsMac(fixture(t, "diskutil_apfs.plist"))
	if err != nil {
		t.Fatal(err)
	}
	if !contem(m["disk5"], "Backup") {
		t.Fatalf("disk5 deveria ter papel Backup: %v", m)
	}
	if !contem(m["disk0"], "System") || !contem(m["disk0"], "Data") {
		t.Fatalf("disk0 deveria ter System e Data: %v", m)
	}
	if _, ok := m["disk4"]; ok {
		t.Fatalf("disk4 não é APFS: %v", m)
	}
}

func TestMacInfoEBoot(t *testing.T) {
	nome, bytes, montagem, err := lerInfoMac(fixture(t, "diskutil_info_disk4s1_depois.plist"))
	if err != nil || nome != "" || bytes != 32010928128 || montagem != "/Volumes/CD2" {
		t.Fatalf("info: %q %d %q %v", nome, bytes, montagem, err)
	}
	nome, bytes, _, err = lerInfoMac(fixture(t, "diskutil_info_disk4.plist"))
	if err != nil || nome != "SanDisk Ultra" || bytes != 32015679488 {
		t.Fatalf("info: %q %d %v", nome, bytes, err)
	}
	boot, err := lerBootMac(fixture(t, "diskutil_info_raiz.plist"))
	if err != nil || boot != "disk3" {
		t.Fatalf("boot: %q %v", boot, err)
	}
}

func TestMacListarMarcaBootETimeMachine(t *testing.T) {
	f := macFalso(t)
	// O disco do "/" passa a ser o disk7: container sintetizado em disk3
	// com loja física lá. Aqui só reaproveitamos o boot direto.
	raiz := strings.ReplaceAll(string(fixture(t, "diskutil_info_raiz.plist")), "<string>disk3</string>", "<string>disk7</string>")
	f.respostas["diskutil info -plist /"] = []byte(raiz)

	// disk4 ganha Backups.backupdb numa pasta temporária.
	tmp := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmp, "Backups.backupdb"), 0o755); err != nil {
		t.Fatal(err)
	}
	lista := strings.ReplaceAll(string(fixture(t, "diskutil_list_externos.plist")), "/Volumes/PENDRIVE", tmp)
	f.respostas["diskutil list -plist external physical"] = []byte(lista)

	brutos, err := NovoMac(f.exec).Listar(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(brutos) != 3 {
		t.Fatalf("esperava 3 discos, veio %+v", brutos)
	}
	por := map[string]Bruto{}
	for _, b := range brutos {
		por[b.ID] = b
	}
	d4, d5, d7 := por["/dev/disk4"], por["/dev/disk5"], por["/dev/disk7"]
	if d4.Nome != "SanDisk Ultra" || d4.Bytes != 32015679488 || !d4.TimeMachine || d4.Boot {
		t.Errorf("disk4: %+v", d4)
	}
	if len(d4.Montagens) != 1 || d4.Montagens[0] != tmp {
		t.Errorf("montagens do disk4: %v", d4.Montagens)
	}
	if !d5.TimeMachine || d5.Boot {
		t.Errorf("disk5 deveria ser Time Machine pelo papel Backup: %+v", d5)
	}
	if !d7.Boot || d7.TimeMachine {
		t.Errorf("disk7 deveria ser Boot: %+v", d7)
	}
	if got := Filtrar(brutos); len(got) != 0 {
		t.Errorf("o filtro deveria esconder os três: %+v", got)
	}
}

func TestMacListarDiscoLimpoAparece(t *testing.T) {
	f := macFalso(t)
	brutos, err := NovoMac(f.exec).Listar(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := Filtrar(brutos)
	if len(got) != 1 || got[0].ID != "/dev/disk4" || got[0].TimeMachine || got[0].Sistema || got[0].Boot {
		t.Fatalf("só o disk4 devia sobrar: %+v", got)
	}
}

// Um disco externo cujo "diskutil info" falha (ou vem ilegível) é pulado:
// não aparece e não derruba a lista dos outros.
func TestMacListarPulaDiscoIlegivel(t *testing.T) {
	for _, quebra := range []string{"erro", "ilegivel"} {
		f := macFalso(t)
		if quebra == "erro" {
			f.erros = map[string]error{"diskutil info -plist /dev/disk5": fmt.Errorf("exit status 1")}
		} else {
			f.respostas["diskutil info -plist /dev/disk5"] = []byte("isto não é plist")
		}
		brutos, err := NovoMac(f.exec).Listar(context.Background())
		if err != nil {
			t.Fatalf("%s: %v", quebra, err)
		}
		var ids []string
		for _, b := range brutos {
			ids = append(ids, b.ID)
		}
		if strings.Join(ids, ",") != "/dev/disk4,/dev/disk7" {
			t.Fatalf("%s: discos %v", quebra, ids)
		}
	}
}

// Erro geral continua erro: sem a lista ou sem os volumes APFS não há o que mostrar.
func TestMacListarErroGeral(t *testing.T) {
	for _, cmd := range []string{"diskutil list -plist external physical", "diskutil apfs list -plist"} {
		f := macFalso(t)
		f.erros = map[string]error{cmd: fmt.Errorf("exit status 1")}
		if _, err := NovoMac(f.exec).Listar(context.Background()); err == nil {
			t.Errorf("%s falhando deveria ser erro", cmd)
		}
	}
}

func TestMacBootViaContainerAPFS(t *testing.T) {
	// "/" vive no container disk6, cuja loja física é disk5s2: o boot é disk5.
	f := macFalso(t)
	raiz := strings.ReplaceAll(string(fixture(t, "diskutil_info_raiz.plist")), "<string>disk3</string>", "<string>disk6</string>")
	f.respostas["diskutil info -plist /"] = []byte(raiz)
	brutos, err := NovoMac(f.exec).Listar(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range brutos {
		if b.ID == "/dev/disk5" && !b.Boot {
			t.Fatalf("disk5 devia ser Boot via container: %+v", b)
		}
		if b.ID == "/dev/disk4" && b.Boot {
			t.Fatalf("disk4 não é boot: %+v", b)
		}
	}
}

// apagarFalso prepara as respostas para Apagar de /dev/disk4. A partição
// formatada monta em mnt (pasta que existe de verdade).
func apagarFalso(t *testing.T, mnt string) *falso {
	f := macFalso(t)
	depois := strings.ReplaceAll(string(fixture(t, "diskutil_info_disk4s1_depois.plist")), "/Volumes/CD2", mnt)
	f.respostas["diskutil unmountDisk force /dev/disk4"] = nil
	f.respostas["diskutil eraseDisk ExFAT CD2 MBR /dev/disk4"] = []byte("Finished erase on disk4\n")
	f.respostas["diskutil info -plist /dev/disk4s1"] = []byte(depois)
	return f
}

func temErase(f *falso) bool {
	for _, c := range f.chamadas {
		if strings.Contains(c, "eraseDisk") || strings.Contains(c, "unmountDisk") {
			return true
		}
	}
	return false
}

func TestMacApagarComandos(t *testing.T) {
	mnt := t.TempDir()
	f := apagarFalso(t, mnt)
	got, err := NovoMac(f.exec).Apagar(context.Background(), "/dev/disk4", "CD2")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"diskutil info -plist /dev/disk4",
		"diskutil info -plist /",
		"diskutil apfs list -plist",
		"diskutil unmountDisk force /dev/disk4",
		"diskutil eraseDisk ExFAT CD2 MBR /dev/disk4",
		"diskutil info -plist /dev/disk4s1",
	}
	if !reflect.DeepEqual(f.chamadas, want) {
		t.Fatalf("chamadas: %v", f.chamadas)
	}
	if got != mnt {
		t.Fatalf("montagem %q", got)
	}
}

func TestMacApagarRecusaInterno(t *testing.T) {
	f := apagarFalso(t, t.TempDir())
	interno := strings.Replace(string(fixture(t, "diskutil_info_disk4.plist")), "<key>Internal</key>\n\t<false/>", "<key>Internal</key>\n\t<true/>", 1)
	if !strings.Contains(interno, "<true/>") {
		t.Fatal("fixture não tem a chave Internal esperada")
	}
	f.respostas["diskutil info -plist /dev/disk4"] = []byte(interno)
	if _, err := NovoMac(f.exec).Apagar(context.Background(), "/dev/disk4", "CD2"); err == nil || temErase(f) {
		t.Fatalf("disco interno tem de ser recusado sem apagar: %v %v", err, f.chamadas)
	}
}

func TestMacApagarRecusaBoot(t *testing.T) {
	f := apagarFalso(t, t.TempDir())
	// "/" no container disk6, que mora em disk5: apagar disk5 é apagar o sistema.
	raiz := strings.ReplaceAll(string(fixture(t, "diskutil_info_raiz.plist")), "<string>disk3</string>", "<string>disk6</string>")
	f.respostas["diskutil info -plist /"] = []byte(raiz)
	if _, err := NovoMac(f.exec).Apagar(context.Background(), "/dev/disk5", "CD2"); err == nil || temErase(f) {
		t.Fatalf("disco de boot tem de ser recusado sem apagar: %v %v", err, f.chamadas)
	}
}

func TestMacApagarRecusaGrande(t *testing.T) {
	f := apagarFalso(t, t.TempDir())
	if _, err := NovoMac(f.exec).Apagar(context.Background(), "/dev/disk7", "CD2"); err == nil || temErase(f) {
		t.Fatalf("disco acima do limite tem de ser recusado: %v %v", err, f.chamadas)
	}
}

func TestMacApagarRecusaIDERotuloInvalidos(t *testing.T) {
	for _, id := range []string{"disk4", "/dev/disk4s1", "/dev/disk4;x", "/dev/rdisk4", ""} {
		f := apagarFalso(t, t.TempDir())
		if _, err := NovoMac(f.exec).Apagar(context.Background(), id, "CD2"); err == nil || len(f.chamadas) != 0 {
			t.Errorf("id %q: %v %v", id, err, f.chamadas)
		}
	}
	for _, r := range []string{"", "cd2", "CD2 COM NOME LONGO", "A'B", "CD2;"} {
		f := apagarFalso(t, t.TempDir())
		if _, err := NovoMac(f.exec).Apagar(context.Background(), "/dev/disk4", r); err == nil || len(f.chamadas) != 0 {
			t.Errorf("rótulo %q: %v %v", r, err, f.chamadas)
		}
	}
}

func TestMacApagarMontaSeNaoMontou(t *testing.T) {
	mnt := t.TempDir()
	f := apagarFalso(t, mnt)
	sem := strings.ReplaceAll(string(fixture(t, "diskutil_info_disk4s1_depois.plist")), "/Volumes/CD2", "/nao/existe")
	f.seq = map[string][][]byte{"diskutil info -plist /dev/disk4s1": {[]byte(sem), f.respostas["diskutil info -plist /dev/disk4s1"]}}
	f.respostas["diskutil mount /dev/disk4s1"] = nil
	got, err := NovoMac(f.exec).Apagar(context.Background(), "/dev/disk4", "CD2")
	if err != nil || got != mnt {
		t.Fatalf("%q %v", got, err)
	}
	if !contem(f.chamadas, "diskutil mount /dev/disk4s1") {
		t.Fatalf("chamadas: %v", f.chamadas)
	}
}

func TestMacApagarMontagemInexistenteDaErro(t *testing.T) {
	f := apagarFalso(t, "/nao/existe/mesmo")
	f.respostas["diskutil mount /dev/disk4s1"] = nil
	if _, err := NovoMac(f.exec).Apagar(context.Background(), "/dev/disk4", "CD2"); err == nil {
		t.Fatal("montagem que não é pasta deve dar erro")
	}
}

func TestMacApagarFalhaDoEraseDisk(t *testing.T) {
	f := apagarFalso(t, t.TempDir())
	delete(f.respostas, "diskutil eraseDisk ExFAT CD2 MBR /dev/disk4")
	f.erros = map[string]error{"diskutil eraseDisk ExFAT CD2 MBR /dev/disk4": fmt.Errorf("saiu 1")}
	if _, err := NovoMac(f.exec).Apagar(context.Background(), "/dev/disk4", "CD2"); err == nil {
		t.Fatal("esperava erro")
	}
}

func TestMacEjetar(t *testing.T) {
	f := &falso{respostas: map[string][]byte{"diskutil eject /dev/disk4": nil}}
	if err := NovoMac(f.exec).Ejetar(context.Background(), "/dev/disk4", "/Volumes/CD2"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.chamadas, []string{"diskutil eject /dev/disk4"}) {
		t.Fatalf("chamadas: %v", f.chamadas)
	}
}

func contem(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}
