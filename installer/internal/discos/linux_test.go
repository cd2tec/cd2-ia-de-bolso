package discos

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

const lsblkTudo = `{"blockdevices":[
 {"name":"sda","path":"/dev/sda","type":"disk","size":0,"rm":true,"hotplug":true,"tran":"usb","model":"Storage Device","vendor":"Mass    ","mountpoints":[null]},
 {"name":"sdb","path":"/dev/sdb","type":"disk","size":32010928128,"rm":true,"hotplug":true,"tran":"usb","model":"Ultra","vendor":"SanDisk ","mountpoints":[null],
  "children":[{"name":"sdb1","path":"/dev/sdb1","type":"part","size":32009879552,"rm":true,"hotplug":true,"tran":"usb","model":null,"vendor":null,"mountpoints":["/media/ana/FOTOS"]}]},
 {"name":"nvme0n1","path":"/dev/nvme0n1","type":"disk","size":512110190592,"rm":false,"hotplug":false,"tran":"nvme","model":"SSD 512GB","vendor":null,"mountpoints":[null],
  "children":[{"name":"nvme0n1p1","path":"/dev/nvme0n1p1","type":"part","size":1,"rm":false,"hotplug":false,"tran":"nvme","model":null,"vendor":null,"mountpoints":["/boot/efi"]},
              {"name":"nvme0n1p2","path":"/dev/nvme0n1p2","type":"part","size":1,"rm":false,"hotplug":false,"tran":"nvme","model":null,"vendor":null,"mountpoints":["/"]}]},
 {"name":"sdc","path":"/dev/sdc","type":"disk","size":"64000000000","rm":"0","hotplug":"1","tran":"usb","model":"Externo","vendor":null,"mountpoints":[null],
  "children":[{"name":"sdc1","path":"/dev/sdc1","type":"part","size":1,"rm":"0","hotplug":"1","tran":"usb","model":null,"vendor":null,"mountpoints":["/home"]}]},
 {"name":"loop0","path":"/dev/loop0","type":"loop","size":1000,"rm":false,"hotplug":false,"tran":null,"model":null,"vendor":null,"mountpoints":["/snap/x"]}
]}`

const lsblkSdb = `{"blockdevices":[
 {"name":"sdb","path":"/dev/sdb","type":"disk","size":32010928128,"rm":true,"hotplug":true,"tran":"usb","model":"Ultra","vendor":"SanDisk ","mountpoints":[null],
  "children":[{"name":"sdb1","path":"/dev/sdb1","type":"part","size":32009879552,"rm":true,"hotplug":true,"tran":"usb","model":null,"vendor":null,"mountpoints":["/media/ana/FOTOS"]}]}]}`

const lsblkCmd = "lsblk -J -b -o " + colunasLsblk
const busPrefixo = "busctl --system --json=short --timeout=600 --allow-interactive-authorization=yes call org.freedesktop.UDisks2 /org/freedesktop/UDisks2/block_devices/sdb org.freedesktop.UDisks2."

func TestLinuxListar(t *testing.T) {
	f := &falso{respostas: map[string][]byte{lsblkCmd: []byte(lsblkTudo)}}
	got, err := NovoLinux(f.exec).Listar(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	quer := []Bruto{
		{ID: "/dev/sdb", Nome: "SanDisk Ultra", Bytes: 32010928128, Montagens: []string{"/media/ana/FOTOS"}},
		{ID: "/dev/sdc", Nome: "Externo", Bytes: 64000000000, Sistema: true, Montagens: []string{"/home"}},
	}
	if !reflect.DeepEqual(got, quer) {
		t.Fatalf("Listar =\n%+v\nquer\n%+v", got, quer)
	}
	// O disco com /home montado some no filtro.
	if f := Filtrar(got); len(f) != 1 || f[0].ID != "/dev/sdb" {
		t.Fatalf("Filtrar = %+v", f)
	}
}

func linuxApagarFalso(t *testing.T) (*falso, string) {
	mnt := t.TempDir()
	return &falso{
		respostas: map[string][]byte{
			lsblkCmd + " /dev/sdb":                   []byte(lsblkSdb),
			"udisksctl unmount -b /dev/sdb1":         nil,
			busPrefixo + "Block Format sa{sv} dos 0": nil,
			busPrefixo + "PartitionTable CreatePartitionAndFormat ttssa{sv}sa{sv} 0 0 0x07  0 exfat 1 label s CD2": []byte(`{"type":"o","data":["/org/freedesktop/UDisks2/block_devices/sdb1"]}` + "\n"),
			"udisksctl mount -b /dev/sdb1": nil,
		},
		seq: map[string][][]byte{
			"lsblk -J -o PATH,MOUNTPOINTS /dev/sdb1": {
				[]byte(`{"blockdevices":[{"path":"/dev/sdb1","mountpoints":[null]}]}`),
				[]byte(`{"blockdevices":[{"path":"/dev/sdb1","mountpoints":["` + mnt + `"]}]}`),
			},
		},
	}, mnt
}

func TestLinuxApagar(t *testing.T) {
	f, mnt := linuxApagarFalso(t)
	got, err := NovoLinux(f.exec).Apagar(context.Background(), "/dev/sdb", "CD2")
	if err != nil {
		t.Fatalf("%v\nchamadas: %q", err, f.chamadas)
	}
	if got != mnt {
		t.Fatalf("montagem = %q, quer %q", got, mnt)
	}
	quer := []string{
		lsblkCmd + " /dev/sdb",
		"udisksctl unmount -b /dev/sdb1",
		busPrefixo + "Block Format sa{sv} dos 0",
		busPrefixo + "PartitionTable CreatePartitionAndFormat ttssa{sv}sa{sv} 0 0 0x07  0 exfat 1 label s CD2",
		"lsblk -J -o PATH,MOUNTPOINTS /dev/sdb1",
		"udisksctl mount -b /dev/sdb1",
		"lsblk -J -o PATH,MOUNTPOINTS /dev/sdb1",
	}
	if !reflect.DeepEqual(f.chamadas, quer) {
		t.Fatalf("chamadas =\n%s", strings.Join(f.chamadas, "\n"))
	}
}

func TestLinuxApagarRecusa(t *testing.T) {
	casos := map[string]struct{ id, rotulo, lsblk string }{
		"id inválido":     {"/dev/sdb; rm -rf /", "CD2", ""},
		"rótulo inválido": {"/dev/sdb", "cd2'", ""},
		"disco interno":   {"/dev/sdb", "CD2", `{"blockdevices":[{"name":"sdb","path":"/dev/sdb","type":"disk","size":32000000000,"rm":false,"hotplug":false,"tran":"sata","mountpoints":[null]}]}`},
		"sistema montado": {"/dev/sdb", "CD2", `{"blockdevices":[{"name":"sdb","path":"/dev/sdb","type":"disk","size":32000000000,"rm":true,"hotplug":true,"tran":"usb","mountpoints":[null],
			"children":[{"name":"sdb1","path":"/dev/sdb1","type":"part","size":1,"mountpoints":["/"]}]}]}`},
		"grande demais": {"/dev/sdb", "CD2", `{"blockdevices":[{"name":"sdb","path":"/dev/sdb","type":"disk","size":2000000000000,"rm":true,"hotplug":true,"tran":"usb","mountpoints":[null]}]}`},
		"é partição":    {"/dev/sdb", "CD2", `{"blockdevices":[{"name":"sdb1","path":"/dev/sdb1","type":"part","size":32000000000,"rm":true,"hotplug":true,"tran":"usb","mountpoints":[null]}]}`},
	}
	for nome, c := range casos {
		f := &falso{respostas: map[string][]byte{lsblkCmd + " /dev/sdb": []byte(c.lsblk)}}
		if _, err := NovoLinux(f.exec).Apagar(context.Background(), c.id, c.rotulo); err == nil {
			t.Errorf("%s: devia recusar", nome)
		}
		for _, ch := range f.chamadas {
			if strings.HasPrefix(ch, "busctl") || strings.HasPrefix(ch, "udisksctl") {
				t.Errorf("%s: mexeu no disco: %s", nome, ch)
			}
		}
	}
}

func TestLinuxEjetar(t *testing.T) {
	f := &falso{
		respostas: map[string][]byte{
			lsblkCmd + " /dev/sdb":           []byte(lsblkSdb),
			"udisksctl unmount -b /dev/sdb1": nil,
		},
		erros: map[string]error{"udisksctl power-off -b /dev/sdb": context.DeadlineExceeded},
	}
	// Desligar falhou, mas desmontou: os dados estão salvos.
	if err := NovoLinux(f.exec).Ejetar(context.Background(), "/dev/sdb", ""); err != nil {
		t.Fatal(err)
	}
	if len(f.chamadas) != 3 {
		t.Fatalf("chamadas = %q", f.chamadas)
	}
}

func TestLinuxObjetoCriado(t *testing.T) {
	if p, err := lerObjetoCriado([]byte(`{"type":"o","data":["/org/freedesktop/UDisks2/block_devices/mmcblk0p1"]}`)); err != nil || p != "/dev/mmcblk0p1" {
		t.Fatalf("= %q, %v", p, err)
	}
	if _, err := lerObjetoCriado([]byte(`{"type":"o","data":["/org/freedesktop/UDisks2/block_devices/../x"]}`)); err == nil {
		t.Fatal("devia recusar caminho estranho")
	}
}
