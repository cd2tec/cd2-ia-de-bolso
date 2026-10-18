// Comando manifesto monta o manifesto.json da release "iadebolso" e o escreve em stdout.
// Os valores fixos vêm dos scripts de preparo, inclusive a URL e o SHA-256 do llamafile; o SHA-256
// do ZIP é calculado do arquivo. O workflow baixa o motor da URL fixa e passa o SHA que mediu, que
// precisa bater com o dos scripts: assim o pen drive do instalador e o dos scripts levam o mesmo motor.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/manifesto"
)

var reSHA = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

func main() {
	if err := executar(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "manifesto:", err)
		os.Exit(1)
	}
}

func executar(args []string, saida io.Writer) error {
	fs := flag.NewFlagSet("manifesto", flag.ContinueOnError)
	prepUnix := fs.String("prepare-unix", "", "caminho de core/prepare-unix.sh")
	prepWin := fs.String("prepare-windows", "", "caminho de core/prepare-windows.ps1")
	versaoArq := fs.String("version-file", "", "caminho do arquivo VERSION")
	zip := fs.String("zip", "", "arquivo ZIP da release (para o SHA-256 e o tamanho)")
	zipURL := fs.String("zip-url", "", "URL pública do ZIP")
	llamaSHA := fs.String("llamafile-sha256", "", "SHA-256 medido do llamafile baixado da URL fixa")
	llamaBytes := fs.Int64("llamafile-bytes", 0, "tamanho do llamafile em bytes")
	modeloBytes := fs.Int64("modelo-bytes", 0, "tamanho do modelo em bytes")
	if err := fs.Parse(args); err != nil {
		return err
	}

	for nome, v := range map[string]string{
		"-prepare-unix": *prepUnix, "-prepare-windows": *prepWin, "-version-file": *versaoArq,
		"-zip": *zip, "-zip-url": *zipURL, "-llamafile-sha256": *llamaSHA,
	} {
		if v == "" {
			return fmt.Errorf("falta a flag %s", nome)
		}
	}
	if !reSHA.MatchString(*llamaSHA) {
		return fmt.Errorf("-llamafile-sha256 não é um SHA-256: %q", *llamaSHA)
	}

	unix, err := os.ReadFile(*prepUnix)
	if err != nil {
		return err
	}
	win, err := os.ReadFile(*prepWin)
	if err != nil {
		return err
	}
	fixos, err := manifesto.DosScripts(string(unix), string(win))
	if err != nil {
		return err
	}
	if fixos.LlamafileSHA256 == "" || !strings.EqualFold(*llamaSHA, fixos.LlamafileSHA256) {
		return fmt.Errorf("o llamafile baixado (%s) não confere com o SHA-256 fixo nos scripts (%s)", *llamaSHA, fixos.LlamafileSHA256)
	}
	versao, err := os.ReadFile(*versaoArq)
	if err != nil {
		return err
	}
	sha, tamanho, err := shaDoArquivo(*zip)
	if err != nil {
		return err
	}

	var m manifesto.Manifesto
	m.Versao = strings.TrimSpace(string(versao))
	m.Zip = manifesto.Arquivo{URL: *zipURL, SHA256: sha, Bytes: tamanho}
	m.Llamafile = manifesto.Arquivo{URL: fixos.LlamafileURL, SHA256: fixos.LlamafileSHA256, Bytes: *llamaBytes}
	m.Modelo = manifesto.Arquivo{URL: fixos.ModeloURL, SHA256: fixos.ModeloSHA256, Bytes: *modeloBytes}
	m.ModeloNome = fixos.ModeloNome
	m.Toolbox.Versao = fixos.ToolboxVersao
	m.Toolbox.Base = fixos.ToolboxBase
	m.Toolbox.Plataformas = fixos.Plataformas
	m.Ctl.Versao = fixos.CtlVersao
	m.Ctl.Base = fixos.CtlBase

	enc := json.NewEncoder(saida)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

func shaDoArquivo(caminho string) (string, int64, error) {
	f, err := os.Open(caminho)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
