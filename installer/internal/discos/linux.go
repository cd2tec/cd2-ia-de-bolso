package discos

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path"
	"regexp"
	"sort"
	"strings"
)

// No Linux quem lista é o lsblk (JSON) e quem formata, monta e ejeta é o
// UDisks2, o serviço de discos do desktop (o mesmo do app "Discos"). A
// formatação passa pelo polkit: o desktop abre a janela de senha quando
// precisa, sem pkexec nem sudo.

var (
	reIDLinux     = regexp.MustCompile(`^/dev/[a-z][a-z0-9]*$`)
	reRotuloLinux = regexp.MustCompile(`^[A-Z0-9 _-]{1,11}$`)
	reObjUDisks   = regexp.MustCompile(`^/org/freedesktop/UDisks2/block_devices/[a-z][a-z0-9]*$`)
)

// colunasLsblk são as colunas pedidas ao lsblk.
const colunasLsblk = "NAME,PATH,TYPE,SIZE,RM,HOTPLUG,TRAN,MODEL,VENDOR,MOUNTPOINTS"

// montagensDoSistema: partição montada num destes pontos é do sistema.
var montagensDoSistema = map[string]bool{
	"/boot": true, "/boot/efi": true, "/efi": true, "/home": true,
	"/usr": true, "/var": true, "[SWAP]": true,
}

type linuxSistema struct{ x Executor }

// NovoLinux devolve o Sistema que fala com o lsblk e o UDisks2 através de x.
func NovoLinux(x Executor) Sistema { return linuxSistema{x: x} }

// flexBool aceita true/false e "0"/"1" (lsblk antigo).
type flexBool bool

func (b *flexBool) UnmarshalJSON(p []byte) error {
	s := strings.Trim(string(p), `"`)
	*b = flexBool(s == "true" || s == "1")
	return nil
}

// flexInt aceita número e texto (lsblk antigo); null vira 0.
type flexInt int64

func (n *flexInt) UnmarshalJSON(p []byte) error {
	s := strings.Trim(string(p), `"`)
	if s == "" || s == "null" {
		*n = 0
		return nil
	}
	var v int64
	if _, err := fmt.Sscan(s, &v); err != nil {
		return err
	}
	*n = flexInt(v)
	return nil
}

type lsblkDisp struct {
	Nome       string      `json:"name"`
	Caminho    string      `json:"path"`
	Tipo       string      `json:"type"`
	Bytes      flexInt     `json:"size"`
	RM         flexBool    `json:"rm"`
	Hotplug    flexBool    `json:"hotplug"`
	Tran       *string     `json:"tran"`
	Modelo     *string     `json:"model"`
	Fabricante *string     `json:"vendor"`
	Montagens  []*string   `json:"mountpoints"`
	Filhos     []lsblkDisp `json:"children"`
}

func texto(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

// montagensDe devolve os pontos de montagem do dispositivo e dos filhos.
func (d lsblkDisp) montagensDe() []string {
	var ms []string
	for _, m := range d.Montagens {
		if t := texto(m); t != "" {
			ms = append(ms, t)
		}
	}
	for _, f := range d.Filhos {
		ms = append(ms, f.montagensDe()...)
	}
	return ms
}

// externo: removível, de encaixe a quente ou num barramento USB/cartão.
func (d lsblkDisp) externo() bool {
	t := texto(d.Tran)
	return bool(d.RM) || bool(d.Hotplug) || t == "usb" || t == "mmc"
}

// lerLsblk lê "lsblk -J -b -o ...": devolve os discos inteiros.
func lerLsblk(js []byte) ([]lsblkDisp, error) {
	var l struct {
		Disps []lsblkDisp `json:"blockdevices"`
	}
	if err := json.Unmarshal(js, &l); err != nil {
		return nil, fmt.Errorf("saída do lsblk ilegível: %w", err)
	}
	var discos []lsblkDisp
	for _, d := range l.Disps {
		if d.Tipo == "disk" {
			discos = append(discos, d)
		}
	}
	return discos, nil
}

// brutoLinux monta o Bruto de um disco do lsblk.
func brutoLinux(d lsblkDisp) Bruto {
	nome := strings.Join(strings.Fields(texto(d.Fabricante)+" "+texto(d.Modelo)), " ")
	b := Bruto{ID: d.Caminho, Nome: nome, Bytes: int64(d.Bytes), Montagens: d.montagensDe()}
	for _, m := range b.Montagens {
		if m == "/" {
			b.Boot = true
		}
		if montagensDoSistema[m] {
			b.Sistema = true
		}
	}
	return b
}

func (l linuxSistema) lsblk(ctx context.Context, disp ...string) ([]lsblkDisp, error) {
	args := append([]string{"-J", "-b", "-o", colunasLsblk}, disp...)
	saida, err := l.x(ctx, "lsblk", args...)
	if err != nil {
		return nil, fmt.Errorf("não consegui listar os discos: %w", err)
	}
	return lerLsblk(saida)
}

// Listar devolve só os discos externos com mídia (leitor de cartão vazio
// aparece com tamanho 0 e fica de fora).
func (l linuxSistema) Listar(ctx context.Context) ([]Bruto, error) {
	ds, err := l.lsblk(ctx)
	if err != nil {
		return nil, err
	}
	var brutos []Bruto
	for _, d := range ds {
		if !d.externo() || d.Bytes <= 0 || !reIDLinux.MatchString(d.Caminho) {
			continue
		}
		brutos = append(brutos, brutoLinux(d))
	}
	sort.SliceStable(brutos, func(i, j int) bool { return brutos[i].ID < brutos[j].ID })
	return brutos, nil
}

// umDisco lê um disco só pelo caminho e confere que é um disco inteiro.
func (l linuxSistema) umDisco(ctx context.Context, id string) (lsblkDisp, error) {
	ds, err := l.lsblk(ctx, id)
	if err != nil {
		return lsblkDisp{}, err
	}
	if len(ds) != 1 || ds[0].Caminho != id {
		return lsblkDisp{}, fmt.Errorf("%s não é um disco inteiro", id)
	}
	return ds[0], nil
}

// busctl chama um método do UDisks2 no barramento do sistema. A janela de
// senha do polkit pode demorar o tempo da pessoa: daí o prazo longo.
func (l linuxSistema) busctl(ctx context.Context, objeto, iface, metodo string, args ...string) ([]byte, error) {
	base := []string{"--system", "--json=short", "--timeout=600", "--allow-interactive-authorization=yes",
		"call", "org.freedesktop.UDisks2", objeto, "org.freedesktop.UDisks2." + iface, metodo}
	return l.x(ctx, "busctl", append(base, args...)...)
}

// desmontar desmonta tudo o que estiver montado do disco (ele e as partições).
func (l linuxSistema) desmontar(ctx context.Context, d lsblkDisp) error {
	var todos []lsblkDisp
	var juntar func(lsblkDisp)
	juntar = func(x lsblkDisp) {
		todos = append(todos, x)
		for _, f := range x.Filhos {
			juntar(f)
		}
	}
	juntar(d)
	for _, x := range todos {
		montado := false
		for _, m := range x.Montagens {
			if texto(m) != "" {
				montado = true
			}
		}
		if !montado {
			continue
		}
		if _, err := l.x(ctx, "udisksctl", "unmount", "-b", x.Caminho); err != nil {
			return fmt.Errorf("não consegui desmontar %s (algum arquivo aberto?): %w", x.Caminho, err)
		}
	}
	return nil
}

func (l linuxSistema) Apagar(ctx context.Context, id, rotulo string) (string, error) {
	if !reIDLinux.MatchString(id) {
		return "", fmt.Errorf("identificador de disco inválido: %q", id)
	}
	if !reRotuloLinux.MatchString(rotulo) {
		return "", fmt.Errorf("rótulo inválido: %q", rotulo)
	}
	// Última checagem antes de apagar: ainda é um disco externo, pequeno o
	// bastante e sem nada do sistema montado.
	d, err := l.umDisco(ctx, id)
	if err != nil {
		return "", err
	}
	b := brutoLinux(d)
	switch {
	case !d.externo():
		return "", fmt.Errorf("%s é um disco interno. Nada foi apagado", id)
	case b.Boot || b.Sistema:
		return "", fmt.Errorf("%s é o disco do sistema. Nada foi apagado", id)
	case b.Bytes <= 0 || b.Bytes > LimiteBytes:
		return "", fmt.Errorf("%s tem um tamanho que não é de pen drive. Nada foi apagado", id)
	}

	if err := l.desmontar(ctx, d); err != nil {
		return "", err
	}
	obj := "/org/freedesktop/UDisks2/block_devices/" + path.Base(id)
	// Tabela MBR nova (apaga as partições) e uma partição exFAT no disco todo.
	if out, err := l.busctl(ctx, obj, "Block", "Format", "sa{sv}", "dos", "0"); err != nil {
		return "", fmt.Errorf("a formatação falhou: %w%s", err, sufixo(out))
	}
	out, err := l.busctl(ctx, obj, "PartitionTable", "CreatePartitionAndFormat",
		"ttssa{sv}sa{sv}", "0", "0", "0x07", "", "0", "exfat", "1", "label", "s", rotulo)
	if err != nil {
		return "", fmt.Errorf("a formatação falhou: %w%s", err, sufixo(out))
	}
	part, err := lerObjetoCriado(out)
	if err != nil {
		return "", err
	}

	// O desktop pode montar sozinho logo depois de formatar; se não montou, monta.
	mnt, err := l.montagem(ctx, part)
	if err != nil {
		return "", err
	}
	if !ehDir(mnt) {
		_, errMontar := l.x(ctx, "udisksctl", "mount", "-b", part)
		if mnt, err = l.montagem(ctx, part); err != nil {
			return "", err
		}
		if !ehDir(mnt) && errMontar != nil {
			return "", fmt.Errorf("formatei, mas não consegui montar %s: %w", part, errMontar)
		}
	}
	if !ehDir(mnt) {
		return "", fmt.Errorf("formatei, mas não achei o ponto de montagem de %s", part)
	}
	return mnt, nil
}

// lerObjetoCriado lê a resposta do CreatePartitionAndFormat ({"type":"o",
// "data":["/org/.../block_devices/sdb1"]}) e devolve o dispositivo /dev/sdb1.
func lerObjetoCriado(js []byte) (string, error) {
	var r struct {
		Data []string `json:"data"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(js), &r); err != nil || len(r.Data) != 1 || !reObjUDisks.MatchString(r.Data[0]) {
		return "", fmt.Errorf("formatei, mas não entendi qual partição foi criada: %q", strings.TrimSpace(string(js)))
	}
	return "/dev/" + path.Base(r.Data[0]), nil
}

// montagem devolve onde a partição está montada, ou "".
func (l linuxSistema) montagem(ctx context.Context, part string) (string, error) {
	saida, err := l.x(ctx, "lsblk", "-J", "-o", "PATH,MOUNTPOINTS", part)
	if err != nil {
		return "", fmt.Errorf("não consegui ler %s: %w", part, err)
	}
	var lista struct {
		Disps []lsblkDisp `json:"blockdevices"`
	}
	if err := json.Unmarshal(saida, &lista); err != nil {
		return "", fmt.Errorf("saída do lsblk ilegível: %w", err)
	}
	for _, d := range lista.Disps {
		if ms := d.montagensDe(); d.Caminho == part && len(ms) > 0 {
			return ms[0], nil
		}
	}
	return "", nil
}

// Ejetar desmonta (o que grava o que falta) e desliga o pen drive. Se só o
// desligar falhar, os dados já estão salvos: não é erro.
func (l linuxSistema) Ejetar(ctx context.Context, id, _ string) error {
	if !reIDLinux.MatchString(id) {
		return fmt.Errorf("identificador de disco inválido: %q", id)
	}
	d, err := l.umDisco(ctx, id)
	if err != nil {
		return fmt.Errorf("não consegui ejetar %s: %w", id, err)
	}
	if err := l.desmontar(ctx, d); err != nil {
		return fmt.Errorf("não consegui ejetar %s: %w", id, err)
	}
	if _, err := l.x(ctx, "udisksctl", "power-off", "-b", id); err != nil {
		log.Printf("discos: desmontei %s, mas não consegui desligar: %v", id, err)
	}
	return nil
}
