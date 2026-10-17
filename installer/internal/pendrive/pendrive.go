// Package pendrive copia o stage para o pen drive montado e confere os
// checksums direto nele, como as etapas 3 e 4 de core/make-usb-unix.sh.
package pendrive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/preparo"
)

var ErrRemovido = errors.New("pen drive removido")
var ErrDiferente = errors.New("checksum diferente no pen drive")

const bloco = 1 << 20

func lixo(nome string) bool {
	return strings.HasPrefix(nome, "._") || nome == ".DS_Store"
}

// sumiu diz se a pasta de destino deixou de existir (pen drive arrancado).
func sumiu(destino string) bool {
	_, err := os.Stat(destino)
	return err != nil
}

// erroDeDestino troca um erro de E/S por ErrRemovido quando o destino sumiu.
func erroDeDestino(destino string, err error) error {
	if sumiu(destino) {
		return fmt.Errorf("%w: %v", ErrRemovido, err)
	}
	return err
}

// TamanhoTotal soma os bytes dos arquivos regulares do stage que Copiar copia.
func TamanhoTotal(stage string) (int64, error) {
	var total int64
	err := filepath.WalkDir(stage, func(c string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if lixo(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}

func progresso(p func(feitos, total int64), feitos, total int64) {
	if p != nil {
		p(feitos, total)
	}
}

// Copiar copia o stage para a raiz de destino, com Sync em cada arquivo, e no
// fim apaga ._* e .DS_Store que existirem no destino.
func Copiar(ctx context.Context, stage, destino string, p func(feitos, total int64)) error {
	total, err := TamanhoTotal(stage)
	if err != nil {
		return err
	}
	var feitos int64
	err = filepath.WalkDir(stage, func(c string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if lixo(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(stage, c)
		if err != nil {
			return err
		}
		alvo := filepath.Join(destino, rel)
		if d.IsDir() {
			if err := os.MkdirAll(alvo, 0o755); err != nil {
				return erroDeDestino(destino, err)
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		return copiarArquivo(ctx, c, alvo, destino, &feitos, total, p)
	})
	if err != nil {
		return err
	}
	return limparLixo(destino)
}

func copiarArquivo(ctx context.Context, origem, alvo, destino string, feitos *int64, total int64, p func(int64, int64)) error {
	info, err := os.Stat(origem)
	if err != nil {
		return err
	}
	in, err := os.Open(origem)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(alvo, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return erroDeDestino(destino, err)
	}
	defer out.Close()
	buf := make([]byte, bloco)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return erroDeDestino(destino, werr)
			}
			*feitos += int64(n)
			progresso(p, *feitos, total)
			if sumiu(destino) {
				return ErrRemovido
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	if err := out.Sync(); err != nil {
		return erroDeDestino(destino, err)
	}
	if err := out.Close(); err != nil {
		return erroDeDestino(destino, err)
	}
	if sumiu(destino) {
		return ErrRemovido
	}
	return nil
}

func limparLixo(destino string) error {
	var alvos []string
	err := filepath.WalkDir(destino, func(c string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if lixo(d.Name()) {
			alvos = append(alvos, c)
			if d.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
	if err != nil {
		return erroDeDestino(destino, err)
	}
	for _, c := range alvos {
		if err := os.RemoveAll(c); err != nil {
			return erroDeDestino(destino, err)
		}
	}
	return nil
}

// Conferir lê destino/CHECKSUMS.sha256 e confere o SHA-256 de cada arquivo
// listado, lendo direto do pen drive.
func Conferir(ctx context.Context, destino string, p func(feitos, total int64)) error {
	linhas, err := preparo.LerChecksums(filepath.Join(destino, "CHECKSUMS.sha256"))
	if err != nil {
		if sumiu(destino) {
			return fmt.Errorf("%w: %v", ErrRemovido, err)
		}
		return fmt.Errorf("%w: CHECKSUMS.sha256 ilegível: %v", ErrDiferente, err)
	}
	var total int64
	for _, l := range linhas {
		if info, err := os.Stat(filepath.Join(destino, filepath.FromSlash(l.Caminho))); err == nil {
			total += info.Size()
		}
	}
	var feitos int64
	for _, l := range linhas {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := conferirArquivo(ctx, destino, l, &feitos, total, p); err != nil {
			return err
		}
	}
	return nil
}

func conferirArquivo(ctx context.Context, destino string, l preparo.Linha, feitos *int64, total int64, p func(int64, int64)) error {
	f, err := os.Open(filepath.Join(destino, filepath.FromSlash(l.Caminho)))
	if err != nil {
		if sumiu(destino) {
			return fmt.Errorf("%w: %v", ErrRemovido, err)
		}
		return fmt.Errorf("%w: %s não pôde ser lido (%v)", ErrDiferente, l.Caminho, err)
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, bloco)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, rerr := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
			*feitos += int64(n)
			progresso(p, *feitos, total)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return erroDeDestino(destino, rerr)
		}
	}
	if hex.EncodeToString(h.Sum(nil)) != l.SHA256 {
		return fmt.Errorf("%w: %s", ErrDiferente, l.Caminho)
	}
	return nil
}

// GuardarChecksums copia destino/CHECKSUMS.sha256 para
// <cache>/CHECKSUMS-AAAAMMDD-HHMMSS.sha256 e devolve o caminho.
func GuardarChecksums(destino, cache string, agora time.Time) (string, error) {
	dados, err := os.ReadFile(filepath.Join(destino, "CHECKSUMS.sha256"))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return "", err
	}
	alvo := filepath.Join(cache, "CHECKSUMS-"+agora.Format("20060102-150405")+".sha256")
	if err := os.WriteFile(alvo, dados, 0o644); err != nil {
		return "", err
	}
	return alvo, nil
}
