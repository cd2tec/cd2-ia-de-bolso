package gravar

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/baixar"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/discos"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/manifesto"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/pendrive"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/preparo"
)

var plats = []string{"windows-amd64", "darwin-arm64", "darwin-amd64", "linux-amd64"}

func sha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// falso é um discos.Sistema que não toca em disco nenhum: registra as
// chamadas (junto com os downloads do servidor) e entrega uma pasta temporária.
type falso struct {
	mu       sync.Mutex
	log      []string
	listas   [][]discos.Bruto // uma por chamada de Listar; a última se repete
	chamadas int
	pasta    string // pasta que faz o papel do volume gravado
	apagar   error
	ejetar   error
	aoApagar func() // chamado dentro do Apagar, antes de ele falhar
	aoListar func() // chamado depois de cada Listar (o teste cancela aqui)
}

func (f *falso) registrar(s string) {
	f.mu.Lock()
	f.log = append(f.log, s)
	f.mu.Unlock()
}

func (f *falso) copia() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.log...)
}

func (f *falso) Listar(context.Context) ([]discos.Bruto, error) {
	f.registrar("Listar")
	f.mu.Lock()
	defer f.mu.Unlock()
	i := f.chamadas
	if i >= len(f.listas) {
		i = len(f.listas) - 1
	}
	f.chamadas++
	l, hook := f.listas[i], f.aoListar
	if hook != nil {
		defer hook()
	}
	return l, nil
}

func (f *falso) Apagar(ctx context.Context, id, rotulo string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err // cancelado: nada é apagado e nada é registrado
	}
	f.registrar("Apagar " + id + " " + rotulo)
	if f.aoApagar != nil {
		f.aoApagar()
		return "", errors.New("processo morto no meio")
	}
	if f.apagar != nil {
		return "", f.apagar
	}
	return f.pasta, nil
}

func (f *falso) Ejetar(_ context.Context, id, montagem string) error {
	f.registrar("Ejetar " + id)
	return f.ejetar
}

func (f *falso) contem(prefixo string) bool {
	for _, l := range f.copia() {
		if strings.HasPrefix(l, prefixo) {
			return true
		}
	}
	return false
}

func (f *falso) indice(prefixo string, ultimo bool) int {
	r := -1
	for i, l := range f.copia() {
		if strings.HasPrefix(l, prefixo) {
			r = i
			if !ultimo {
				return r
			}
		}
	}
	return r
}

type ambiente struct {
	t      *testing.T
	srv    *httptest.Server
	m      *manifesto.Manifesto
	sis    *falso
	disco  discos.Disco
	cache  string
	url    string
	falhas map[string]bool
	mu     sync.Mutex
	gets   int
}

func criarZip(t *testing.T) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(nome, conteudo string) {
		h := &zip.FileHeader{Name: "cd2tech-iadebolso/" + nome, Method: zip.Deflate}
		h.SetMode(0o644)
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(conteudo))
	}
	for _, n := range []string{"INICIAR-CD2-Windows.bat", "INICIAR-CD2-Mac.command", "ia-de-bolso.sh", "README.md", "LICENSE"} {
		add(n, n)
	}
	add("VERSION", "0.13.0")
	add("core/server.js", "js")
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func novoAmbiente(t *testing.T) *ambiente {
	t.Helper()
	t.Cleanup(preparo.AjustarMinimosParaTeste(10, 10))
	a := &ambiente{t: t, cache: t.TempDir(), falhas: map[string]bool{}}
	zipB := criarZip(t)
	modelo := []byte("modelo-conteudo-grande")
	motor := []byte("llamafile-conteudo")
	arquivos := map[string][]byte{
		"/zip":       zipB,
		"/llamafile": motor,
		"/modelo":    modelo,
	}
	var sums strings.Builder
	for _, p := range plats {
		os_, arch, _ := strings.Cut(p, "-")
		ext := ""
		if os_ == "windows" {
			ext = ".exe"
		}
		arquivos[fmt.Sprintf("/tb/%s/%s/toolbox%s", os_, arch, ext)] = []byte("toolbox-" + p)
		c := []byte("ctl-" + p)
		arquivos["/ctl/cd2-ctl-"+p+ext] = c
		fmt.Fprintf(&sums, "%s  cd2-ctl-%s%s\n", sha(c), p, ext)
	}
	arquivos["/ctl/SHA256SUMS"] = []byte(sums.String())

	// A pasta do "pen drive" fica dentro de outra, para o teste de remoção
	// trocar a pasta de fora por um arquivo.
	fora := filepath.Join(t.TempDir(), "pen")
	pasta := filepath.Join(fora, "CD2")
	if err := os.MkdirAll(pasta, 0o755); err != nil {
		t.Fatal(err)
	}
	a.disco = discos.Disco{ID: "disk9", Nome: "Teste", Bytes: 32_000_000_000, Situacao: discos.Ok, Rotulo: "Teste 32 GB"}
	bruto := discos.Bruto{ID: "disk9", Nome: "Teste", Bytes: 32_000_000_000}
	a.sis = &falso{listas: [][]discos.Bruto{{bruto}}, pasta: pasta}

	a.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		a.gets++
		falha := a.falhas[r.URL.Path]
		a.mu.Unlock()
		a.sis.registrar("get " + r.URL.Path)
		if falha {
			http.Error(w, "quebrou", http.StatusInternalServerError)
			return
		}
		if r.URL.Path == "/manifesto.json" {
			json.NewEncoder(w).Encode(a.m)
			return
		}
		b, ok := arquivos[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(b))
	}))
	t.Cleanup(a.srv.Close)
	a.url = a.srv.URL + "/manifesto.json"

	a.m = &manifesto.Manifesto{
		Versao:     "0.13.0",
		Zip:        manifesto.Arquivo{URL: a.srv.URL + "/zip", SHA256: sha(zipB), Bytes: int64(len(zipB))},
		Llamafile:  manifesto.Arquivo{URL: a.srv.URL + "/llamafile", SHA256: sha(motor), Bytes: int64(len(motor))},
		Modelo:     manifesto.Arquivo{URL: a.srv.URL + "/modelo", SHA256: sha(modelo), Bytes: int64(len(modelo))},
		ModeloNome: "modelo.gguf",
	}
	a.m.Toolbox.Versao = "1.14.0"
	a.m.Toolbox.Base = a.srv.URL + "/tb"
	a.m.Toolbox.Plataformas = plats
	a.m.Ctl.Versao = "0.6.0"
	a.m.Ctl.Base = a.srv.URL + "/ctl"
	return a
}

func (a *ambiente) opcoes() Opcoes {
	return Opcoes{Disco: a.disco, Cache: a.cache, URLManifesto: a.url, HTTP: a.srv.Client(), Sistema: a.sis}
}

func (a *ambiente) requisicoes() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.gets
}

func fraseDe(t *testing.T, err error) string {
	t.Helper()
	var e *Erro
	if !errors.As(err, &e) {
		t.Fatalf("erro %v (%T) não é *Erro", err, err)
	}
	return e.Frase
}

func TestOrdem(t *testing.T) {
	a := novoAmbiente(t)
	var eventos []Evento
	if err := Gravar(context.Background(), a.opcoes(), func(e Evento) { eventos = append(eventos, e) }); err != nil {
		t.Fatal(err)
	}
	log := a.sis.copia()
	var chamadas []string
	for _, l := range log {
		if !strings.HasPrefix(l, "get ") {
			chamadas = append(chamadas, strings.Fields(l)[0])
		}
	}
	if got := strings.Join(chamadas, ","); got != "Listar,Apagar,Ejetar" {
		t.Fatalf("chamadas = %s", got)
	}
	if a.sis.indice("Apagar", false) < a.sis.indice("get ", true) {
		t.Fatalf("Apagar antes do último download: %v", log)
	}
	if !a.sis.contem("Apagar disk9 CD2") {
		t.Fatalf("Apagar sem o id e o rótulo CD2: %v", log)
	}
	for _, p := range []string{"VERSION", "CHECKSUMS.sha256", "models/modelo.gguf", "bin/llamafile"} {
		if _, err := os.Stat(filepath.Join(a.sis.pasta, p)); err != nil {
			t.Errorf("falta no pen drive: %s", p)
		}
	}
	if len(eventos) == 0 || eventos[len(eventos)-1].Etapa != "pronto" || eventos[len(eventos)-1].Frase != "Pronto." {
		t.Fatalf("último evento = %+v", eventos[len(eventos)-1])
	}
	gs, _ := filepath.Glob(filepath.Join(a.cache, "CHECKSUMS-*.sha256"))
	if len(gs) != 1 {
		t.Errorf("checksums guardados = %v", gs)
	}
}

func TestDownloadFalhaNaoApaga(t *testing.T) {
	a := novoAmbiente(t)
	a.falhas["/modelo"] = true
	err := Gravar(context.Background(), a.opcoes(), nil)
	if got := fraseDe(t, err); got != fraseInternet {
		t.Fatalf("frase = %q", got)
	}
	if a.sis.contem("Apagar") || a.sis.contem("Listar") {
		t.Fatalf("mexeu no disco: %v", a.sis.copia())
	}
}

func TestDiscoMudou(t *testing.T) {
	casos := map[string][]discos.Bruto{
		"outro tamanho": {{ID: "disk9", Nome: "Teste", Bytes: 64_000_000_000}},
		"outro nome":    {{ID: "disk9", Nome: "Outro", Bytes: 32_000_000_000}},
		"sumiu":         {},
		"outro id":      {{ID: "disk7", Nome: "Teste", Bytes: 32_000_000_000}},
		"virou pequeno": {{ID: "disk9", Nome: "Teste", Bytes: 32_000_000_000}},
	}
	for nome, lista := range casos {
		t.Run(nome, func(t *testing.T) {
			a := novoAmbiente(t)
			a.sis.listas = [][]discos.Bruto{lista}
			if nome == "virou pequeno" {
				// Mesmo disco para Mesmo, mas a Situacao recalculada fica Pequeno.
				a.disco.Bytes, a.disco.Situacao = 1_000_000_000, discos.Pequeno
				a.sis.listas = [][]discos.Bruto{{{ID: "disk9", Nome: "Teste", Bytes: 1_000_000_000}}}
			}
			err := Gravar(context.Background(), a.opcoes(), nil)
			if got := fraseDe(t, err); got != fraseDiscoMudou {
				t.Fatalf("frase = %q", got)
			}
			if a.sis.contem("Apagar") {
				t.Fatalf("apagou: %v", a.sis.copia())
			}
		})
	}
}

func TestRemovidoNoMeio(t *testing.T) {
	a := novoAmbiente(t)
	fora := filepath.Dir(a.sis.pasta)
	tirado, copiando := false, 0
	err := Gravar(context.Background(), a.opcoes(), func(e Evento) {
		if e.Frase == fraseCopiando {
			copiando++
		}
		// Segundo evento de cópia: já houve bytes copiados.
		if copiando == 2 && !tirado {
			tirado = true
			os.RemoveAll(fora)
			os.WriteFile(fora, []byte("x"), 0o644)
		}
	})
	if got := fraseDe(t, err); got != fraseRemovido {
		t.Fatalf("frase = %q (%v)", got, err)
	}
	if !errors.Is(err, pendrive.ErrRemovido) {
		t.Errorf("Causa deveria ser ErrRemovido: %v", err)
	}
}

func TestCancelarAntesEDepois(t *testing.T) {
	t.Run("durante o download", func(t *testing.T) {
		a := novoAmbiente(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := Gravar(ctx, a.opcoes(), func(e Evento) {
			if e.Etapa == "baixando" {
				cancel()
			}
		})
		if got := fraseDe(t, err); got != fraseCancelouAntes {
			t.Fatalf("frase = %q", got)
		}
		if a.sis.contem("Apagar") {
			t.Fatal("apagou")
		}
	})
	t.Run("depois de apagar", func(t *testing.T) {
		a := novoAmbiente(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := Gravar(ctx, a.opcoes(), func(e Evento) {
			if e.Frase == fraseCopiando {
				cancel()
			}
		})
		if got := fraseDe(t, err); got != fraseCancelouApos {
			t.Fatalf("frase = %q", got)
		}
		if !a.sis.contem("Apagar") {
			t.Fatal("deveria ter apagado antes de cancelar")
		}
	})
}

func TestCancelarEntreReconferenciaEApagar(t *testing.T) {
	a := novoAmbiente(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.sis.aoListar = cancel
	var copiou bool
	err := Gravar(ctx, a.opcoes(), func(e Evento) {
		if e.Frase == fraseCopiando {
			copiou = true
		}
	})
	if got := fraseDe(t, err); got != fraseCancelouAntes {
		t.Fatalf("frase = %q", got)
	}
	if a.sis.contem("Apagar") || copiou {
		t.Fatalf("apagou ou anunciou cópia: %v", a.sis.copia())
	}
}

func TestCancelarDuranteApagar(t *testing.T) {
	a := novoAmbiente(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.sis.aoApagar = cancel
	var copiou bool
	err := Gravar(ctx, a.opcoes(), func(e Evento) {
		if e.Frase == fraseCopiando {
			copiou = true
		}
	})
	if got := fraseDe(t, err); got != fraseCancelouApos {
		t.Fatalf("frase = %q", got)
	}
	if copiou {
		t.Fatal("anunciou cópia sem o Apagar ter dado certo")
	}
}

func TestEtapaNovaSempreEmite(t *testing.T) {
	a := novoAmbiente(t)
	vistas := map[string]bool{}
	if err := Gravar(context.Background(), a.opcoes(), func(e Evento) { vistas[e.Etapa] = true }); err != nil {
		t.Fatal(err)
	}
	for _, et := range []string{"baixando", "copiando", "conferindo", "pronto"} {
		if !vistas[et] {
			t.Errorf("etapa %s sem evento", et)
		}
	}
	// Direto no andamento: troca de etapa e de frase vencem o passo mínimo.
	var n int
	ad := &andamento{agora: time.Now, ev: func(Evento) { n++ }, baixar: 1000, copiar: 1000}
	ad.publicar("copiando", "a", 0, 10, 0, false)
	ad.publicar("copiando", "a", 1, 10000000, 0, false) // dentro do passo: não sai
	ad.publicar("conferindo", "b", 1, 10000000, 0, false)
	ad.publicar("conferindo", "c", 1, 10000000, 0, false)
	if n != 3 {
		t.Fatalf("eventos = %d, esperava 3", n)
	}
}

func TestGuardarChecksumsFalhaNaoEhErro(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("chmod não impede escrita no Windows nem para root")
	}
	a := novoAmbiente(t)
	t.Cleanup(func() { os.Chmod(a.cache, 0o755) })
	var ultimo Evento
	err := Gravar(context.Background(), a.opcoes(), func(e Evento) {
		if e.Etapa == "conferindo" {
			os.Chmod(a.cache, 0o555)
		}
		ultimo = e
	})
	if err != nil {
		t.Fatal(err)
	}
	if ultimo.Etapa != "pronto" || !a.sis.contem("Ejetar") {
		t.Fatalf("último = %+v, log %v", ultimo, a.sis.copia())
	}
}

func TestFracaoCresce(t *testing.T) {
	a := novoAmbiente(t)
	var eventos []Evento
	if err := Gravar(context.Background(), a.opcoes(), func(e Evento) { eventos = append(eventos, e) }); err != nil {
		t.Fatal(err)
	}
	var etapas []string
	ant := -1.0
	for _, e := range eventos {
		if e.Fracao < ant {
			t.Fatalf("fração diminuiu de %v para %v em %+v", ant, e.Fracao, e)
		}
		ant = e.Fracao
		if len(etapas) == 0 || etapas[len(etapas)-1] != e.Etapa {
			etapas = append(etapas, e.Etapa)
		}
	}
	if got := strings.Join(etapas, ","); got != "baixando,copiando,conferindo,pronto" {
		t.Fatalf("etapas = %s", got)
	}
	u := eventos[len(eventos)-1]
	if u.Etapa != "pronto" || u.Fracao != 1 {
		t.Fatalf("último = %+v", u)
	}
	for _, e := range eventos[:len(eventos)-1] {
		if e.Fracao >= 1 {
			t.Fatalf("fração 1 antes do pronto: %+v", e)
		}
	}
}

func TestSemRedeComCache(t *testing.T) {
	a := novoAmbiente(t)
	if err := Gravar(context.Background(), a.opcoes(), nil); err != nil {
		t.Fatal(err)
	}
	a.srv.Close()
	antes := a.requisicoes()
	var ultimo Evento
	if err := Gravar(context.Background(), a.opcoes(), func(e Evento) { ultimo = e }); err != nil {
		t.Fatalf("segunda gravação: %v", err)
	}
	if a.requisicoes() != antes {
		t.Fatalf("houve requisição com o servidor fechado")
	}
	if ultimo.Etapa != "pronto" {
		t.Fatalf("último = %+v", ultimo)
	}
}

func TestFormatarFalha(t *testing.T) {
	a := novoAmbiente(t)
	a.sis.apagar = errors.New("diskutil falhou")
	err := Gravar(context.Background(), a.opcoes(), nil)
	if got := fraseDe(t, err); got != fraseFormatar {
		t.Fatalf("frase = %q", got)
	}
}

func TestEjetarComErroEhPronto(t *testing.T) {
	a := novoAmbiente(t)
	a.sis.ejetar = errors.New("ocupado")
	var ultimo Evento
	if err := Gravar(context.Background(), a.opcoes(), func(e Evento) { ultimo = e }); err != nil {
		t.Fatal(err)
	}
	if ultimo.Etapa != "pronto" || ultimo.Frase != fraseProntoSem || ultimo.Fracao != 1 {
		t.Fatalf("último = %+v", ultimo)
	}
}

func TestAntivirus(t *testing.T) {
	antigo := sistemaOperacional
	sistemaOperacional = "windows"
	t.Cleanup(func() { sistemaOperacional = antigo })
	a := novoAmbiente(t)
	exe := filepath.Join(a.cache, "CD2-HARNESS", "bin", "llamafile.exe")
	err := Gravar(context.Background(), a.opcoes(), func(e Evento) {
		// O motor já foi copiado para .exe quando o modelo começa a baixar.
		if strings.Contains(e.Frase, "o modelo") {
			os.Truncate(exe, 0)
		}
	})
	if got := fraseDe(t, err); got != fraseAntivirus || strings.Contains(got, a.cache) {
		t.Fatalf("frase = %q", got)
	}
	if a.sis.contem("Apagar") || a.sis.contem("Listar") {
		t.Fatalf("mexeu no disco: %v", a.sis.copia())
	}
}

func TestFrases(t *testing.T) {
	casos := []struct {
		nome string
		got  string
		want string
	}{
		{"sem manifesto", frasePreparo(manifesto.ErrSemManifesto), "Não consegui baixar o que falta. Confira a internet e tente de novo. Nada foi apagado."},
		{"falta internet", frasePreparo(fmt.Errorf("zip: %w", preparo.ErrFaltaInternet)), "Não consegui baixar o que falta. Confira a internet e tente de novo. Nada foi apagado."},
		{"http 500", frasePreparo(fmt.Errorf("modelo: %w: download de x: 500", baixar.ErrHTTP)), "Não consegui baixar o que falta. Confira a internet e tente de novo. Nada foi apagado."},
		{"corte no corpo", frasePreparo(fmt.Errorf("modelo: %w", io.ErrUnexpectedEOF)), "Não consegui baixar o que falta. Confira a internet e tente de novo. Nada foi apagado."},
		{"texto parecido não é rede", frasePreparo(errors.New("download de algo")), "Não deu certo. Tente de novo."},
		{"conferência", frasePreparo(fmt.Errorf("modelo: %w", baixar.ErrConferencia)), "Um arquivo chegou com defeito duas vezes. Tente de novo daqui a pouco. Nada foi apagado."},
		{"antivírus", fraseAntivirus, "O antivírus removeu o motor da IA. Libere a pasta da IA de Bolso no antivírus e tente de novo. Nada foi apagado."},
		{"conferindo os arquivos", fraseConferindoArquivos, "Conferindo os arquivos"},
		{"apagando", fraseApagando, "Apagando o pen drive"},
		{"não cabe", fraseNaoCabe, "Esse pen drive é pequeno demais para a IA de Bolso. Nada foi apagado."},
		{"próprio", fraseProprio, "O instalador está rodando deste pen drive. Copie o instalador para o computador e abra de lá."},
		{"disco mudou", fraseDiscoMudou, "O pen drive mudou desde que você escolheu. Escolha de novo. Nada foi apagado."},
		{"formatar", fraseFormatar, "Não consegui formatar o pen drive. Tire, plugue de novo e tente outra vez."},
		{"removido", fraseGravacao(fmt.Errorf("x: %w", pendrive.ErrRemovido)), "O pen drive foi removido no meio da gravação. Plugue de novo e grave outra vez."},
		{"diferente", fraseGravacao(fmt.Errorf("x: %w", pendrive.ErrDiferente)), "A conferência encontrou diferença. Não use este pen drive. Grave de novo e, se repetir, troque de pen drive."},
		{"cancelou antes", fraseCancelouAntes, "Gravação cancelada. Nada foi apagado."},
		{"cancelou depois", fraseCancelouApos, "Gravação cancelada. O pen drive ficou incompleto. Grave de novo antes de usar."},
		{"ejetar", fraseProntoSem, "Pronto. Ejete o pen drive pelo sistema antes de tirar."},
		{"não mapeado no preparo", frasePreparo(errors.New("disco cheio")), "Não deu certo. Tente de novo."},
		{"não mapeado depois de apagar", fraseGravacao(errors.New("erro qualquer")), "Não deu certo. O pen drive ficou incompleto. Grave de novo antes de usar."},
		{"copiando", fraseCopiando, "Copiando para o pen drive"},
		{"conferindo", fraseConferindo, "Conferindo o que foi gravado"},
		{"baixando", fraseBaixando("modelo", 1_600_000_000, 3_900_000_000), "Baixando o modelo (1,6 de 3,9 GB)"},
		{"baixando zip", fraseBaixando("zip", 0, 2_500_000_000), "Baixando os arquivos da IA de Bolso (0,0 de 2,5 GB)"},
		{"baixando motor", fraseBaixando("motor", 100_000_000, 200_000_000), "Baixando o motor (0,1 de 0,2 GB)"},
		{"baixando toolbox", fraseBaixando("toolbox windows-amd64", 1, 2), "Baixando o toolbox (0,0 de 0,0 GB)"},
		{"baixando ctl", fraseBaixando("cd2-ctl darwin-arm64", 0, 0), "Baixando o controle do banco"},
		{"baixando sem total", fraseBaixando("modelo", 5, 0), "Baixando o modelo"},
	}
	for _, c := range casos {
		if c.got != c.want {
			t.Errorf("%s: %q, esperava %q", c.nome, c.got, c.want)
		}
	}
}

func TestErroDevolveFrase(t *testing.T) {
	e := &Erro{Frase: "oi", Causa: baixar.ErrConferencia}
	if e.Error() != "oi" || !errors.Is(e, baixar.ErrConferencia) {
		t.Fatal("Erro mal formado")
	}
}

func TestDiscosVazioEhLista(t *testing.T) {
	s := &falso{listas: [][]discos.Bruto{nil}}
	l, err := Discos(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(l)
	if string(b) != "[]" {
		t.Fatalf("json = %s", b)
	}
	s = &falso{listas: [][]discos.Bruto{{
		{ID: "disk2", Nome: "A", Bytes: 32_000_000_000},
		{ID: "disk0", Nome: "Boot", Bytes: 500_000_000_000, Boot: true},
		{ID: "disk3", Nome: "Grande", Bytes: 2_000_000_000_000},
	}}}
	l, _ = Discos(context.Background(), s)
	if len(l) != 1 || l[0].ID != "disk2" {
		t.Fatalf("lista = %+v", l)
	}
}

func TestRestante(t *testing.T) {
	t0 := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	agora := t0
	var ult Evento
	a := &andamento{agora: func() time.Time { return agora }, ev: func(e Evento) { ult = e }, baixar: 1000, copiar: 0}
	a.publicar("baixando", "x", 0, 1000, 0, true)
	agora = t0.Add(2 * time.Second)
	a.publicar("baixando", "x", 200, 1000, 0, true)
	if ult.Restante != -1 {
		t.Fatalf("antes de 3 s deveria ser -1, veio %d", ult.Restante)
	}
	agora = t0.Add(4 * time.Second)
	a.publicar("baixando", "x", 400, 1000, 0, true)
	// 400 bytes em 4 s = 100 B/s; faltam 600.
	if ult.Restante != 6 {
		t.Fatalf("restante = %d, esperava 6", ult.Restante)
	}
}

// No segundo pen drive a cache já está completa: nada é baixado, então
// nenhuma frase pode dizer "Baixando". A conferência e o Apagar falam.
func TestSegundaGravacaoNaoDizBaixando(t *testing.T) {
	a := novoAmbiente(t)
	if err := Gravar(context.Background(), a.opcoes(), nil); err != nil {
		t.Fatal(err)
	}
	a.sis.pasta = t.TempDir()
	var frases []string
	if err := Gravar(context.Background(), a.opcoes(), func(e Evento) { frases = append(frases, e.Etapa+": "+e.Frase) }); err != nil {
		t.Fatal(err)
	}
	vistas := map[string]bool{}
	for _, f := range frases {
		if strings.Contains(f, "Baixando") {
			t.Fatalf("frase de download com a cache completa: %q (todas: %v)", f, frases)
		}
		vistas[f] = true
	}
	for _, f := range []string{"baixando: " + fraseConferindoArquivos, "copiando: " + fraseApagando} {
		if !vistas[f] {
			t.Errorf("faltou %q em %v", f, frases)
		}
	}
}

// "Apagando o pen drive" sai antes do Apagar, na etapa copiando, sem a
// fração andar; e a conferência da cache sai na etapa baixando.
func TestApagandoAntesDoApagarSemAndar(t *testing.T) {
	a := novoAmbiente(t)
	var eventos []Evento
	err := Gravar(context.Background(), a.opcoes(), func(e Evento) {
		if e.Frase == fraseApagando {
			a.sis.registrar("evento Apagando")
		}
		eventos = append(eventos, e)
	})
	if err != nil {
		t.Fatal(err)
	}
	ev, ap := a.sis.indice("evento Apagando", false), a.sis.indice("Apagar", false)
	if ev < 0 || ap < 0 || ev > ap {
		t.Fatalf("Apagando (%d) deveria vir antes do Apagar (%d): %v", ev, ap, a.sis.copia())
	}
	for i, e := range eventos {
		if e.Frase == fraseApagando {
			if e.Etapa != "copiando" || i == 0 || e.Fracao != eventos[i-1].Fracao {
				t.Fatalf("Apagando = %+v, anterior %+v", e, eventos[i-1])
			}
		}
		if e.Frase == fraseConferindoArquivos && e.Etapa != "baixando" {
			t.Fatalf("Conferindo os arquivos fora da etapa baixando: %+v", e)
		}
	}
}

func TestNaoCabeNaoApaga(t *testing.T) {
	a := novoAmbiente(t)
	// Tudo cabe em bytes, mas sem a margem de 64 MB.
	a.disco.Bytes = margemPenDrive
	a.sis.listas = [][]discos.Bruto{{{ID: "disk9", Nome: "Teste", Bytes: margemPenDrive}}}
	err := Gravar(context.Background(), a.opcoes(), nil)
	if got := fraseDe(t, err); got != fraseNaoCabe {
		t.Fatalf("frase = %q", got)
	}
	if a.sis.contem("Apagar") {
		t.Fatalf("apagou: %v", a.sis.copia())
	}
	if !cabe(100, 100+margemPenDrive) || cabe(101, 100+margemPenDrive) {
		t.Fatal("limite do cabe errado")
	}
}

// Se o instalador roda de dentro do pen drive, apagar o mata no meio.
func TestProprioPenDrive(t *testing.T) {
	a := novoAmbiente(t)
	montagem := t.TempDir()
	antigo := caminhoDoInstalador
	caminhoDoInstalador = func() string { return filepath.Join(montagem, "CD2-IA-de-Bolso-Windows.exe") }
	t.Cleanup(func() { caminhoDoInstalador = antigo })
	bruto := discos.Bruto{ID: "disk9", Nome: "Teste", Bytes: 32_000_000_000, Montagens: []string{montagem}}
	outro := discos.Bruto{ID: "disk8", Nome: "Outro", Bytes: 32_000_000_000, Montagens: []string{t.TempDir()}}
	a.sis.listas = [][]discos.Bruto{{bruto, outro}}

	l, err := Discos(context.Background(), a.sis)
	if err != nil {
		t.Fatal(err)
	}
	por := map[string]discos.Disco{}
	for _, d := range l {
		por[d.ID] = d
	}
	if por["disk9"].Situacao != discos.Proprio || por["disk8"].Situacao != discos.Ok {
		t.Fatalf("situações: %+v", l)
	}
	// Mesmo escolhido antes, a reconferência recusa sem apagar.
	err = Gravar(context.Background(), a.opcoes(), nil)
	if got := fraseDe(t, err); got != fraseProprio {
		t.Fatalf("frase = %q", got)
	}
	if a.sis.contem("Apagar") {
		t.Fatalf("apagou: %v", a.sis.copia())
	}
}
