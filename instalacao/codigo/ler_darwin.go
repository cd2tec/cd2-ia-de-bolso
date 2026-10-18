//go:build darwin

package codigo

import (
	"fmt"
	"os/exec"
)

// Ler devolve o código deste Mac, derivado do IOPlatformUUID.
func Ler() (string, error) {
	saida, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err != nil {
		return "", fmt.Errorf("não consegui ler a identidade do Mac: %w", err)
	}
	id, err := ExtrairUUIDIoreg(string(saida))
	if err != nil {
		return "", err
	}
	return Derivar(id), nil
}
