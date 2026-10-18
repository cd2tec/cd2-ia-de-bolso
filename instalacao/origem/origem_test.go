package origem

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRegra(t *testing.T) {
	casos := []struct {
		nome string
		v    Volume
		quer Origem
	}{
		{"disco do sistema", Volume{DoSistema: true}, Instalada},
		{"sistema vence removível", Volume{DoSistema: true, Removivel: true}, Instalada},
		{"pen drive fixo do Windows", Volume{USB: true}, Pendrive},
		{"externo", Volume{Externo: true}, Pendrive},
		{"removível", Volume{Removivel: true}, Pendrive},
		{"segundo disco interno", Volume{}, Instalada},
	}
	for _, c := range casos {
		if got := Regra(c.v); got != c.quer {
			t.Errorf("%s: Regra = %q, quer %q", c.nome, got, c.quer)
		}
	}
}

func TestNaDuvidaEPendrive(t *testing.T) {
	antes := lerVolume
	defer func() { lerVolume = antes }()
	lerVolume = func(string) (Volume, error) { return Volume{}, errors.New("falhou") }
	if got := Detectar("/qualquer"); got != Pendrive {
		t.Fatalf("Detectar = %q", got)
	}
	lerVolume = func(string) (Volume, error) { return Volume{DoSistema: true}, nil }
	if got := Detectar("/qualquer"); got != Instalada {
		t.Fatalf("Detectar = %q", got)
	}
}

func TestValoresJSON(t *testing.T) {
	if Pendrive != "pendrive" || Instalada != "instalada" || Desconhecida != "desconhecida" {
		t.Fatal("strings consumidas pela web mudaram")
	}
}

const plistExterno = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Internal</key><false/>
<key>RemovableMedia</key><false/>
<key>Ejectable</key><true/>
<key>MountPoint</key><string>/Volumes/PEN</string>
</dict></plist>`

const plistInterno = `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>Internal</key><true/>
<key>RemovableMedia</key><false/>
<key>Ejectable</key><false/>
</dict></plist>`

func TestVolumeDePlist(t *testing.T) {
	v, err := volumeDePlist([]byte(plistExterno))
	if err != nil {
		t.Fatal(err)
	}
	if !v.Externo || !v.Removivel || v.USB || v.DoSistema {
		t.Fatalf("externo: %+v", v)
	}
	v, err = volumeDePlist([]byte(plistInterno))
	if err != nil {
		t.Fatal(err)
	}
	if v.Externo || v.Removivel {
		t.Fatalf("interno: %+v", v)
	}
	if _, err := volumeDePlist([]byte("lixo")); err == nil {
		t.Fatal("aceitou plist inválido")
	}
}

func TestEhDoSistemaMac(t *testing.T) {
	for p, quer := range map[string]bool{"/": true, "/System/Volumes/Data": true, "/Volumes/PEN": false, "/Users": false} {
		if got := montagemDoSistema(p); got != quer {
			t.Errorf("%s: %v", p, got)
		}
	}
}

func TestDiskutilTravadoViraErro(t *testing.T) {
	antesE, antesT := executarDiskutil, timeoutDiskutil
	defer func() { executarDiskutil, timeoutDiskutil = antesE, antesT }()
	timeoutDiskutil = 50 * time.Millisecond
	executarDiskutil = func(ctx context.Context, _ string) ([]byte, error) {
		<-ctx.Done() // simula um diskutil que nunca responde
		return nil, ctx.Err()
	}
	inicio := time.Now()
	if _, err := volumeDoDiskutil("/Volumes/PEN"); err == nil {
		t.Fatal("travou e não deu erro")
	}
	if time.Since(inicio) > 2*time.Second {
		t.Fatal("não respeitou o timeout")
	}
	antesL := lerVolume
	defer func() { lerVolume = antesL }()
	lerVolume = func(string) (Volume, error) { return volumeDoDiskutil("/Volumes/PEN") }
	if got := Detectar("/Volumes/PEN"); got != Pendrive {
		t.Fatalf("Detectar = %q", got)
	}
}
