package discos

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Executor roda um comando e devolve a saída padrão. Nos testes é um
// falso: nenhum teste toca em disco de verdade.
type Executor func(ctx context.Context, nome string, args ...string) ([]byte, error)

// executorReal roda o comando de verdade. Em caso de falha o erro leva o
// que o comando escreveu em stderr, que é o que explica o problema.
func executorReal(ctx context.Context, nome string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, nome, args...)
	ajustarProcesso(cmd)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = strings.TrimSpace(out.String())
		}
		if msg != "" {
			return out.Bytes(), fmt.Errorf("%s: %w: %s", nome, err, msg)
		}
		return out.Bytes(), fmt.Errorf("%s: %w", nome, err)
	}
	return out.Bytes(), nil
}
