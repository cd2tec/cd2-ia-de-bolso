//go:build linux

package origem

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Onde o Linux descreve montagens e discos. Em variáveis para o teste.
var (
	arquivoMountinfo = "/proc/self/mountinfo"
	raizSys          = "/sys"
)

// montagem é uma linha do mountinfo: o ponto de montagem e o dispositivo (major:minor).
type montagem struct {
	ponto, dispositivo string
}

// lerMountinfo lê as montagens. O ponto vem com os escapes do kernel desfeitos (\040 = espaço).
func lerMountinfo(arq string) ([]montagem, error) {
	f, err := os.Open(arq)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var ms []montagem
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		// id pai major:minor raiz ponto opções ... - tipo origem superopções
		c := strings.Fields(sc.Text())
		if len(c) < 5 {
			continue
		}
		ms = append(ms, montagem{ponto: desescapar(c[4]), dispositivo: c[2]})
	}
	return ms, sc.Err()
}

// desescapar desfaz os \ooo (octal) que o kernel usa em espaço, tab, \n e \\.
func desescapar(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			var v byte
			ok := true
			for _, d := range []byte(s[i+1 : i+4]) {
				if d < '0' || d > '7' {
					ok = false
					break
				}
				v = v*8 + (d - '0')
			}
			if ok {
				b.WriteByte(v)
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// montagemDe devolve a montagem mais longa que contém o caminho (o volume dele).
func montagemDe(ms []montagem, caminho string) (montagem, bool) {
	var melhor montagem
	achou := false
	for _, m := range ms {
		dentro := m.ponto == "/" || caminho == m.ponto || strings.HasPrefix(caminho, m.ponto+"/")
		if dentro && (!achou || len(m.ponto) >= len(melhor.ponto)) {
			melhor, achou = m, true
		}
	}
	return melhor, achou
}

// disco descreve, pelo /sys, o disco de um dispositivo major:minor: o nome do
// disco inteiro (sem a partição), se é removível e se está num barramento USB
// ou de cartão (mmc). Volume de LVM, LUKS ou rede não tem esses dados e volta
// sem nome: conta como disco fixo.
type disco struct {
	nome              string
	removivel, usbMmc bool
}

func discoDe(sys, dispositivo string) disco {
	real, err := filepath.EvalSymlinks(filepath.Join(sys, "dev", "block", dispositivo))
	if err != nil {
		return disco{}
	}
	// Partição: o disco é a pasta de cima (…/block/sdb/sdb1 -> sdb).
	if _, err := os.Stat(filepath.Join(real, "partition")); err == nil {
		real = filepath.Dir(real)
	}
	d := disco{nome: filepath.Base(real)}
	if b, err := os.ReadFile(filepath.Join(real, "removable")); err == nil {
		d.removivel = strings.TrimSpace(string(b)) == "1"
	}
	d.usbMmc = strings.Contains(real, "/usb") || strings.Contains(real, "/mmc")
	return d
}

func lerVolumeDoSistema(raiz string) (Volume, error) {
	abs, err := filepath.Abs(raiz)
	if err != nil {
		return Volume{}, err
	}
	if r, err := filepath.EvalSymlinks(abs); err == nil {
		abs = r
	}
	ms, err := lerMountinfo(arquivoMountinfo)
	if err != nil {
		return Volume{}, fmt.Errorf("não consegui ler as montagens: %w", err)
	}
	m, ok := montagemDe(ms, abs)
	if !ok {
		return Volume{}, errors.New("não achei o volume de " + abs)
	}
	if m.ponto == "/" {
		return Volume{DoSistema: true}, nil
	}
	d := discoDe(raizSys, m.dispositivo)
	// Outra partição do mesmo disco do sistema (ex.: /home separado) também é do sistema.
	if barra, ok := montagemDe(ms, "/"); ok && d.nome != "" && discoDe(raizSys, barra.dispositivo).nome == d.nome {
		return Volume{DoSistema: true}, nil
	}
	return Volume{Removivel: d.removivel, USB: d.usbMmc}, nil
}
