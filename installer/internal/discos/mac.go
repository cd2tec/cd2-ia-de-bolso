package discos

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"howett.net/plist"
)

// Sempre -plist: a saída em texto do diskutil muda com o idioma do sistema.

var (
	reIDMac     = regexp.MustCompile(`^/dev/disk[0-9]+$`)
	reRotuloMac = regexp.MustCompile(`^[A-Z0-9 _-]{1,11}$`)
)

type macSistema struct{ x Executor }

// NovoMac devolve o Sistema que fala com o diskutil através de x.
func NovoMac(x Executor) Sistema { return macSistema{x: x} }

type macLista struct {
	WholeDisks []string `plist:"WholeDisks"`
	Discos     []struct {
		DeviceIdentifier string `plist:"DeviceIdentifier"`
		Partitions       []struct {
			MountPoint string `plist:"MountPoint"`
		} `plist:"Partitions"`
		APFSVolumes []struct {
			MountPoint string `plist:"MountPoint"`
		} `plist:"APFSVolumes"`
	} `plist:"AllDisksAndPartitions"`
}

// lerListaMac devolve os discos inteiros de "diskutil list -plist ...".
func lerListaMac(p []byte) ([]string, error) {
	var l macLista
	if _, err := plist.Unmarshal(p, &l); err != nil {
		return nil, fmt.Errorf("lista do diskutil ilegível: %w", err)
	}
	return l.WholeDisks, nil
}

// lerMontagensMac devolve, por disco inteiro, os pontos de montagem das
// partições dele.
func lerMontagensMac(p []byte) (map[string][]string, error) {
	var l macLista
	if _, err := plist.Unmarshal(p, &l); err != nil {
		return nil, fmt.Errorf("lista do diskutil ilegível: %w", err)
	}
	m := map[string][]string{}
	for _, d := range l.Discos {
		for _, part := range d.Partitions {
			if part.MountPoint != "" {
				m[d.DeviceIdentifier] = append(m[d.DeviceIdentifier], part.MountPoint)
			}
		}
		for _, v := range d.APFSVolumes {
			if v.MountPoint != "" {
				m[d.DeviceIdentifier] = append(m[d.DeviceIdentifier], v.MountPoint)
			}
		}
	}
	return m, nil
}

type macInfo struct {
	MediaName       string `plist:"MediaName"`
	Size            int64  `plist:"Size"`
	TotalSize       int64  `plist:"TotalSize"`
	MountPoint      string `plist:"MountPoint"`
	ParentWholeDisk string `plist:"ParentWholeDisk"`
	Internal        bool   `plist:"Internal"`
}

// lerInfoMac lê "diskutil info -plist <disco>".
func lerInfoMac(p []byte) (nome string, bytes int64, montagem string, err error) {
	var i macInfo
	if _, err = plist.Unmarshal(p, &i); err != nil {
		return "", 0, "", fmt.Errorf("info do diskutil ilegível: %w", err)
	}
	bytes = i.Size
	if bytes == 0 {
		bytes = i.TotalSize
	}
	return strings.TrimSpace(i.MediaName), bytes, i.MountPoint, nil
}

// lerBootMac lê o disco inteiro que contém o "/" (diskutil info -plist /).
func lerBootMac(p []byte) (string, error) {
	var i macInfo
	if _, err := plist.Unmarshal(p, &i); err != nil {
		return "", fmt.Errorf("info do diskutil ilegível: %w", err)
	}
	if i.ParentWholeDisk == "" {
		return "", fmt.Errorf("o diskutil não disse em que disco está o /")
	}
	return i.ParentWholeDisk, nil
}

type macApfs struct {
	Containers []struct {
		ContainerReference string `plist:"ContainerReference"`
		PhysicalStores     []struct {
			DeviceIdentifier string `plist:"DeviceIdentifier"`
		} `plist:"PhysicalStores"`
		Volumes []struct {
			Roles []string `plist:"Roles"`
		} `plist:"Volumes"`
	} `plist:"Containers"`
}

// discoInteiro tira a partição: "disk5s2" vira "disk5".
func discoInteiro(id string) string {
	id = strings.TrimPrefix(id, "/dev/")
	if !strings.HasPrefix(id, "disk") {
		return id
	}
	i := 4
	for i < len(id) && id[i] >= '0' && id[i] <= '9' {
		i++
	}
	return id[:i]
}

// lerApfsMac devolve, por disco inteiro físico, os papéis dos volumes APFS
// que moram nele (System, Data, Backup...).
func lerApfsMac(p []byte) (map[string][]string, error) {
	var a macApfs
	if _, err := plist.Unmarshal(p, &a); err != nil {
		return nil, fmt.Errorf("lista APFS ilegível: %w", err)
	}
	m := map[string][]string{}
	for _, c := range a.Containers {
		var papeis []string
		for _, v := range c.Volumes {
			papeis = append(papeis, v.Roles...)
		}
		for _, s := range c.PhysicalStores {
			d := discoInteiro(s.DeviceIdentifier)
			m[d] = append(m[d], papeis...)
		}
	}
	return m, nil
}

// lerContainersMac liga cada container APFS (disk3, sintetizado) aos discos
// físicos onde ele guarda os dados. O "/" mora no container, não no físico.
func lerContainersMac(p []byte) (map[string][]string, error) {
	var a macApfs
	if _, err := plist.Unmarshal(p, &a); err != nil {
		return nil, fmt.Errorf("lista APFS ilegível: %w", err)
	}
	m := map[string][]string{}
	for _, c := range a.Containers {
		for _, s := range c.PhysicalStores {
			m[c.ContainerReference] = append(m[c.ContainerReference], discoInteiro(s.DeviceIdentifier))
		}
	}
	return m, nil
}

// sistemaMac devolve os discos físicos onde o macOS em uso mora e os papéis
// APFS de cada disco. O "/" pode morar num container sintetizado; o boot de
// verdade é o disco físico dele.
func (m macSistema) sistemaMac(ctx context.Context) (boot map[string]bool, papeis map[string][]string, err error) {
	raiz, err := m.x(ctx, "diskutil", "info", "-plist", "/")
	if err != nil {
		return nil, nil, fmt.Errorf("não consegui saber qual é o disco do sistema: %w", err)
	}
	disco, err := lerBootMac(raiz)
	if err != nil {
		return nil, nil, err
	}
	apfsPlist, err := m.x(ctx, "diskutil", "apfs", "list", "-plist")
	if err != nil {
		return nil, nil, fmt.Errorf("não consegui ler os volumes APFS: %w", err)
	}
	if papeis, err = lerApfsMac(apfsPlist); err != nil {
		return nil, nil, err
	}
	containers, err := lerContainersMac(apfsPlist)
	if err != nil {
		return nil, nil, err
	}
	boot = map[string]bool{disco: true}
	for _, f := range containers[disco] {
		boot[f] = true
	}
	return boot, papeis, nil
}

func (m macSistema) Listar(ctx context.Context) ([]Bruto, error) {
	saida, err := m.x(ctx, "diskutil", "list", "-plist", "external", "physical")
	if err != nil {
		return nil, fmt.Errorf("não consegui listar os discos: %w", err)
	}
	inteiros, err := lerListaMac(saida)
	if err != nil {
		return nil, err
	}
	if len(inteiros) == 0 {
		return nil, nil
	}
	montagens, err := lerMontagensMac(saida)
	if err != nil {
		return nil, err
	}

	bootFisicos, papeis, err := m.sistemaMac(ctx)
	if err != nil {
		return nil, err
	}

	var brutos []Bruto
	for _, d := range inteiros {
		id := "/dev/" + d
		// Disco que não dá para ler é pulado: não aparece (nunca é tratado
		// como seguro) e não derruba a lista dos outros.
		info, err := m.x(ctx, "diskutil", "info", "-plist", id)
		if err != nil {
			log.Printf("discos: pulei %s, o diskutil info falhou: %v", d, err)
			continue
		}
		nome, bytes, _, err := lerInfoMac(info)
		if err != nil {
			log.Printf("discos: pulei %s: %v", d, err)
			continue
		}
		b := Bruto{ID: id, Nome: nome, Bytes: bytes, Boot: bootFisicos[d], Montagens: montagens[d]}
		for _, papel := range papeis[d] {
			switch papel {
			case "System", "Data":
				b.Sistema = true
			case "Backup":
				b.TimeMachine = true
			}
		}
		for _, mnt := range b.Montagens {
			if _, err := os.Stat(filepath.Join(mnt, "Backups.backupdb")); err == nil {
				b.TimeMachine = true
			}
		}
		brutos = append(brutos, b)
	}
	sort.SliceStable(brutos, func(i, j int) bool { return brutos[i].ID < brutos[j].ID })
	return brutos, nil
}

func (m macSistema) Apagar(ctx context.Context, id, rotulo string) (string, error) {
	if !reIDMac.MatchString(id) {
		return "", fmt.Errorf("identificador de disco inválido: %q", id)
	}
	if !reRotuloMac.MatchString(rotulo) {
		return "", fmt.Errorf("rótulo inválido: %q", rotulo)
	}
	// Última checagem antes de apagar: o disco ainda é um externo, pequeno
	// o bastante e não é o do sistema.
	infoDisco, err := m.x(ctx, "diskutil", "info", "-plist", id)
	if err != nil {
		return "", fmt.Errorf("não consegui ler o disco %s: %w", id, err)
	}
	var i macInfo
	if _, err := plist.Unmarshal(infoDisco, &i); err != nil {
		return "", fmt.Errorf("info do diskutil ilegível: %w", err)
	}
	if i.Internal {
		return "", fmt.Errorf("%s é um disco interno. Nada foi apagado", id)
	}
	tam := i.TotalSize
	if tam == 0 {
		tam = i.Size
	}
	if tam > LimiteBytes {
		return "", fmt.Errorf("%s é grande demais para ser um pen drive. Nada foi apagado", id)
	}
	boot, _, err := m.sistemaMac(ctx)
	if err != nil {
		return "", err
	}
	if boot[discoInteiro(id)] {
		return "", fmt.Errorf("%s é o disco do sistema. Nada foi apagado", id)
	}

	// Falhar ao desmontar não impede o eraseDisk: ele mesmo tenta desmontar.
	_, _ = m.x(ctx, "diskutil", "unmountDisk", "force", id)
	if out, err := m.x(ctx, "diskutil", "eraseDisk", "ExFAT", rotulo, "MBR", id); err != nil {
		return "", fmt.Errorf("a formatação falhou: %w%s", err, sufixo(out))
	}
	part := id + "s1"
	mnt, err := m.montagem(ctx, part)
	if err != nil {
		return "", err
	}
	if !ehDir(mnt) {
		if _, err := m.x(ctx, "diskutil", "mount", part); err != nil {
			return "", fmt.Errorf("formatei, mas não consegui montar %s: %w", part, err)
		}
		if mnt, err = m.montagem(ctx, part); err != nil {
			return "", err
		}
	}
	if !ehDir(mnt) {
		return "", fmt.Errorf("formatei, mas não achei o ponto de montagem de %s", part)
	}
	return mnt, nil
}

func ehDir(p string) bool {
	if p == "" {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func (m macSistema) montagem(ctx context.Context, part string) (string, error) {
	info, err := m.x(ctx, "diskutil", "info", "-plist", part)
	if err != nil {
		return "", fmt.Errorf("não consegui ler %s: %w", part, err)
	}
	_, _, mnt, err := lerInfoMac(info)
	return mnt, err
}

func (m macSistema) Ejetar(ctx context.Context, id, _ string) error {
	if _, err := m.x(ctx, "diskutil", "eject", id); err != nil {
		return fmt.Errorf("não consegui ejetar %s: %w", id, err)
	}
	return nil
}

func sufixo(out []byte) string {
	s := strings.TrimSpace(string(out))
	if s == "" {
		return ""
	}
	return ": " + s
}
