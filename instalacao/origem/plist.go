package origem

import (
	"context"
	"fmt"
	"os/exec"
	"time"

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

// timeoutDiskutil limita a espera: um diskutil travado (USB instável) não
// pode prender Detectar; o erro vira Pendrive.
var timeoutDiskutil = 5 * time.Second

// executarDiskutil fica atrás de uma variável para o teste trocar.
var executarDiskutil = func(ctx context.Context, ponto string) ([]byte, error) {
	return exec.CommandContext(ctx, "diskutil", "info", "-plist", ponto).Output()
}

func volumeDoDiskutil(ponto string) (Volume, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDiskutil)
	defer cancel()
	saida, err := executarDiskutil(ctx, ponto)
	if err != nil {
		return Volume{}, fmt.Errorf("diskutil falhou: %w", err)
	}
	return volumeDePlist(saida)
}
