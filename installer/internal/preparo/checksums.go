package preparo

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Linha é uma linha de um arquivo no formato do sha256sum.
type Linha struct{ SHA256, Caminho string }

// LerChecksums lê "<sha>  <caminho>" (e também "<sha> *<caminho>"), ignorando
// linhas em branco. O SHA sai em minúsculas.
func LerChecksums(caminho string) ([]Linha, error) {
	f, err := os.Open(caminho)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var ls []Linha
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(l) == "" {
			continue
		}
		soma, resto, ok := strings.Cut(l, " ")
		if !ok || soma == "" {
			return nil, fmt.Errorf("linha inválida em %s: %q", caminho, l)
		}
		resto = strings.TrimPrefix(resto, " ")
		resto = strings.TrimPrefix(resto, "*")
		if resto == "" {
			return nil, fmt.Errorf("linha inválida em %s: %q", caminho, l)
		}
		ls = append(ls, Linha{SHA256: strings.ToLower(soma), Caminho: resto})
	}
	return ls, sc.Err()
}

// GerarChecksums escreve <stage>/CHECKSUMS.sha256 com bin/llamafile,
// models/<modeloNome>, cada toolbox e cada cd2-ctl, em ordem alfabética.
// Outros arquivos de models/ (um .DS_Store, por exemplo) ficam de fora, como
// no script: o Copiar pula lixo e a conferência no pen drive falharia.
func GerarChecksums(stage, modeloNome string) error {
	var caminhos []string
	if _, err := os.Stat(filepath.Join(stage, "bin", "llamafile")); err == nil {
		caminhos = append(caminhos, "bin/llamafile")
	}
	if st, err := os.Stat(filepath.Join(stage, "models", modeloNome)); modeloNome != "" && err == nil && st.Mode().IsRegular() {
		caminhos = append(caminhos, "models/"+modeloNome)
	}
	for _, par := range [][2]string{{"bin/toolbox", "toolbox"}, {"bin/ctl", "cd2-ctl"}} {
		raiz := filepath.Join(stage, filepath.FromSlash(par[0]))
		_ = filepath.WalkDir(raiz, func(p string, d fs.DirEntry, err error) error {
			if err != nil || !d.Type().IsRegular() {
				return nil
			}
			if n := d.Name(); n == par[1] || n == par[1]+".exe" {
				rel, rerr := filepath.Rel(stage, p)
				if rerr == nil {
					caminhos = append(caminhos, filepath.ToSlash(rel))
				}
			}
			return nil
		})
	}
	ordenar(caminhos)
	var sb strings.Builder
	for _, c := range caminhos {
		soma, err := sha256Arquivo(filepath.Join(stage, filepath.FromSlash(c)))
		if err != nil {
			return err
		}
		fmt.Fprintf(&sb, "%s  %s\n", soma, c)
	}
	return os.WriteFile(filepath.Join(stage, "CHECKSUMS.sha256"), []byte(sb.String()), 0o644)
}

func ordenar(s []string) { sort.Strings(s) }

func sha256Arquivo(caminho string) (string, error) {
	f, err := os.Open(caminho)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
