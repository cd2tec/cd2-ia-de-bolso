// Pacote preparo monta a cópia local da IA de Bolso no cache, o mesmo que o
// core/make-usb-unix.sh faz na etapa "1. copia local".
package preparo

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/baixar"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/manifesto"
)

// Evento descreve o andamento de um item. Item: "zip", "motor", "modelo",
// "toolbox windows-amd64", "cd2-ctl darwin-arm64"... Total é 0 quando o
// tamanho não é conhecido pelo manifesto. Com Conferindo, nada está sendo
// baixado: o item (ou a cópia local, no Item "stage") está sendo conferido,
// extraído ou somado, e Feitos e Total não valem.
type Evento struct {
	Item          string
	Feitos, Total int64
	Conferindo    bool
}

// ErrFaltaInternet indica que algo falta (ou não confere) na cache e o modo é sem rede.
var ErrFaltaInternet = errors.New("falta baixar e não há internet")

// minimos são os tamanhos mínimos dos scripts (modelo e toolbox). Os testes trocam.
var minimos = struct{ Modelo, Toolbox int64 }{1_000_000_000, 100_000_000}

const prefixoZip = "cd2tech-iadebolso/"

var itensDoZip = map[string]bool{
	"INICIAR-CD2-Windows.bat": true, "INICIAR-CD2-Mac.command": true, "iniciar-cd2-linux.sh": true,
	"README.md": true, "LICENSE": true, "VERSION": true,
}

func nomeDoZip(m *manifesto.Manifesto, cache string) string {
	return filepath.Join(cache, "cd2tech-iadebolso-"+m.Versao+".zip")
}

func caminhoStage(cache string) string { return filepath.Join(cache, "CD2-HARNESS") }

// Montar monta <cache>/CD2-HARNESS e devolve o caminho. Com semRede, só confere
// o que já existe e devolve ErrFaltaInternet se algo faltar ou não conferir.
func Montar(ctx context.Context, c *http.Client, m *manifesto.Manifesto, cache string, semRede bool, ev func(Evento)) (string, error) {
	stage := caminhoStage(cache)
	if err := os.MkdirAll(stage, 0o755); err != nil {
		return "", err
	}
	conferindo := func(item string) {
		if ev != nil {
			ev(Evento{Item: item, Conferindo: true})
		}
	}
	emitir := func(item string, total int64) baixar.Progresso {
		// fim é o total que o download conhece (o do manifesto ou o do servidor).
		return func(feitos, fim int64) {
			if ev == nil {
				return
			}
			ev(Evento{Item: item, Feitos: feitos, Total: total})
			// O último pedaço chegou: o que vem é a conferência do SHA.
			if fim > 0 && feitos >= fim {
				conferindo(item)
			}
		}
	}
	// obter baixa (ou só confere, sem rede) um arquivo. Antes, avisa que está
	// conferindo: se o arquivo já estiver completo na cache, a conferência do
	// SHA é tudo o que acontece.
	obter := func(item, url, destino string, esperado manifesto.Arquivo) error {
		conferindo(item)
		if semRede {
			if err := baixar.Conferir(destino, esperado); err != nil {
				return fmt.Errorf("%w: %s (%v)", ErrFaltaInternet, item, err)
			}
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
			return err
		}
		if err := baixar.Arquivo(ctx, c, url, destino, esperado, emitir(item, esperado.Bytes)); err != nil {
			return fmt.Errorf("%s: %w", item, err)
		}
		return nil
	}

	// ZIP e extração.
	zipPath := nomeDoZip(m, cache)
	if err := obter("zip", m.Zip.URL, zipPath, m.Zip); err != nil {
		return "", err
	}
	conferindo("stage")
	if err := limparStage(stage); err != nil {
		return "", err
	}
	if err := extrair(zipPath, stage); err != nil {
		return "", err
	}

	// Motor.
	llama := filepath.Join(stage, "bin", "llamafile")
	if err := obter("motor", m.Llamafile.URL, llama, m.Llamafile); err != nil {
		return "", err
	}
	if err := os.Chmod(llama, 0o755); err != nil {
		return "", err
	}
	if err := copiarSeDiferente(llama, llama+".exe"); err != nil {
		return "", err
	}

	// Modelo.
	modelo := filepath.Join(stage, "models", m.ModeloNome)
	if err := obter("modelo", m.Modelo.URL, modelo, m.Modelo); err != nil {
		return "", err
	}
	if err := tamanhoMinimo(modelo, minimos.Modelo, "modelo"); err != nil {
		return "", err
	}
	if err := tirarModelosAntigos(filepath.Join(stage, "models"), m.ModeloNome); err != nil {
		return "", err
	}

	// Toolbox.
	for _, plat := range m.Toolbox.Plataformas {
		os_, arch, ok := cortarPlat(plat)
		if !ok {
			return "", fmt.Errorf("plataforma inválida no manifesto: %q", plat)
		}
		ext := extWin(os_)
		item := "toolbox " + plat
		destino := filepath.Join(stage, "bin", "toolbox", plat, "toolbox"+ext)
		url := strings.TrimRight(m.Toolbox.Base, "/") + "/" + os_ + "/" + arch + "/toolbox" + ext
		if err := obter(item, url, destino, manifesto.Arquivo{}); err != nil {
			return "", err
		}
		if err := tamanhoMinimo(destino, minimos.Toolbox, item); err != nil {
			return "", err
		}
		if err := os.Chmod(destino, 0o755); err != nil {
			return "", err
		}
	}

	// cd2-ctl, conferido pelo SHA256SUMS do release.
	sums, err := somasDoCtl(ctx, c, m, cache, semRede)
	if err != nil {
		return "", err
	}
	for _, plat := range m.Toolbox.Plataformas {
		os_, _, _ := cortarPlat(plat)
		ext := extWin(os_)
		asset := "cd2-ctl-" + plat + ext
		item := "cd2-ctl " + plat
		want := sums[asset]
		if want == "" {
			return "", fmt.Errorf("%s: o SHA256SUMS do release não lista %s", item, asset)
		}
		destino := filepath.Join(stage, "bin", "ctl", plat, "cd2-ctl"+ext)
		url := strings.TrimRight(m.Ctl.Base, "/") + "/" + asset
		if err := obter(item, url, destino, manifesto.Arquivo{SHA256: want}); err != nil {
			return "", err
		}
		if err := os.Chmod(destino, 0o755); err != nil {
			return "", err
		}
	}

	conferindo("stage")
	if err := GerarChecksums(stage, m.ModeloNome); err != nil {
		return "", err
	}
	return stage, nil
}

// BytesParaBaixar estima quanto falta baixar: soma dos Bytes conhecidos (zip,
// motor e modelo) cujo arquivo ainda não está completo em disco. Não usa rede.
func BytesParaBaixar(m *manifesto.Manifesto, cache string) int64 {
	stage := caminhoStage(cache)
	var total int64
	for _, it := range []struct {
		a       manifesto.Arquivo
		destino string
	}{
		{m.Zip, nomeDoZip(m, cache)},
		{m.Llamafile, filepath.Join(stage, "bin", "llamafile")},
		{m.Modelo, filepath.Join(stage, "models", m.ModeloNome)},
	} {
		if it.a.Bytes <= 0 {
			continue
		}
		if st, err := os.Stat(it.destino); err == nil && st.Size() == it.a.Bytes {
			continue
		}
		total += it.a.Bytes
	}
	return total
}

func extWin(os_ string) string {
	if os_ == "windows" {
		return ".exe"
	}
	return ""
}

func cortarPlat(plat string) (os_, arch string, ok bool) {
	i := strings.LastIndex(plat, "-")
	if i <= 0 || i == len(plat)-1 {
		return "", "", false
	}
	return plat[:i], plat[i+1:], true
}

// somasDoCtl devolve asset -> sha256. Online, baixa e guarda em
// <cache>/ctl-SHA256SUMS-<versao>; se o download falhar, usa a cópia guardada.
func somasDoCtl(ctx context.Context, c *http.Client, m *manifesto.Manifesto, cache string, semRede bool) (map[string]string, error) {
	guardado := filepath.Join(cache, "ctl-SHA256SUMS-"+m.Ctl.Versao)
	if !semRede {
		if err := baixarSomas(ctx, c, strings.TrimRight(m.Ctl.Base, "/")+"/SHA256SUMS", guardado); err != nil {
			if _, serr := os.Stat(guardado); serr != nil {
				return nil, fmt.Errorf("SHA256SUMS do cd2-ctl: %w", err)
			}
		}
	}
	ls, err := LerChecksums(guardado)
	if err != nil {
		if semRede {
			return nil, fmt.Errorf("%w: SHA256SUMS do cd2-ctl", ErrFaltaInternet)
		}
		return nil, err
	}
	out := map[string]string{}
	for _, l := range ls {
		out[l.Caminho] = l.SHA256
	}
	return out, nil
}

func baixarSomas(ctx context.Context, c *http.Client, url, destino string) error {
	if c == nil {
		c = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "cd2-harness")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: download de %s: %s", baixar.ErrHTTP, url, resp.Status)
	}
	corpo, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	tmp := destino + ".tmp"
	if err := os.WriteFile(tmp, corpo, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, destino); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// limparStage apaga tudo do stage, menos bin, models e CHECKSUMS.sha256.
func limparStage(stage string) error {
	es, err := os.ReadDir(stage)
	if err != nil {
		return err
	}
	for _, e := range es {
		switch e.Name() {
		case "bin", "models", "CHECKSUMS.sha256":
			continue
		}
		if err := os.RemoveAll(filepath.Join(stage, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func aceitoNoZip(rel string) bool {
	if rel == "" || strings.Contains(rel, "..") {
		return false
	}
	for _, parte := range strings.Split(rel, "/") {
		if parte == ".DS_Store" || strings.HasPrefix(parte, "._") {
			return false
		}
	}
	switch rel {
	case "core/make-usb-unix.sh", "core/make-usb-windows.ps1":
		return false
	}
	return itensDoZip[rel] || strings.HasPrefix(rel, "web/") || strings.HasPrefix(rel, "core/")
}

// extrair tira do ZIP só o que vai para o pen drive, sem o prefixo cd2tech-iadebolso/.
func extrair(zipPath, stage string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("zip ilegível: %w", err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, prefixoZip) {
			continue
		}
		rel := path.Clean(strings.TrimPrefix(f.Name, prefixoZip))
		if rel == "." || !aceitoNoZip(rel) {
			continue
		}
		destino := filepath.Join(stage, filepath.FromSlash(rel))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(destino, 0o755); err != nil {
				return err
			}
			continue
		}
		if !f.Mode().IsRegular() {
			continue
		}
		if err := extrairUm(f, destino); err != nil {
			return err
		}
	}
	return nil
}

func extrairUm(f *zip.File, destino string) error {
	if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	modo := f.Mode().Perm()
	if modo == 0 {
		modo = 0o644
	}
	modo |= 0o600
	out, err := os.OpenFile(destino, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, modo)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(destino, modo)
}

func tamanhoMinimo(caminho string, minimo int64, nome string) error {
	st, err := os.Stat(caminho)
	if err != nil {
		return err
	}
	if st.Size() < minimo {
		return fmt.Errorf("%s com tamanho suspeito (%d bytes): %w", nome, st.Size(), baixar.ErrConferencia)
	}
	return nil
}

func tirarModelosAntigos(dir, atual string) error {
	es, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range es {
		if e.Name() != atual && strings.HasSuffix(e.Name(), ".gguf") {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

// copiarSeDiferente cria dst com o conteúdo de src, se dst não existir ou diferir.
func copiarSeDiferente(src, dst string) error {
	a, err := os.Stat(src)
	if err != nil {
		return err
	}
	if b, err := os.Stat(dst); err == nil && b.Size() == a.Size() {
		sa, err1 := sha256Arquivo(src)
		sb, err2 := sha256Arquivo(dst)
		if err1 == nil && err2 == nil && sa == sb {
			return nil
		}
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(dst, 0o755)
}
