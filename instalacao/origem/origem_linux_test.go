//go:build linux

package origem

import (
	"os"
	"path/filepath"
	"testing"
)

// sysFalso monta um /sys de mentira: dev/block/<maj:min> aponta para o
// dispositivo, como no kernel. part: nome da partição ("" = disco inteiro).
func sysFalso(t *testing.T, sys, majmin, caminhoDisco, part, removable string) {
	t.Helper()
	disco := filepath.Join(sys, caminhoDisco)
	alvo := disco
	if part != "" {
		alvo = filepath.Join(disco, part)
		os.MkdirAll(alvo, 0o755)
		os.WriteFile(filepath.Join(alvo, "partition"), []byte("1\n"), 0o644)
	}
	os.MkdirAll(disco, 0o755)
	os.WriteFile(filepath.Join(disco, "removable"), []byte(removable+"\n"), 0o644)
	os.MkdirAll(filepath.Join(sys, "dev", "block"), 0o755)
	if err := os.Symlink(alvo, filepath.Join(sys, "dev", "block", majmin)); err != nil {
		t.Fatal(err)
	}
}

func cenario(t *testing.T, mountinfo string) string {
	t.Helper()
	dir := t.TempDir()
	mi := filepath.Join(dir, "mountinfo")
	os.WriteFile(mi, []byte(mountinfo), 0o644)
	sys := filepath.Join(dir, "sys")
	sysFalso(t, sys, "259:2", "devices/pci0/nvme/nvme0n1", "nvme0n1p2", "0")
	sysFalso(t, sys, "259:3", "devices/pci0/nvme/nvme0n1", "nvme0n1p3", "0")
	sysFalso(t, sys, "8:17", "devices/pci0/usb2/2-1/host0/block/sdb", "sdb1", "1")
	sysFalso(t, sys, "8:33", "devices/pci0/ata1/host1/block/sdc", "sdc1", "0")
	antesMi, antesSys := arquivoMountinfo, raizSys
	arquivoMountinfo, raizSys = mi, sys
	t.Cleanup(func() { arquivoMountinfo, raizSys = antesMi, antesSys })
	return dir
}

const mountinfoBase = `31 1 259:2 / / rw,relatime shared:1 - ext4 /dev/nvme0n1p2 rw
40 31 259:3 / /home rw,relatime shared:2 - ext4 /dev/nvme0n1p3 rw
50 31 8:17 / /media/ana/CD2\040IA rw,nosuid shared:3 - exfat /dev/sdb1 rw
60 31 8:33 / /mnt/dados rw shared:4 - ext4 /dev/sdc1 rw
`

func TestLinuxOrigem(t *testing.T) {
	cenario(t, mountinfoBase)
	casos := []struct {
		caminho string
		quer    Origem
	}{
		{"/home/ana/work/cd2-ia-de-bolso", Instalada}, // /home: outra partição do disco do sistema
		{"/opt/cd2", Instalada},                       // a própria raiz
		{"/media/ana/CD2 IA", Pendrive},               // pen drive USB com espaço no nome
		{"/media/ana/CD2 IA/core", Pendrive},
		{"/mnt/dados/ia", Instalada}, // segundo disco interno, como no Windows e no Mac
	}
	for _, c := range casos {
		v, err := lerVolumeDoSistema(c.caminho)
		if err != nil {
			t.Fatalf("%s: %v", c.caminho, err)
		}
		if got := Regra(v); got != c.quer {
			t.Errorf("%s: origem %q (volume %+v), quer %q", c.caminho, got, v, c.quer)
		}
	}
}

func TestLinuxMontagemMaisLonga(t *testing.T) {
	ms := []montagem{{"/", "1"}, {"/media/ana", "2"}, {"/media/ana/CD2", "3"}}
	if m, _ := montagemDe(ms, "/media/ana/CD2/web"); m.dispositivo != "3" {
		t.Fatalf("montagem = %+v", m)
	}
	if m, _ := montagemDe(ms, "/media/ana/CD2x"); m.dispositivo != "2" {
		t.Fatalf("prefixo sem barra não conta: %+v", m)
	}
}

func TestLinuxSemMountinfoNaDuvidaEPendrive(t *testing.T) {
	antes := arquivoMountinfo
	arquivoMountinfo = filepath.Join(t.TempDir(), "nao-existe")
	defer func() { arquivoMountinfo = antes }()
	if got := Detectar("/qualquer"); got != Pendrive {
		t.Fatalf("Detectar = %q", got)
	}
}

func TestDesescapar(t *testing.T) {
	for in, quer := range map[string]string{`a\040b`: "a b", `sem`: "sem", `x\134y`: `x\y`, `fim\04`: `fim\04`} {
		if got := desescapar(in); got != quer {
			t.Errorf("desescapar(%q) = %q, quer %q", in, got, quer)
		}
	}
}
