// Pacote manifesto descreve as versões e os arquivos que o instalador baixa.
// O manifesto.json é publicado junto com o ZIP da release "iadebolso".
package manifesto

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Arquivo é um arquivo remoto, com conferência opcional de SHA-256 e tamanho.
type Arquivo struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256,omitempty"`
	Bytes  int64  `json:"bytes,omitempty"`
}

// Manifesto é o conteúdo do manifesto.json.
type Manifesto struct {
	Versao     string  `json:"versao"`
	Zip        Arquivo `json:"zip"`
	Llamafile  Arquivo `json:"llamafile"`
	Modelo     Arquivo `json:"modelo"`
	ModeloNome string  `json:"modelo_nome"`
	Toolbox    struct {
		Versao      string   `json:"versao"`
		Base        string   `json:"base"`
		Plataformas []string `json:"plataformas"`
	} `json:"toolbox"`
	Ctl struct {
		Versao string `json:"versao"`
		Base   string `json:"base"`
	} `json:"ctl"`
}

// ErrSemManifesto indica que não houve rede nem manifesto na cache.
var ErrSemManifesto = errors.New("sem manifesto")

const nomeNaCache = "manifesto.json"

// Ler lê e valida um manifesto. Recusa o que não tiver os campos obrigatórios.
func Ler(r io.Reader) (*Manifesto, error) {
	var m Manifesto
	if err := json.NewDecoder(r).Decode(&m); err != nil {
		return nil, fmt.Errorf("manifesto ilegível: %w", err)
	}
	obrigatorios := []struct{ nome, valor string }{
		{"versao", m.Versao},
		{"zip.url", m.Zip.URL},
		{"zip.sha256", m.Zip.SHA256},
		// Sem o SHA do motor, um llamafile antigo e menor na cache seria
		// retomado com Range e passaria na conferência só pelo tamanho.
		{"llamafile.url", m.Llamafile.URL},
		{"llamafile.sha256", m.Llamafile.SHA256},
		{"modelo.url", m.Modelo.URL},
		{"modelo_nome", m.ModeloNome},
		{"toolbox.versao", m.Toolbox.Versao},
		{"toolbox.base", m.Toolbox.Base},
		{"ctl.versao", m.Ctl.Versao},
		{"ctl.base", m.Ctl.Base},
	}
	for _, o := range obrigatorios {
		if o.valor == "" {
			return nil, fmt.Errorf("manifesto incompleto: falta %s", o.nome)
		}
	}
	if len(m.Toolbox.Plataformas) == 0 {
		return nil, fmt.Errorf("manifesto incompleto: falta toolbox.plataformas")
	}
	return &m, nil
}

// Obter baixa o manifesto de url e o grava em <cache>/manifesto.json. Qualquer
// falha de rede, status diferente de 200 ou manifesto inválido faz a leitura
// cair para a cache (daCache == true). Sem rede e sem cache, devolve ErrSemManifesto.
func Obter(ctx context.Context, c *http.Client, url, cache string) (m *Manifesto, daCache bool, err error) {
	if c == nil {
		c = &http.Client{}
	}
	if m, ok := baixar(ctx, c, url, cache); ok {
		return m, false, nil
	}
	f, err := os.Open(filepath.Join(cache, nomeNaCache))
	if err != nil {
		return nil, false, ErrSemManifesto
	}
	defer f.Close()
	m, err = Ler(f)
	if err != nil {
		return nil, false, ErrSemManifesto
	}
	return m, true, nil
}

func baixar(ctx context.Context, c *http.Client, url, cache string) (*Manifesto, bool) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	corpo, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, false
	}
	m, err := Ler(bytesReader(corpo))
	if err != nil {
		return nil, false
	}
	// Falha ao gravar a cache não impede de usar o manifesto recém-baixado.
	_ = gravarAtomico(filepath.Join(cache, nomeNaCache), corpo)
	return m, true
}

func gravarAtomico(destino string, dados []byte) error {
	if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(destino), "manifesto-*.tmp")
	if err != nil {
		return err
	}
	nome := tmp.Name()
	if _, err := tmp.Write(dados); err != nil {
		tmp.Close()
		os.Remove(nome)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(nome)
		return err
	}
	if err := os.Rename(nome, destino); err != nil {
		os.Remove(nome)
		return err
	}
	return nil
}
