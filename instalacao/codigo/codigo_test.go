package codigo_test

import (
	"regexp"
	"testing"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/codigo"
)

func TestDerivarFormato(t *testing.T) {
	c := codigo.Derivar("  4C4C4544-0042-3510-8051-B4C04F4B4E32 ")
	if !regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{4}-[0-9A-HJKMNP-TV-Z]{4}-[0-9A-HJKMNP-TV-Z]{4}$`).MatchString(c) {
		t.Fatal(c)
	}
	if c != codigo.Derivar("4c4c4544-0042-3510-8051-b4c04f4b4e32") {
		t.Fatal("maiúsculas e espaços mudaram o código")
	}
}

func TestExtrairUUIDDoIoreg(t *testing.T) {
	saida := `+-o Root  <class IORegistryEntry, id 0x100000100, retain 36>
  +-o J314sAP  <class IOPlatformExpertDevice, id 0x100000225, registered, matched, active, busy 0 (1234 ms), retain 40>
    {
      "IOPlatformSerialNumber" = "ABCDEF1234"
      "IOPlatformUUID" = "4C4C4544-0042-3510-8051-B4C04F4B4E32"
      "manufacturer" = <"Apple Inc.">
    }
`
	got, err := codigo.ExtrairUUIDIoreg(saida)
	if err != nil || got != "4C4C4544-0042-3510-8051-B4C04F4B4E32" {
		t.Fatal(got, err)
	}
	if _, err := codigo.ExtrairUUIDIoreg("nada aqui"); err == nil {
		t.Fatal("deveria falhar sem IOPlatformUUID")
	}
}
