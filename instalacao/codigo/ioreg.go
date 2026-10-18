package codigo

import (
	"errors"
	"regexp"
)

var reUUID = regexp.MustCompile(`"IOPlatformUUID"\s*=\s*"([^"]+)"`)

// ExtrairUUIDIoreg acha o IOPlatformUUID na saída de
// `ioreg -rd1 -c IOPlatformExpertDevice`.
func ExtrairUUIDIoreg(saida string) (string, error) {
	m := reUUID.FindStringSubmatch(saida)
	if m == nil {
		return "", errors.New("IOPlatformUUID não encontrado na saída do ioreg")
	}
	return m[1], nil
}
