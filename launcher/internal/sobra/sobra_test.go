package sobra

import (
	"errors"
	"testing"
	"time"
)

func TestDoPenDrive(t *testing.T) {
	alvo := `E:\bin\llamafile.exe`
	casos := []struct {
		exe  string
		quer bool
	}{
		{`E:\bin\llamafile.exe`, true},
		{`e:\BIN\LlamaFile.EXE`, true},
		{`E:/bin/llamafile.exe`, true},
		{`\\?\E:\bin\llamafile.exe`, true},
		{`F:\bin\llamafile.exe`, false},     // outro pen drive
		{`C:\Users\x\llamafile.exe`, false}, // outro llamafile
		{`E:\bin\llamafile.exe.bak`, false}, // só o começo igual não basta
		{`E:\outra\bin\llamafile.exe`, false},
		{``, false}, // sem caminho: não arrisca
	}
	for _, c := range casos {
		if got := DoPenDrive(alvo, c.exe); got != c.quer {
			t.Errorf("DoPenDrive(%q, %q) = %v, quer %v", alvo, c.exe, got, c.quer)
		}
	}
	if DoPenDrive("", "") {
		t.Error("alvo vazio casou")
	}
}

// falsos troca os efeitos e devolve os PIDs mortos.
func falsos(t *testing.T, procs []Processo, errLista error, presoAte int) *[]uint32 {
	t.Helper()
	l, m, a, tent, p := listarFn, matarFn, abrirFn, Tentativas, Pausa
	t.Cleanup(func() { listarFn, matarFn, abrirFn, Tentativas, Pausa = l, m, a, tent, p })
	Tentativas, Pausa = 5, time.Millisecond
	mortos := &[]uint32{}
	listarFn = func() ([]Processo, error) { return procs, errLista }
	matarFn = func(pid uint32) error { *mortos = append(*mortos, pid); return nil }
	aberturas := 0
	abrirFn = func(string) error {
		aberturas++
		if aberturas <= presoAte {
			return errors.New("em uso")
		}
		return nil
	}
	return mortos
}

func TestEncerrarSoODoPenDrive(t *testing.T) {
	mortos := falsos(t, []Processo{
		{PID: 10, Exe: `C:\outro\llamafile.exe`},
		{PID: 11, Exe: `E:\bin\llamafile.exe`},
		{PID: 12, Exe: ``},
	}, nil, 2)
	if err := Encerrar(`E:\bin\llamafile.exe`, []string{"a.log", "b.log"}); err != nil {
		t.Fatalf("Encerrar: %v", err)
	}
	if len(*mortos) != 1 || (*mortos)[0] != 11 {
		t.Fatalf("mortos = %v, quer [11]", *mortos)
	}
}

func TestEncerrarSemSobraNaoEspera(t *testing.T) {
	mortos := falsos(t, []Processo{{PID: 10, Exe: `C:\outro\llamafile.exe`}}, nil, 1000)
	if err := Encerrar(`E:\bin\llamafile.exe`, []string{"a.log"}); err != nil {
		t.Fatalf("Encerrar: %v", err)
	}
	if len(*mortos) != 0 {
		t.Fatalf("mortos = %v, quer nenhum", *mortos)
	}
}

func TestEncerrarLogPreso(t *testing.T) {
	falsos(t, []Processo{{PID: 11, Exe: `E:\bin\llamafile.exe`}}, nil, 1000)
	if err := Encerrar(`E:\bin\llamafile.exe`, []string{"a.log"}); !errors.Is(err, ErrAindaFechando) {
		t.Fatalf("err = %v, quer ErrAindaFechando", err)
	}
}

func TestEncerrarSemListaSegue(t *testing.T) {
	mortos := falsos(t, nil, errors.New("sem acesso"), 0)
	if err := Encerrar(`E:\bin\llamafile.exe`, nil); err != nil {
		t.Fatalf("Encerrar: %v", err)
	}
	if len(*mortos) != 0 {
		t.Fatalf("mortos = %v", *mortos)
	}
}
