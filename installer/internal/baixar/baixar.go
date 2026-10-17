// Pacote baixar baixa arquivos grandes com retomada e conferência de SHA-256.
package baixar

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/manifesto"
)

// Progresso recebe os bytes já em disco e o total esperado (0 se desconhecido).
type Progresso func(feitos, total int64)

// ErrConferencia indica que o arquivo em disco não bate com o manifesto.
var ErrConferencia = errors.New("arquivo baixado não confere")

// ErrHTTP indica status HTTP inesperado no download.
var ErrHTTP = errors.New("status HTTP inesperado")

// Arquivo baixa url em destino. Pula se já estiver completo (SHA confere, ou
// Bytes bate quando não há SHA), sem chamar p. Retoma pela metade com Range; se o servidor
// ignorar o Range (200), recomeça do zero. Se a conferência falhar no fim,
// apaga e tenta UMA vez do zero; se falhar de novo, devolve ErrConferencia.
// O parâmetro url prevalece sobre esperado.URL.
func Arquivo(ctx context.Context, c *http.Client, url, destino string, esperado manifesto.Arquivo, p Progresso) error {
	if c == nil {
		c = http.DefaultClient
	}
	if esperado.SHA256 == "" && esperado.Bytes == 0 {
		// Sem SHA nem tamanho no manifesto: pergunta o tamanho por HEAD, como o
		// remote_size do core/prepare-unix.sh. Se falhar, segue sem tamanho.
		esperado.Bytes = tamanhoRemoto(ctx, c, url)
	}
	if st, err := os.Stat(destino); err == nil {
		switch {
		case esperado.Bytes > 0 && st.Size() > esperado.Bytes:
			// Maior que o esperado nunca vira o arquivo certo.
			if err := os.Remove(destino); err != nil {
				return err
			}
		case esperado.SHA256 == "" && esperado.Bytes == 0:
			// Sem como saber se está completo: baixa ou retoma.
		case esperado.Bytes == 0 || st.Size() == esperado.Bytes:
			if Conferir(destino, esperado) == nil {
				// Já completo: nada foi baixado, então não há progresso a dar.
				return nil
			}
			if esperado.Bytes > 0 {
				// Tamanho certo e conteúdo errado: outra versão, do zero.
				if err := os.Remove(destino); err != nil {
					return err
				}
			}
		}
	}

	var errConf error
	for tentativa := 0; tentativa < 2; tentativa++ {
		if err := baixar(ctx, c, url, destino, esperado, p); err != nil {
			return err
		}
		if errConf = Conferir(destino, esperado); errConf == nil {
			return nil
		}
		if err := os.Remove(destino); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return errConf
}

// tamanhoRemoto devolve o Content-Length de um HEAD, ou 0 se não souber.
func tamanhoRemoto(ctx context.Context, c *http.Client, url string) int64 {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return 0
	}
	req.Header.Set("User-Agent", "cd2-harness")
	resp, err := c.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.ContentLength <= 0 {
		return 0
	}
	return resp.ContentLength
}

// baixar faz uma passada: retoma do tamanho atual do destino, se houver.
func baixar(ctx context.Context, c *http.Client, url, destino string, esperado manifesto.Arquivo, p Progresso) error {
	var offset int64
	if st, err := os.Stat(destino); err == nil {
		offset = st.Size()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "cd2-harness")
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := c.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	defer resp.Body.Close()

	flags := os.O_WRONLY | os.O_CREATE
	switch resp.StatusCode {
	case http.StatusPartialContent:
		flags |= os.O_APPEND
	case http.StatusOK:
		// Servidor ignorou o Range (ou não havia): recomeça do zero.
		flags |= os.O_TRUNC
		offset = 0
	case http.StatusRequestedRangeNotSatisfiable:
		// O que há em disco já cobre tudo; a conferência decide.
		return nil
	default:
		return fmt.Errorf("%w: download de %s: %s", ErrHTTP, url, resp.Status)
	}

	total := esperado.Bytes
	if total == 0 && resp.ContentLength > 0 {
		total = offset + resp.ContentLength
	}
	f, err := os.OpenFile(destino, flags, 0o644)
	if err != nil {
		return err
	}
	feitos := offset
	buf := make([]byte, 256<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				return werr
			}
			feitos += int64(n)
			if p != nil {
				p(feitos, total)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return rerr
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	if p != nil && total == 0 {
		p(feitos, feitos)
	}
	return nil
}

// Conferir só confere o que já está em disco, sem rede (modo cache).
// Com SHA, confere o SHA; sem SHA, confere Bytes quando informado; sem nenhum
// dos dois, aceita qualquer arquivo não vazio.
func Conferir(destino string, esperado manifesto.Arquivo) error {
	st, err := os.Stat(destino)
	if err != nil {
		return err
	}
	if esperado.Bytes > 0 && st.Size() != esperado.Bytes {
		return fmt.Errorf("%w: %s tem %d bytes, esperado %d", ErrConferencia, destino, st.Size(), esperado.Bytes)
	}
	if esperado.SHA256 == "" {
		if esperado.Bytes == 0 && st.Size() == 0 {
			return fmt.Errorf("%w: %s está vazio", ErrConferencia, destino)
		}
		return nil
	}
	f, err := os.Open(destino)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), esperado.SHA256) {
		return fmt.Errorf("%w: SHA-256 de %s diferente do manifesto", ErrConferencia, destino)
	}
	return nil
}
