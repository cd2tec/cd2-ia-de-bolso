// Package cache cuida das pastas de cache e de logs e da cópia do motor
// para o cache (no Mac, o motor roda de lá, não do pen drive).
package cache

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
)

const nome = "cd2-harness"

// Pasta devolve a pasta de cache da plataforma. O caminho é montado com o
// separador do goos informado, não o do sistema em que o código roda, para
// os testes funcionarem em qualquer máquina.
func Pasta(goos string, env func(string) string, home string) string {
	switch goos {
	case "windows":
		return env("LOCALAPPDATA") + `\` + nome
	case "darwin":
		return home + "/Library/Caches/" + nome
	}
	// Linux: o lançador não roda aqui, mas o caminho padrão XDG é barato.
	base := ""
	if env != nil {
		base = env("XDG_CACHE_HOME")
	}
	if base == "" {
		base = home + "/.cache"
	}
	return base + "/" + nome
}

// PastaLogs devolve %TEMP%\cd2-harness no Windows; no Mac, a pasta do cache.
func PastaLogs(goos string, env func(string) string, home string) string {
	if goos == "windows" {
		return env("TEMP") + `\` + nome
	}
	return Pasta(goos, env, home)
}

// Preparar copia origem para destino só se o destino não existir ou for
// diferente byte a byte. Copia para destino+".tmp", ajusta o modo e renomeia,
// para nunca deixar um motor pela metade no lugar do bom.
func Preparar(origem, destino string) (copiou bool, err error) {
	igual, err := iguais(origem, destino)
	if err != nil {
		return false, err
	}
	if igual {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
		return false, err
	}
	tmp := destino + ".tmp"
	if err := copiar(origem, tmp); err != nil {
		os.Remove(tmp)
		return false, err
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		os.Remove(tmp)
		return false, err
	}
	tirarQuarentena(tmp)
	if err := os.Rename(tmp, destino); err != nil {
		os.Remove(tmp)
		return false, err
	}
	return true, nil
}

func copiar(origem, destino string) error {
	in, err := os.Open(origem)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destino, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// iguais compara primeiro o tamanho e só depois o conteúdo em blocos,
// como o cmp -s do script. Destino inexistente conta como diferente.
func iguais(a, b string) (bool, error) {
	fa, err := os.Open(a)
	if err != nil {
		return false, err
	}
	defer fa.Close()
	fb, err := os.Open(b)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer fb.Close()
	sa, err := fa.Stat()
	if err != nil {
		return false, err
	}
	sb, err := fb.Stat()
	if err != nil {
		return false, err
	}
	if sa.Size() != sb.Size() {
		return false, nil
	}
	ba := make([]byte, 1<<20)
	bb := make([]byte, 1<<20)
	for {
		na, ea := io.ReadFull(fa, ba)
		nb, eb := io.ReadFull(fb, bb)
		if na != nb || !bytes.Equal(ba[:na], bb[:nb]) {
			return false, nil
		}
		if ea != nil || eb != nil {
			if (ea == io.EOF || ea == io.ErrUnexpectedEOF) && (eb == io.EOF || eb == io.ErrUnexpectedEOF) {
				return true, nil
			}
			if ea != nil && ea != io.EOF && ea != io.ErrUnexpectedEOF {
				return false, ea
			}
			if eb != nil && eb != io.EOF && eb != io.ErrUnexpectedEOF {
				return false, eb
			}
			return false, nil
		}
	}
}
