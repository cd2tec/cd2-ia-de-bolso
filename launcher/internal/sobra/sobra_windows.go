package sobra

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const criarSemJanela = 0x08000000 // CREATE_NO_WINDOW

// esperaSaida é quanto o matar espera o processo sair, em milissegundos.
const esperaSaida = 5000

// listar devolve os llamafile.exe rodando, com o caminho de cada um.
func listar() ([]Processo, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	var lista []Processo
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		if !strings.EqualFold(windows.UTF16ToString(e.ExeFile[:]), "llamafile.exe") {
			continue
		}
		lista = append(lista, Processo{PID: e.ProcessID, Exe: caminhoDe(e.ProcessID)})
	}
	return lista, nil
}

// caminhoDe lê o caminho completo do executável; vazio se não der.
func caminhoDe(pid uint32) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}

// matar derruba a árvore do processo com o taskkill do System32, como o
// script e o proc, e espera ele sair.
func matar(pid uint32) error {
	h, _ := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
	if h != 0 {
		defer windows.CloseHandle(h)
	}
	exe := "taskkill"
	if raiz := os.Getenv("SystemRoot"); raiz != "" {
		exe = filepath.Join(raiz, "System32", "taskkill.exe")
	}
	k := exec.Command(exe, "/PID", strconv.FormatUint(uint64(pid), 10), "/T", "/F")
	k.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: criarSemJanela}
	err := k.Run()
	if h != 0 {
		_, _ = windows.WaitForSingleObject(h, esperaSaida)
	}
	return err
}
