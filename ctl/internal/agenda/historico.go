package agenda

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// ErrId é o erro de consultaId ou chave fora do formato aceito (nada de caminho com "..").
var ErrId = errors.New("agenda: id inválido")

// maxPagina é o maior n aceito em Pagina; bloco é o tamanho da leitura de trás para a frente.
const (
	maxPagina = 50
	bloco     = 64 << 10
)

var (
	reId    = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	reChave = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

// Execucao é uma linha do histórico. Tipo: "agendada" | "manual" | "perda". Rodada e Novo não
// vão para o disco: saem da leitura.
type Execucao struct {
	Seq       int64      `json:"seq"`
	Tipo      string     `json:"tipo"`
	Ref       time.Time  `json:"ref"` // horário de referência dos períodos
	Em        time.Time  `json:"em"`  // quando rodou (ou quando a perda foi achada)
	Valores   []string   `json:"valores,omitempty"`
	Resultado *Resultado `json:"resultado,omitempty"`
	Motivo    string     `json:"motivo,omitempty"` // perdas: fechada | dormindo | outro_banco | anterior
	De        *time.Time `json:"de,omitempty"`     // perdas: primeiro horário perdido
	Ate       *time.Time `json:"ate,omitempty"`    // perdas: último horário perdido (= Ref)
	N         int        `json:"n,omitempty"`      // perdas: quantos horários
	Rodada    *time.Time `json:"rodada,omitempty"` // perdas: preenchido na leitura a partir do estado
	Novo      bool       `json:"novo"`             // preenchido na leitura: agendada/perda com Seq > visto
}

// estado é o <consultaId>.estado.json: último seq, último visto e quando cada perda foi rodada.
type estado struct {
	Seq     int64             `json:"seq"`
	Visto   int64             `json:"visto"`
	Rodadas map[string]string `json:"rodadas"`
}

// Historico guarda, em <cache>/agenda/<chave>/, uma execução por linha de <consultaId>.jsonl
// (só acrescenta) mais o estado de cada consulta.
type Historico struct {
	dir string
	mu  sync.Mutex
}

// NovoHistorico cria o histórico em <cache>/agenda; as pastas só nascem na primeira gravação.
func NovoHistorico(cache string) *Historico {
	return &Historico{dir: filepath.Join(cache, "agenda")}
}

func (h *Historico) pasta(chave string) (string, error) {
	if !reChave.MatchString(chave) {
		return "", ErrId
	}
	return filepath.Join(h.dir, chave), nil
}

// caminhos valida chave e id e devolve o .jsonl e o .estado.json.
func (h *Historico) caminhos(chave, id string) (jsonl, est string, err error) {
	pasta, err := h.pasta(chave)
	if err != nil {
		return "", "", err
	}
	if !reId.MatchString(id) {
		return "", "", ErrId
	}
	return filepath.Join(pasta, id+".jsonl"), filepath.Join(pasta, id+".estado.json"), nil
}

func lerEstado(arq string) (estado, error) {
	var e estado
	raw, err := os.ReadFile(arq)
	if errors.Is(err, fs.ErrNotExist) {
		return e, nil
	}
	if err != nil {
		return e, err
	}
	err = json.Unmarshal(raw, &e)
	return e, err
}

func gravarEstado(arq string, e estado) error {
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return gravarAtomico(arq, raw)
}

// gravarAtomico troca o arquivo inteiro: temporário na mesma pasta, Sync e rename, como o
// notas.Store.Gravar.
func gravarAtomico(arq string, dados []byte) error {
	if err := os.MkdirAll(filepath.Dir(arq), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(arq), "tmp-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(dados)
	if err == nil {
		err = f.Sync()
	}
	if errFechar := f.Close(); err == nil {
		err = errFechar
	}
	if err == nil {
		err = os.Chmod(tmp, 0o600)
	}
	if err == nil {
		err = os.Rename(tmp, arq)
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// Anexar atribui Seq = estado.seq+1 e acrescenta a linha. O estado é gravado antes: se o
// acréscimo falhar sobra um buraco no Seq, nunca um Seq repetido.
func (h *Historico) Anexar(chave, id string, e Execucao) (Execucao, error) {
	arq, arqEst, err := h.caminhos(chave, id)
	if err != nil {
		return e, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	est, err := lerEstado(arqEst)
	if err != nil {
		return e, err
	}
	est.Seq++
	e.Seq = est.Seq
	e.Rodada, e.Novo = nil, false
	linha, err := json.Marshal(e)
	if err != nil {
		return e, err
	}
	if err := os.MkdirAll(filepath.Dir(arq), 0o700); err != nil {
		return e, err
	}
	if err := gravarEstado(arqEst, est); err != nil {
		return e, err
	}
	f, err := os.OpenFile(arq, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return e, err
	}
	// Linha cortada por uma queda: fecha com \n para a próxima não colar nela.
	var pre []byte
	if fi, err := f.Stat(); err == nil && fi.Size() > 0 {
		if u := make([]byte, 1); readAtFull(f, u, fi.Size()-1) == nil && u[0] != '\n' {
			pre = []byte{'\n'}
		}
	}
	_, err = f.Write(append(append(pre, linha...), '\n'))
	if err == nil {
		err = f.Sync()
	}
	if errFechar := f.Close(); err == nil {
		err = errFechar
	}
	return e, err
}

func readAtFull(f *os.File, b []byte, off int64) error {
	_, err := f.ReadAt(b, off)
	return err
}

// preencher põe Novo e Rodada, que não ficam no disco.
func (est estado) preencher(e *Execucao) {
	e.Novo = e.Tipo != "manual" && e.Seq > est.Visto
	if e.Tipo == "perda" {
		if s, ok := est.Rodadas[strconv.FormatInt(e.Seq, 10)]; ok {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				e.Rodada = &t
			}
		}
	}
}

// Pagina devolve até n execuções (no máximo 50), da mais nova para a mais velha, começando
// abaixo de antes (antes<=0: do fim). mais diz se sobrou histórico. O arquivo é lido de trás
// para a frente em blocos de 64 KB; linha que não é JSON válido é pulada.
func (h *Historico) Pagina(chave, id string, antes int64, n int) (itens []Execucao, mais bool, err error) {
	arq, arqEst, err := h.caminhos(chave, id)
	if err != nil {
		return nil, false, err
	}
	if n <= 0 || n > maxPagina {
		n = maxPagina
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	est, err := lerEstado(arqEst)
	if err != nil {
		return nil, false, err
	}
	f, err := os.Open(arq)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, false, err
	}
	pos := fi.Size()
	var resto []byte // começo de linha ainda sem o início
	// linha trata uma linha completa; devolve true quando já achou n+1.
	linha := func(b []byte) bool {
		var e Execucao
		if len(bytes.TrimSpace(b)) == 0 || json.Unmarshal(b, &e) != nil {
			return false
		}
		if antes > 0 && e.Seq >= antes {
			return false
		}
		if len(itens) == n {
			mais = true
			return true
		}
		est.preencher(&e)
		itens = append(itens, e)
		return false
	}
	for pos > 0 {
		tam := int64(bloco)
		if pos < tam {
			tam = pos
		}
		pos -= tam
		buf := make([]byte, tam+int64(len(resto)))
		if err := readAtFull(f, buf[:tam], pos); err != nil && !errors.Is(err, io.EOF) {
			return nil, false, err
		}
		copy(buf[tam:], resto)
		partes := bytes.Split(buf, []byte{'\n'})
		resto = partes[0]
		ate := 1 // partes[0] pode ser o fim de uma linha que começa no bloco anterior
		if pos == 0 {
			resto, ate = nil, 0
		}
		for i := len(partes) - 1; i >= ate; i-- {
			if linha(partes[i]) {
				return itens, mais, nil
			}
		}
	}
	return itens, mais, nil
}

// varrer lê o .jsonl de frente para trás, linha a linha, sem guardar o arquivo.
func (h *Historico) varrer(arq string, fn func(linha []byte)) error {
	f, err := os.Open(arq)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, bloco)
	for {
		b, err := r.ReadBytes('\n')
		if len(bytes.TrimSpace(b)) > 0 {
			fn(b)
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// Contar devolve quantas execuções válidas o histórico tem.
func (h *Historico) Contar(chave, id string) (int, error) {
	arq, _, err := h.caminhos(chave, id)
	if err != nil {
		return 0, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	err = h.varrer(arq, func(b []byte) {
		if json.Valid(b) {
			n++
		}
	})
	return n, err
}

// NaoVistas conta as agendadas e perdas com Seq > visto.
func (h *Historico) NaoVistas(chave, id string) (int, error) {
	arq, arqEst, err := h.caminhos(chave, id)
	if err != nil {
		return 0, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	est, err := lerEstado(arqEst)
	if err != nil {
		return 0, err
	}
	n := 0
	err = h.varrer(arq, func(b []byte) {
		var e struct {
			Seq  int64  `json:"seq"`
			Tipo string `json:"tipo"`
		}
		if json.Unmarshal(b, &e) == nil && e.Tipo != "manual" && e.Seq > est.Visto {
			n++
		}
	})
	return n, err
}

// MarcarVisto leva o visto até o último seq.
func (h *Historico) MarcarVisto(chave, id string) error {
	return h.alterar(chave, id, func(e *estado) { e.Visto = e.Seq })
}

// MarcarRodada anota quando a perda de número seq foi rodada.
func (h *Historico) MarcarRodada(chave, id string, seq int64, em time.Time) error {
	return h.alterar(chave, id, func(e *estado) {
		if e.Rodadas == nil {
			e.Rodadas = map[string]string{}
		}
		e.Rodadas[strconv.FormatInt(seq, 10)] = em.Format(time.RFC3339)
	})
}

func (h *Historico) alterar(chave, id string, fn func(*estado)) error {
	_, arqEst, err := h.caminhos(chave, id)
	if err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	est, err := lerEstado(arqEst)
	if err != nil {
		return err
	}
	fn(&est)
	return gravarEstado(arqEst, est)
}

// Apagar remove o histórico e o estado da consulta.
func (h *Historico) Apagar(chave, id string) error {
	arq, arqEst, err := h.caminhos(chave, id)
	if err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, a := range []string{arq, arqEst} {
		if err := os.Remove(a); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

type marca struct {
	Ate time.Time `json:"ate"`
}

func lerMarca(arq string) (time.Time, error) {
	raw, err := os.ReadFile(arq)
	if errors.Is(err, fs.ErrNotExist) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	var m marca
	if err := json.Unmarshal(raw, &m); err != nil {
		return time.Time{}, err
	}
	return m.Ate, nil
}

// Aberta devolve a última vez que a IA esteve aberta com este banco e com qualquer banco;
// zero se nunca.
func (h *Historico) Aberta(chave string) (banco, qualquer time.Time, err error) {
	pasta, err := h.pasta(chave)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if banco, err = lerMarca(filepath.Join(pasta, "aberta.json")); err != nil {
		return
	}
	qualquer, err = lerMarca(filepath.Join(h.dir, "aberta.json"))
	return
}

// MarcarAberta grava t nas duas marcas.
func (h *Historico) MarcarAberta(chave string, t time.Time) error {
	pasta, err := h.pasta(chave)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(marca{Ate: t})
	if err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := gravarAtomico(filepath.Join(pasta, "aberta.json"), raw); err != nil {
		return err
	}
	return gravarAtomico(filepath.Join(h.dir, "aberta.json"), raw)
}

// MarcarAbertaSemBanco grava t só na marca de qualquer banco: a IA aberta sem banco conectado.
func (h *Historico) MarcarAbertaSemBanco(t time.Time) error {
	raw, err := json.Marshal(marca{Ate: t})
	if err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return gravarAtomico(filepath.Join(h.dir, "aberta.json"), raw)
}
