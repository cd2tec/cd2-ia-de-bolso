//go:build linux

package codigo

import (
	"errors"
	"os"
	"strings"
)

// arquivosMachineID são onde o Linux guarda a identidade da máquina: o do
// systemd e, em sistemas mais antigos, o do D-Bus. Em variável para o teste.
var arquivosMachineID = []string{"/etc/machine-id", "/var/lib/dbus/machine-id"}

// Ler devolve o código deste computador, derivado do machine-id.
func Ler() (string, error) {
	for _, arq := range arquivosMachineID {
		b, err := os.ReadFile(arq)
		if err != nil {
			continue
		}
		if id, ok := MachineIDValido(string(b)); ok {
			return Derivar(id), nil
		}
	}
	return "", errors.New("não consegui ler a identidade do computador (/etc/machine-id)")
}

// MachineIDValido confere o formato do machine-id: 32 dígitos hexadecimais.
// Vazio ou "uninitialized" (imagem ainda não iniciada) não vale.
func MachineIDValido(s string) (string, bool) {
	id := strings.ToLower(strings.TrimSpace(s))
	if len(id) != 32 {
		return "", false
	}
	for _, c := range id {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return "", false
		}
	}
	return id, true
}
