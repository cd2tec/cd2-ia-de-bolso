//go:build windows

package elevacao

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows"
)

// Elevado diz se o processo roda com o token de administrador.
func Elevado() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// Destacar solta o programa do grupo e do console do instalador.
func Destacar(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS}
}

// sinal é o mutex da instância reaberta. Fica aberto até o processo sair:
// é ele que diz à instância elevada que a reabertura deu certo.
var sinal windows.Handle

// Sinalizar cria o mutex nomeado do nonce. Roda na instância reaberta, sem
// administrador, para a elevada conseguir abri-lo.
func Sinalizar(nonce string) error {
	if !sinalValido(nonce) {
		return errors.New("sinal com formato errado")
	}
	nome, err := windows.UTF16PtrFromString(nomeSinal(nonce))
	if err != nil {
		return err
	}
	h, err := windows.CreateMutex(nil, false, nome)
	if err != nil && h == 0 {
		return fmt.Errorf("CreateMutex: %w", err)
	}
	sinal = h
	return nil
}

// EsperarSinal procura o mutex do nonce a cada 100 ms, até o prazo. Acesso
// negado também conta: o objeto existe, só foi criado por outra conta.
func EsperarSinal(nonce string, prazo time.Duration) bool {
	nome, err := windows.UTF16PtrFromString(nomeSinal(nonce))
	if err != nil {
		return false
	}
	fim := time.Now().Add(prazo)
	for {
		h, err := windows.OpenMutex(windows.SYNCHRONIZE, false, nome)
		if err == nil {
			windows.CloseHandle(h)
			return true
		}
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return true
		}
		if time.Now().After(fim) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// GUIDs da técnica "pedir ao Explorer que abra o programa": a janela do
// desktop em ShellWindows, o navegador dela, a visão ativa e o objeto Shell
// por trás dela, que roda como o usuário logado e sem administrador.
var (
	clsidShellWindows       = ole.NewGUID("{9BA05972-F6A8-11CF-A442-00A0C90A8F39}")
	iidIShellWindows        = ole.NewGUID("{85CB6900-4D95-11CF-960C-0080C7F4EE85}")
	iidIServiceProvider     = ole.NewGUID("{6D5140C1-7436-11CE-8034-00AA006009FA}")
	sidSTopLevelBrowser     = ole.NewGUID("{4C96BE40-915C-11CF-99D3-00AA004AE837}")
	iidIShellBrowser        = ole.NewGUID("{000214E2-0000-0000-C000-000000000046}")
	errExplorerNaoEncontrou = errors.New("o Explorer não está rodando (não achei a janela do desktop)")
)

// Posições na vtable (IUnknown tem 3 métodos, IDispatch mais 4, IOleWindow mais 2).
const (
	vtFindWindowSW         = 15 // IShellWindows
	vtQueryService         = 3  // IServiceProvider
	vtQueryActiveShellView = 15 // IShellBrowser
	vtGetItemObject        = 15 // IShellView

	csidlDesktop     = 0
	swcDesktop       = 8
	swfoNeedDispatch = 1
	svgioBackground  = 0
	swHide           = 0
	sFalse           = 1
)

// ReabrirSemAdmin pede ao Explorer, que roda como o usuário logado e sem
// administrador, que abra exe com args. Tudo roda numa thread só dela, com o
// COM iniciado e terminado ali.
func ReabrirSemAdmin(exe string, args []string) error {
	params, err := parametrosSemAdmin(exe, args)
	if err != nil {
		return err
	}
	cmd := os.Getenv("ComSpec")
	if cmd == "" {
		dir, err := windows.GetSystemDirectory()
		if err != nil {
			return fmt.Errorf("pasta do sistema: %w", err)
		}
		cmd = filepath.Join(dir, "cmd.exe")
	}
	pronto := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		pronto <- shellExecuteComoUsuario(cmd, params, filepath.Dir(exe))
	}()
	return <-pronto
}

func shellExecuteComoUsuario(arquivo, params, dir string) error {
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		var oe *ole.OleError
		if !errors.As(err, &oe) || oe.Code() != sFalse {
			return fmt.Errorf("CoInitializeEx: %w", err)
		}
	}
	defer ole.CoUninitialize()

	unk, err := ole.CreateInstance(clsidShellWindows, iidIShellWindows)
	if err != nil {
		return fmt.Errorf("ShellWindows: %w", err)
	}
	defer unk.Release()

	// FindWindowSW(CSIDL_DESKTOP, vazio, SWC_DESKTOP, &hwnd, SWFO_NEEDDISPATCH, &disp)
	loc := ole.NewVariant(ole.VT_I4, csidlDesktop)
	var raiz ole.VARIANT
	ole.VariantInit(&raiz)
	var hwnd int32
	var desktop *ole.IDispatch
	hr, _, _ := syscall.SyscallN(metodo(unk, vtFindWindowSW), uintptr(unsafe.Pointer(unk)),
		uintptr(unsafe.Pointer(&loc)),
		uintptr(unsafe.Pointer(&raiz)),
		swcDesktop,
		uintptr(unsafe.Pointer(&hwnd)),
		swfoNeedDispatch,
		uintptr(unsafe.Pointer(&desktop)))
	if hr == sFalse || (hr == 0 && desktop == nil) {
		return errExplorerNaoEncontrou
	}
	if hr != 0 {
		return fmt.Errorf("FindWindowSW: %w", ole.NewError(hr))
	}
	defer desktop.Release()

	sp, err := desktop.QueryInterface(iidIServiceProvider)
	if err != nil {
		return fmt.Errorf("IServiceProvider: %w", err)
	}
	defer sp.Release()

	var browser *ole.IUnknown
	hr, _, _ = syscall.SyscallN(metodo(&sp.IUnknown, vtQueryService), uintptr(unsafe.Pointer(sp)),
		uintptr(unsafe.Pointer(sidSTopLevelBrowser)),
		uintptr(unsafe.Pointer(iidIShellBrowser)),
		uintptr(unsafe.Pointer(&browser)))
	if hr != 0 || browser == nil {
		return fmt.Errorf("QueryService(IShellBrowser): %w", ole.NewError(hr))
	}
	defer browser.Release()

	var view *ole.IUnknown
	hr, _, _ = syscall.SyscallN(metodo(browser, vtQueryActiveShellView), uintptr(unsafe.Pointer(browser)),
		uintptr(unsafe.Pointer(&view)))
	if hr != 0 || view == nil {
		return fmt.Errorf("QueryActiveShellView: %w", ole.NewError(hr))
	}
	defer view.Release()

	var fundo *ole.IDispatch
	hr, _, _ = syscall.SyscallN(metodo(view, vtGetItemObject), uintptr(unsafe.Pointer(view)),
		svgioBackground,
		uintptr(unsafe.Pointer(ole.IID_IDispatch)),
		uintptr(unsafe.Pointer(&fundo)))
	if hr != 0 || fundo == nil {
		return fmt.Errorf("GetItemObject: %w", ole.NewError(hr))
	}
	defer fundo.Release()

	app, err := oleutil.GetProperty(fundo, "Application")
	if err != nil {
		return fmt.Errorf("Application: %w", err)
	}
	defer app.Clear()
	shell := app.ToIDispatch()
	if shell == nil {
		return errors.New("o Explorer não devolveu o objeto Shell")
	}

	// IShellDispatch2.ShellExecute(arquivo, parâmetros, pasta, verbo, janela).
	// O cmd roda escondido; o "start" abre o instalador com janela normal.
	r, err := oleutil.CallMethod(shell, "ShellExecute", arquivo, params, dir, "open", swHide)
	if err != nil {
		return fmt.Errorf("ShellExecute: %w", err)
	}
	r.Clear()
	return nil
}

// metodo devolve o endereço do método idx na vtable de obj. A chamada em
// si fica no syscall.SyscallN de cada lugar, que mantém vivos os ponteiros
// convertidos para uintptr na própria lista de argumentos.
func metodo(obj *ole.IUnknown, idx int) uintptr {
	vtbl := (*[32]uintptr)(unsafe.Pointer(obj.RawVTable))
	return vtbl[idx]
}
