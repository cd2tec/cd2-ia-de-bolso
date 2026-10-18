//go:build windows

package sistema

import (
	"errors"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

// registroDoSistema grava em HKEY_CURRENT_USER de verdade.
type registroDoSistema struct{}

func (registroDoSistema) abrir(chave string) (registry.Key, error) {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, chave, registry.SET_VALUE|registry.QUERY_VALUE)
	return k, err
}

func (r registroDoSistema) GravarTexto(chave, nome, valor string) error {
	k, err := r.abrir(chave)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(nome, valor)
}

func (r registroDoSistema) GravarNumero(chave, nome string, valor uint32) error {
	k, err := r.abrir(chave)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetDWordValue(nome, valor)
}

func (registroDoSistema) Existe(chave, nome string) bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, chave, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetValue(nome, nil)
	return err == nil
}

func (registroDoSistema) ApagarValor(chave, nome string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, chave, registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.DeleteValue(nome); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}

func (registroDoSistema) ApagarChave(chave string) error {
	if err := registry.DeleteKey(registry.CURRENT_USER, chave); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}

// esconderJanela evita a janela preta do PowerShell piscar na tela.
func esconderJanela(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
