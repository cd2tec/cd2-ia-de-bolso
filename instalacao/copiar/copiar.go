// Package copiar instala a IA do pen drive no disco do computador: monta o
// plano do que copiar, copia para uma pasta ".novo" conferindo o SHA-256 e só
// no fim troca pela instalação anterior.
package copiar

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// ErrCopiaRuim indica que um arquivo copiado não bate com o SHA-256 esperado.
var ErrCopiaRuim = errors.New("Não consegui copiar direito. Tente de novo.")

// ErrSemEspaco indica que o disco de destino não comporta a instalação.
var ErrSemEspaco = errors.New("Falta espaço no computador. Libere uns 3 GB e tente de novo.")

// Item é um arquivo a copiar. Destino é relativo à pasta de instalação, com
// barras normais. SHA256 fica vazio para quem não está no CHECKSUMS.sha256.
type Item struct {
	Origem, Destino string
	Tamanho         int64
	SHA256          string
}

// espacoLivre fica atrás de uma variável para o teste forçar pouco espaço.
var espacoLivre = EspacoLivre

const tamanhoPedaco = 1 << 20

// plataforma devolve o nome da pasta de binários, igual ao ctl.Caminho do
// lançador: no Windows sempre windows-amd64; no Mac, pelo goarch.
func plataforma(goos, goarch string) string {
	if goos == "windows" {
		return "windows-amd64"
	}
	if goarch == "arm64" {
		return "darwin-arm64"
	}
	return "darwin-amd64"
}

// lerChecksums lê o CHECKSUMS.sha256 ("<sha>  <caminho>") para um mapa.
func lerChecksums(raiz string) (map[string]string, error) {
	f, err := os.Open(filepath.Join(raiz, "CHECKSUMS.sha256"))
	if err != nil {
		return nil, fmt.Errorf("não achei o CHECKSUMS.sha256 no pen drive: %w", err)
	}
	defer f.Close()
	m := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if len(l) < 66 {
			continue
		}
		sha, rel := strings.ToLower(l[:64]), strings.TrimSpace(l[64:])
		rel = strings.TrimPrefix(rel, "*")
		m[rel] = sha
	}
	return m, sc.Err()
}

// Planejar lista o que vai para o computador e o tamanho total.
func Planejar(raiz, goos, goarch string) ([]Item, int64, error) {
	sums, err := lerChecksums(raiz)
	if err != nil {
		return nil, 0, err
	}
	plat := plataforma(goos, goarch)
	lancador := "IA de Bolso.app"
	if goos == "windows" {
		lancador = "IA de Bolso.exe"
	}
	var itens []Item
	var total int64

	add := func(rel string) error {
		fi, err := os.Stat(filepath.Join(raiz, filepath.FromSlash(rel)))
		if err != nil {
			return fmt.Errorf("falta %s no pen drive: %w", rel, err)
		}
		itens = append(itens, Item{Origem: filepath.Join(raiz, filepath.FromSlash(rel)), Destino: rel, Tamanho: fi.Size(), SHA256: sums[rel]})
		total += fi.Size()
		return nil
	}
	andar := func(rel string) error {
		base := filepath.Join(raiz, filepath.FromSlash(rel))
		if fi, err := os.Stat(base); err != nil || !fi.IsDir() {
			return fmt.Errorf("falta a pasta %s no pen drive", rel)
		}
		var achados []string
		err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			n := d.Name()
			if strings.HasPrefix(n, "._") || n == ".DS_Store" {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}
			r, err := filepath.Rel(raiz, p)
			if err != nil {
				return err
			}
			achados = append(achados, filepath.ToSlash(r))
			return nil
		})
		if err != nil {
			return err
		}
		if len(achados) == 0 {
			return fmt.Errorf("a pasta %s está vazia no pen drive", rel)
		}
		for _, r := range achados {
			if err := add(r); err != nil {
				return err
			}
		}
		return nil
	}

	for _, rel := range []string{"VERSION", "LICENSE", "bin/llamafile"} {
		if err := add(rel); err != nil {
			return nil, 0, err
		}
	}
	modelos, _ := filepath.Glob(filepath.Join(raiz, "models", "*.gguf"))
	if len(modelos) == 0 {
		return nil, 0, errors.New("falta o modelo (.gguf) no pen drive")
	}
	sort.Strings(modelos)
	for _, m := range modelos {
		if err := add(path.Join("models", filepath.Base(m))); err != nil {
			return nil, 0, err
		}
	}
	for _, d := range []string{"web", "core", "bin/toolbox/" + plat, "bin/ctl/" + plat} {
		if err := andar(d); err != nil {
			return nil, 0, err
		}
	}
	if goos == "windows" {
		err = add(lancador)
	} else {
		err = andar(lancador)
	}
	if err != nil {
		return nil, 0, err
	}
	return itens, total, nil
}

// pedirEspaco devolve o espaço livre do disco que vai receber destino. Se a
// pasta pai ainda não existe, sobe até a primeira que existe.
func pedirEspaco(destino string) (int64, error) {
	dir := filepath.Dir(filepath.Clean(destino))
	for {
		if _, err := os.Stat(dir); err == nil {
			return espacoLivre(dir)
		}
		pai := filepath.Dir(dir)
		if pai == dir {
			return espacoLivre(dir)
		}
		dir = pai
	}
}

// Copiar instala itens em destino. Copia para destino+".novo", confere o
// SHA-256 e só no fim troca. Em erro, apaga o .novo e deixa destino intacto.
func Copiar(ctx context.Context, itens []Item, destino string, prog func(feitos, total int64)) (err error) {
	var total int64
	for _, it := range itens {
		total += it.Tamanho
	}
	livre, err := pedirEspaco(destino)
	if err != nil {
		return fmt.Errorf("não consegui ver o espaço livre: %w", err)
	}
	if livre < total+total/10 {
		return ErrSemEspaco
	}

	novo, velho := destino+".novo", destino+".velho"
	if err := os.RemoveAll(novo); err != nil {
		return err
	}
	if err := os.RemoveAll(velho); err != nil {
		return err
	}
	if err := os.MkdirAll(novo, 0o755); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(novo)
		}
	}()

	var feitos int64
	avisar := func() {
		if prog != nil {
			prog(feitos, total)
		}
	}
	for _, it := range itens {
		if err = ctx.Err(); err != nil {
			return err
		}
		if err = copiarUm(ctx, it, filepath.Join(novo, filepath.FromSlash(it.Destino)), &feitos, avisar); err != nil {
			return err
		}
	}
	avisar()

	if _, e := os.Stat(destino); e == nil {
		if err = os.Rename(destino, velho); err != nil {
			return err
		}
		if err = os.Rename(novo, destino); err != nil {
			os.Rename(velho, destino)
			return err
		}
		os.RemoveAll(velho)
		return nil
	}
	return os.Rename(novo, destino)
}

func copiarUm(ctx context.Context, it Item, alvo string, feitos *int64, avisar func()) error {
	src, err := os.Open(it.Origem)
	if err != nil {
		return err
	}
	defer src.Close()
	fi, err := src.Stat()
	if err != nil {
		return err
	}
	modo := os.FileMode(0o644)
	if fi.Mode()&0o111 != 0 {
		modo = 0o755
	}
	if err := os.MkdirAll(filepath.Dir(alvo), 0o755); err != nil {
		return err
	}
	dst, err := os.OpenFile(alvo, os.O_CREATE|os.O_EXCL|os.O_WRONLY, modo)
	if err != nil {
		return err
	}
	h := sha256.New()
	buf := make([]byte, tamanhoPedaco)
	for {
		if err := ctx.Err(); err != nil {
			dst.Close()
			return err
		}
		n, rerr := src.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
			if _, werr := dst.Write(buf[:n]); werr != nil {
				dst.Close()
				return werr
			}
			*feitos += int64(n)
			avisar()
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			dst.Close()
			return rerr
		}
	}
	if err := dst.Close(); err != nil {
		return err
	}
	// chmod depois do Close: o umask pode ter tirado o bit na criação.
	if err := os.Chmod(alvo, modo); err != nil {
		return err
	}
	if it.SHA256 != "" && !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), it.SHA256) {
		return ErrCopiaRuim
	}
	return nil
}
