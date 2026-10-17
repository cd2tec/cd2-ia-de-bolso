// Package modelo localiza o modelo de IA no pen drive e o lê inteiro uma vez,
// para esquentar o cache de arquivos do sistema e dar um progresso real.
package modelo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// Bloco é o tamanho de cada leitura: 32 MiB, como o CD2_ROBO_BLOCO_MB de core/robo-unix.sh.
const Bloco int64 = 32 << 20

// ErrSemModelo indica que não há nenhum .gguf em models/.
var ErrSemModelo = errors.New("nenhum modelo .gguf em models/")

// Achar devolve o primeiro models/*.gguf da raiz, em ordem alfabética.
func Achar(raiz string) (string, error) {
	achados, err := filepath.Glob(filepath.Join(raiz, "models", "*.gguf"))
	if err != nil {
		return "", err
	}
	if len(achados) == 0 {
		return "", ErrSemModelo
	}
	sort.Strings(achados)
	return achados[0], nil
}

// Ler lê o arquivo inteiro em blocos e chama prog depois de cada um, com a
// última chamada em feitos == total. Se o ctx for cancelado, para e devolve
// o erro dele sem anunciar o fim.
func Ler(ctx context.Context, caminho string, bloco int64, prog func(feitos, total int64)) error {
	if bloco <= 0 {
		bloco = Bloco
	}
	f, err := os.Open(caminho)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	total := st.Size()
	// Não aloca 32 MiB para ler um arquivo pequeno.
	tam := bloco
	if total < tam {
		tam = total
	}
	buf := make([]byte, tam)
	var feitos int64
	for feitos < total {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := io.ReadFull(f, buf)
		feitos += int64(n)
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return err
		}
		if n == 0 {
			return fmt.Errorf("o arquivo encolheu durante a leitura: %s", caminho)
		}
		if prog != nil {
			prog(feitos, total)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if total == 0 && prog != nil {
		prog(0, 0)
	}
	return nil
}
