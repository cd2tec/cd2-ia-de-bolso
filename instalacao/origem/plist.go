package origem

import (
	"fmt"

	"howett.net/plist"
)

// volumeDePlist lê a saída de `diskutil info -plist`. DoSistema fica por
// conta de quem chama, que conhece o ponto de montagem.
func volumeDePlist(b []byte) (Volume, error) {
	var info struct {
		Internal       bool `plist:"Internal"`
		RemovableMedia bool `plist:"RemovableMedia"`
		Ejectable      bool `plist:"Ejectable"`
	}
	if _, err := plist.Unmarshal(b, &info); err != nil {
		return Volume{}, fmt.Errorf("resposta do diskutil ilegível: %w", err)
	}
	return Volume{Externo: !info.Internal, Removivel: info.RemovableMedia || info.Ejectable}, nil
}
