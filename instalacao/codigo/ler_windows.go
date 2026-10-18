//go:build windows

package codigo

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// Ler devolve o código deste PC, derivado do MachineGuid do Windows.
func Ler() (string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return "", fmt.Errorf("não consegui abrir o registro do Windows: %w", err)
	}
	defer k.Close()
	guid, _, err := k.GetStringValue("MachineGuid")
	if err != nil || guid == "" {
		return "", fmt.Errorf("não consegui ler a identidade do PC: %v", err)
	}
	return Derivar(guid), nil
}
