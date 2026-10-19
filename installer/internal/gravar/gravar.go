// Package gravar liga os pacotes do instalador na ordem fixa da gravação:
// baixar tudo, conferir, só então apagar o pen drive, copiar e conferir.
package gravar

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/discos"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/manifesto"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/pendrive"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/preparo"
)

// URLManifesto é onde o manifesto.json é publicado.
const URLManifesto = "https://github.com/cd2tec/cd2-ia-de-bolso/releases/download/iadebolso/manifesto.json"

// rotuloDisco é o rótulo do volume gravado, o mesmo dos scripts.
const rotuloDisco = "CD2"

// sistemaOperacional existe para o teste simular o Windows.
var sistemaOperacional = runtime.GOOS

// margemPenDrive é a folga, além do tamanho do stage, que o pen drive
// precisa ter: o exFAT e a tabela de partição ocupam um pouco.
const margemPenDrive = 64 << 20

// cabe diz se um stage de total bytes cabe num pen drive de bytes, com a margem.
func cabe(total, bytes int64) bool { return total+margemPenDrive <= bytes }

// caminhoDoInstalador devolve o executável em uso, sem links simbólicos, ou
// "" se não souber. O teste troca.
var caminhoDoInstalador = func() string {
	p, err := os.Executable()
	if err != nil {
		return ""
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	return p
}

// errProprio: o disco escolhido é o pen drive de onde o instalador roda.
var errProprio = errors.New("o instalador roda deste disco")

// Evento é o andamento da gravação, como a tela recebe.
type Evento struct {
	Etapa    string  `json:"etapa"` // "baixando" | "copiando" | "conferindo" | "pronto"
	Frase    string  `json:"frase"`
	Fracao   float64 `json:"fracao"`   // 0..1 da gravação inteira
	Restante int     `json:"restante"` // segundos estimados; -1 se ainda não dá para estimar
}

// Erro é o erro que a tela mostra: Frase já está pronta para a pessoa.
type Erro struct {
	Frase string
	Causa error
}

func (e *Erro) Error() string { return e.Frase }
func (e *Erro) Unwrap() error { return e.Causa }

// Opcoes reúne o que Gravar precisa.
type Opcoes struct {
	Disco        discos.Disco
	Cache        string
	URLManifesto string
	HTTP         *http.Client
	Sistema      discos.Sistema
	Agora        func() time.Time // relógio; time.Now quando nil
}

// PastaCache devolve a mesma pasta de cache dos scripts.
func PastaCache() (string, error) {
	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			return "", errors.New("LOCALAPPDATA vazio")
		}
		return filepath.Join(base, "cd2-harness", "usb"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Caches", "cd2-harness", "usb"), nil
}

// Discos lista os discos que a tela pode mostrar: Listar, Filtrar, Descrever.
// O disco de onde o instalador está rodando aparece como Proprio, que não
// pode ser escolhido. Sem disco, devolve lista vazia (nunca nil), para o JSON
// sair [].
func Discos(ctx context.Context, s discos.Sistema) ([]discos.Disco, error) {
	brutos, err := s.Listar(ctx)
	if err != nil {
		return nil, err
	}
	exe := caminhoDoInstalador()
	saida := []discos.Disco{}
	for _, b := range discos.Filtrar(brutos) {
		d := discos.Descrever(b, discos.ContarArquivos)
		if discos.NasMontagens(b.Montagens, exe) {
			d.Situacao = discos.Proprio
		}
		saida = append(saida, d)
	}
	return saida, nil
}

type amostra struct {
	t time.Time
	b int64
}

const (
	esperaEstimativa = 3 * time.Second
	janelaVazao      = 10 * time.Second
	passoMinimo      = 0.001
)

// andamento calcula fração e tempo restante e emite os eventos.
type andamento struct {
	agora func() time.Time
	ev    func(Evento)
	// pesos em bytes: o que falta baixar e o total da cópia (que conta duas
	// vezes: copiar e conferir).
	baixar, copiar int64
	// copiaUmaVez: a cópia pesa uma vez só (instalar no computador, onde a
	// conferência acontece dentro da própria cópia).
	copiaUmaVez  bool
	ultima       float64 // maior fração já calculada
	etapa        string
	etapaEmitida string
	inicio       time.Time
	amostras     []amostra // janela (instante, bytes feitos) da etapa atual
	ultimaFrase  string
	emitiu       bool
	emitida      float64
}

// publicar emite um evento. feitos/total são da etapa; base é o que as
// etapas anteriores já pesaram. A fração nunca diminui.
func (a *andamento) publicar(etapa, frase string, feitos, total, base int64, forcar bool) {
	var f float64
	peso := a.baixar + 2*a.copiar
	if a.copiaUmaVez {
		peso = a.baixar + a.copiar
	}
	if peso > 0 {
		f = float64(base+feitos) / float64(peso)
	}
	if f > 0.999 {
		f = 0.999 // 1 é só do evento pronto
	}
	if f < a.ultima {
		f = a.ultima
	}
	a.ultima = f

	agora := a.agora()
	if etapa != a.etapa {
		a.etapa, a.inicio, a.amostras = etapa, agora, nil
	}
	a.amostras = append(a.amostras, amostra{agora, feitos})
	for len(a.amostras) > 2 && agora.Sub(a.amostras[0].t) > janelaVazao {
		a.amostras = a.amostras[1:]
	}

	forcar = forcar || etapa != a.etapaEmitida || frase != a.ultimaFrase
	a.etapaEmitida, a.ultimaFrase = etapa, frase
	if !forcar && a.emitiu && f-a.emitida < passoMinimo {
		return
	}
	a.emitiu, a.emitida = true, f
	a.ev(Evento{Etapa: etapa, Frase: frase, Fracao: f, Restante: a.restante(agora, feitos, total)})
}

// restante estima os segundos que faltam na etapa atual, pela vazão da
// janela recente. -1 nos primeiros 3 s da etapa ou sem vazão medida.
func (a *andamento) restante(agora time.Time, feitos, total int64) int {
	if total <= 0 || agora.Sub(a.inicio) < esperaEstimativa || len(a.amostras) < 2 {
		return -1
	}
	p, u := a.amostras[0], a.amostras[len(a.amostras)-1]
	dt := u.t.Sub(p.t).Seconds()
	db := u.b - p.b
	if dt <= 0 || db <= 0 {
		return -1
	}
	falta := total - feitos
	if falta < 0 {
		falta = 0
	}
	return int(float64(falta)/(float64(db)/dt) + 0.5)
}

// parado emite uma frase sem a fração andar, para uma fase que não mede
// bytes (o Apagar). A etapa seguinte recomeça a medir a vazão do zero.
func (a *andamento) parado(etapa, frase string) {
	a.etapa, a.amostras = "", nil
	a.etapaEmitida, a.ultimaFrase = etapa, frase
	a.emitiu, a.emitida = true, a.ultima
	a.ev(Evento{Etapa: etapa, Frase: frase, Fracao: a.ultima, Restante: -1})
}

func (a *andamento) pronto(frase string) {
	a.ev(Evento{Etapa: "pronto", Frase: frase, Fracao: 1, Restante: 0})
}

// Gravar baixa, confere, apaga o pen drive escolhido e grava. Todo erro
// devolvido é *Erro.
func Gravar(ctx context.Context, o Opcoes, ev func(Evento)) error {
	if ev == nil {
		ev = func(Evento) {}
	}
	agora := o.Agora
	if agora == nil {
		agora = time.Now
	}
	url := o.URLManifesto
	if url == "" {
		url = URLManifesto
	}
	apagou := false
	// falha devolve a frase de cancelamento quando o contexto caiu.
	falha := func(frase string, causa error) error {
		if ctx.Err() != nil {
			if apagou {
				return &Erro{Frase: fraseCancelouApos, Causa: causa}
			}
			return &Erro{Frase: fraseCancelouAntes, Causa: causa}
		}
		return &Erro{Frase: frase, Causa: causa}
	}

	m, daCache, err := manifesto.Obter(ctx, o.HTTP, url, o.Cache)
	if err != nil {
		return falha(frasePreparo(err), err)
	}

	a := &andamento{agora: agora, ev: ev}
	a.baixar = preparo.BytesParaBaixar(m, o.Cache)
	// Antes de ver o stage, estima a cópia pelos tamanhos que o manifesto conhece.
	a.copiar = m.Zip.Bytes + m.Llamafile.Bytes + m.Modelo.Bytes
	if a.copiar < a.baixar {
		a.copiar = a.baixar
	}

	itens := map[string]int64{}
	stage, err := preparo.Montar(ctx, o.HTTP, m, o.Cache, daCache, func(e preparo.Evento) {
		if !e.Conferindo {
			itens[e.Item] = e.Feitos
		}
		var baixados int64
		for _, v := range itens {
			baixados += v
		}
		if baixados > a.baixar {
			baixados = a.baixar
		}
		frase := fraseConferindoArquivos
		if !e.Conferindo {
			frase = fraseBaixando(e.Item, e.Feitos, e.Total)
		}
		a.publicar("baixando", frase, baixados, a.baixar, 0, false)
	})
	if err != nil {
		return falha(frasePreparo(err), err)
	}

	if sistemaOperacional == "windows" {
		exe := filepath.Join(stage, "bin", "llamafile.exe")
		if st, err := os.Stat(exe); err != nil || st.Size() == 0 {
			// A tela não mostra caminho de arquivo; a pasta vai só para o log.
			log.Printf("gravar: %s sumiu ou zerou (antivírus?), pasta para liberar: %s", exe, o.Cache)
			return falha(fraseAntivirus, fmt.Errorf("llamafile.exe sumiu ou zerou: %v", err))
		}
	}

	total, err := pendrive.TamanhoTotal(stage)
	if err != nil {
		return falha(fraseGenerica, err)
	}
	a.copiar = total
	if !cabe(total, o.Disco.Bytes) {
		return falha(fraseNaoCabe, fmt.Errorf("o stage tem %d bytes e o pen drive %d", total, o.Disco.Bytes))
	}

	// Reconfere o disco escolhido imediatamente antes de apagar.
	if err := ctx.Err(); err != nil {
		return falha("", err)
	}
	if err := reconferir(ctx, o); err != nil {
		if errors.Is(err, errProprio) {
			return falha(fraseProprio, err)
		}
		return falha(fraseDiscoMudou, err)
	}
	// Cancelar nesta janela ainda não apagou nada.
	if err := ctx.Err(); err != nil {
		return falha("", err)
	}

	// Daqui em diante o disco pode estar apagado, mesmo se Apagar falhar ou
	// for cancelado no meio.
	apagou = true
	a.parado("copiando", fraseApagando)
	montagem, err := o.Sistema.Apagar(ctx, o.Disco.ID, rotuloDisco)
	if err != nil {
		return falha(fraseFormatar, err)
	}
	a.publicar("copiando", fraseCopiando, 0, total, a.baixar, true)
	if err := pendrive.Copiar(ctx, stage, montagem, func(f, t int64) {
		a.publicar("copiando", fraseCopiando, f, t, a.baixar, false)
	}); err != nil {
		return falha(fraseGravacao(err), err)
	}
	if err := pendrive.Conferir(ctx, montagem, func(f, t int64) {
		a.publicar("conferindo", fraseConferindo, f, t, a.baixar+a.copiar, false)
	}); err != nil {
		return falha(fraseGravacao(err), err)
	}
	// Guardar os checksums na cache não é erro: o pen drive já foi conferido.
	_, _ = pendrive.GuardarChecksums(montagem, o.Cache, agora())
	if err := o.Sistema.Ejetar(ctx, o.Disco.ID, montagem); err != nil {
		a.pronto(fraseProntoSem)
		return nil
	}
	a.pronto(fraseProntoOk)
	return nil
}

// reconferir lista os discos de novo e exige o mesmo disco, ainda escolhível
// e sem ser o pen drive de onde o instalador roda.
func reconferir(ctx context.Context, o Opcoes) error {
	lista, err := Discos(ctx, o.Sistema)
	if err != nil {
		return err
	}
	for _, d := range lista {
		if d.ID != o.Disco.ID {
			continue
		}
		if !discos.Mesmo(d, o.Disco) {
			return fmt.Errorf("disco %s mudou (nome ou tamanho)", d.ID)
		}
		if d.Situacao == discos.Pequeno {
			return fmt.Errorf("disco %s pequeno demais", d.ID)
		}
		if d.Situacao == discos.Proprio {
			return fmt.Errorf("disco %s: %w", d.ID, errProprio)
		}
		return nil
	}
	return fmt.Errorf("disco %s não está mais lá", o.Disco.ID)
}
