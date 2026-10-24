package agenda

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// ErrId é o erro de consultaId ou chave fora do formato aceito (nada de caminho com "..").
var ErrId = errors.New("agenda: id inválido")

const (
	maxPagina     = 50 // maior n aceito em Pagina
	versaoEsquema = 1  // PRAGMA user_version do historico.sqlite
	arqHistorico  = "historico.sqlite"
)

var (
	reId    = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	reChave = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

// Execucao é uma linha do histórico. Tipo: "agendada" | "manual" | "perda". Rodada e Novo não
// vão para o banco: saem da leitura.
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

// esquema v1. execucoes.json é a Execucao sem Rodada e Novo (e sem Linhas quando igual a Todas);
// estado.seq é o último seq dado (não volta quando a última execução some); aberta com chave
// vazia é "qualquer banco".
const esquema = `
CREATE TABLE IF NOT EXISTS execucoes (
	chave TEXT NOT NULL, consulta TEXT NOT NULL, seq INTEGER NOT NULL, tipo TEXT NOT NULL, json TEXT NOT NULL,
	PRIMARY KEY (chave, consulta, seq)
) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS estado (
	chave TEXT NOT NULL, consulta TEXT NOT NULL, seq INTEGER NOT NULL, visto INTEGER NOT NULL,
	PRIMARY KEY (chave, consulta)
) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS rodadas (
	chave TEXT NOT NULL, consulta TEXT NOT NULL, seq INTEGER NOT NULL, em TEXT NOT NULL,
	PRIMARY KEY (chave, consulta, seq)
) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS aberta (chave TEXT NOT NULL PRIMARY KEY, ate TEXT NOT NULL) WITHOUT ROWID;
`

// Historico guarda as execuções de todos os bancos em <dados>/historico.sqlite. O banco abre na
// primeira chamada; pen drive e instalada podem ter o mesmo arquivo aberto (WAL + busy_timeout).
type Historico struct {
	dir    string
	avisos io.Writer // aviso de banco corrompido; nil: os.Stderr
	mu     sync.Mutex
	db     *sql.DB
}

// NovoHistorico cria o histórico em <dados>/historico.sqlite; pasta e arquivo só nascem no primeiro uso.
func NovoHistorico(dados string) *Historico {
	return &Historico{dir: dados}
}

// Fechar fecha o banco; a próxima chamada abre de novo.
func (h *Historico) Fechar() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.db == nil {
		return nil
	}
	err := h.db.Close()
	h.db = nil
	return err
}

// banco devolve o banco aberto, abrindo na primeira vez (com h.mu travado). Banco corrompido é
// guardado de lado e outro começa; outro erro volta e a próxima chamada tenta de novo.
func (h *Historico) banco() (*sql.DB, error) {
	if h.db != nil {
		return h.db, nil
	}
	arq := filepath.Join(h.dir, arqHistorico)
	db, err := abrir(arq)
	if err != nil && corrompido(err) {
		db, err = h.recriar(arq, err)
	}
	if err != nil {
		return nil, err
	}
	h.db = db
	return db, nil
}

func abrir(arq string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(arq), 0o700); err != nil {
		return nil, err
	}
	// O SQLite criaria com 0644: o arquivo nasce antes, vazio (já é um banco válido), com 0600.
	f, err := os.OpenFile(arq, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	f.Close()
	// immediate: o BEGIN já pega a escrita, e o outro cd2-ctl espera no busy_timeout em vez de
	// falhar no meio da transação.
	db, err := sql.Open("sqlite", arq+"?_pragma=busy_timeout(5000)&_txlock=immediate")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // h.mu já serializa; uma conexão só, sempre com os pragmas acima
	if err := preparar(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// corrupcao é o quick_check que não deu "ok".
type corrupcao string

func (c corrupcao) Error() string { return "quick_check: " + string(c) }

func preparar(db *sql.DB) error {
	var r string
	if err := db.QueryRow(`PRAGMA quick_check`).Scan(&r); err != nil {
		return err
	}
	if r != "ok" {
		return corrupcao(r)
	}
	if err := modoWAL(db); err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var v int
	if err := tx.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return err
	}
	if v >= versaoEsquema {
		return nil
	}
	if _, err := tx.Exec(esquema); err != nil {
		return err
	}
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, versaoEsquema)); err != nil {
		return err
	}
	return tx.Commit()
}

// modoWAL liga o WAL. O busy_timeout não vale para a troca do journal_mode (o outro cd2-ctl
// abrindo junto dá SQLITE_BUSY na hora): tenta de novo por até 5 s.
func modoWAL(db *sql.DB) error {
	fim := time.Now().Add(5 * time.Second)
	for {
		var modo string
		err := db.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&modo)
		if err == nil && modo != "wal" {
			err = fmt.Errorf("journal_mode %s", modo)
		}
		var e *sqlite.Error
		if err == nil || !errors.As(err, &e) || e.Code()&0xff != sqlite3.SQLITE_BUSY || time.Now().After(fim) {
			return err
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// corrompido diz se o erro é de arquivo estragado (e não de trava, disco cheio ou permissão,
// que não justificam jogar o histórico de lado).
func corrompido(err error) bool {
	var c corrupcao
	if errors.As(err, &c) {
		return true
	}
	var e *sqlite.Error
	if errors.As(err, &e) {
		switch e.Code() & 0xff {
		case sqlite3.SQLITE_CORRUPT, sqlite3.SQLITE_NOTADB:
			return true
		}
	}
	return false
}

// recriar guarda o banco corrompido (e o -wal/-shm) como .corrompido-<quando> e abre um novo.
func (h *Historico) recriar(arq string, causa error) (*sql.DB, error) {
	sufixo := ".corrompido-" + time.Now().Format("20060102-150405")
	for _, s := range []string{"", "-wal", "-shm"} {
		if err := os.Rename(arq+s, arq+s+sufixo); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	w := h.avisos
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprintf(w, "cd2-ctl: historico dos agendamentos corrompido (%v); guardado em %s, comecando outro\n", causa, arq+sufixo)
	return abrir(arq)
}

func validar(chave, id string) error {
	if !reChave.MatchString(chave) || !reId.MatchString(id) {
		return ErrId
	}
	return nil
}

// usar valida chave e id, trava e devolve o banco aberto; quem chama destrava.
func (h *Historico) usar(chave, id string) (*sql.DB, error) {
	if err := validar(chave, id); err != nil {
		return nil, err
	}
	h.mu.Lock()
	db, err := h.banco()
	if err != nil {
		h.mu.Unlock()
	}
	return db, err
}

// paraDisco tira Linhas quando é igual a Todas (o comum: as mesmas até 200 linhas duas vezes);
// a leitura refaz com refazer. Não mexe no Resultado de quem chamou.
func paraDisco(e Execucao) Execucao {
	r := e.Resultado
	if r == nil || !r.Ok || r.Linhas == nil || len(r.Linhas) != len(r.Todas) {
		return e
	}
	l, err1 := json.Marshal(r.Linhas)
	t, err2 := json.Marshal(r.Todas)
	if err1 != nil || err2 != nil || !bytes.Equal(l, t) {
		return e
	}
	c := *r
	c.Linhas = nil
	e.Resultado = &c
	return e
}

// refazer põe de volta Linhas = Todas na execução lida do disco (o contrato com a tela não muda).
func refazer(e *Execucao) {
	if r := e.Resultado; r != nil && r.Ok && r.Linhas == nil && r.Todas != nil {
		r.Linhas = r.Todas
	}
}

// Anexar atribui Seq = estado.seq+1 e grava a execução, tudo numa transação.
func (h *Historico) Anexar(chave, id string, e Execucao) (Execucao, error) {
	db, err := h.usar(chave, id)
	if err != nil {
		return e, err
	}
	defer h.mu.Unlock()
	e.Rodada, e.Novo = nil, false
	tx, err := db.Begin()
	if err != nil {
		return e, err
	}
	defer tx.Rollback()
	var seq int64
	err = tx.QueryRow(`SELECT seq FROM estado WHERE chave = ? AND consulta = ?`, chave, id).Scan(&seq)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return e, err
	}
	e.Seq = seq + 1
	linha, err := json.Marshal(paraDisco(e))
	if err != nil {
		return e, err
	}
	if _, err := tx.Exec(`INSERT INTO estado (chave, consulta, seq, visto) VALUES (?, ?, ?, 0)
		ON CONFLICT (chave, consulta) DO UPDATE SET seq = excluded.seq`, chave, id, e.Seq); err != nil {
		return e, err
	}
	if _, err := tx.Exec(`INSERT INTO execucoes (chave, consulta, seq, tipo, json) VALUES (?, ?, ?, ?, ?)`,
		chave, id, e.Seq, e.Tipo, string(linha)); err != nil {
		return e, err
	}
	return e, tx.Commit()
}

// Pagina devolve até n execuções (no máximo 50), da mais nova para a mais velha, começando
// abaixo de antes (antes<=0: do fim). mais diz se sobrou histórico. Novo e Rodada saem do estado.
func (h *Historico) Pagina(chave, id string, antes int64, n int) (itens []Execucao, mais bool, err error) {
	if n <= 0 || n > maxPagina {
		n = maxPagina
	}
	db, err := h.usar(chave, id)
	if err != nil {
		return nil, false, err
	}
	defer h.mu.Unlock()
	rows, err := db.Query(`SELECT x.seq, x.json, r.em,
			COALESCE((SELECT visto FROM estado s WHERE s.chave = x.chave AND s.consulta = x.consulta), 0)
		FROM execucoes x
		LEFT JOIN rodadas r ON r.chave = x.chave AND r.consulta = x.consulta AND r.seq = x.seq
		WHERE x.chave = ? AND x.consulta = ? AND (? <= 0 OR x.seq < ?)
		ORDER BY x.seq DESC LIMIT ?`, chave, id, antes, antes, n+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			seq, visto int64
			raw        string
			em         sql.NullString
		)
		if err := rows.Scan(&seq, &raw, &em, &visto); err != nil {
			return nil, false, err
		}
		if len(itens) == n {
			mais = true
			break
		}
		var e Execucao
		if json.Unmarshal([]byte(raw), &e) != nil {
			continue
		}
		e.Seq = seq
		e.Novo = e.Tipo != "manual" && e.Seq > visto
		if e.Tipo == "perda" && em.Valid {
			if t, err := time.Parse(time.RFC3339, em.String); err == nil {
				e.Rodada = &t
			}
		}
		refazer(&e)
		itens = append(itens, e)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	return itens, mais, nil
}

// Contar devolve quantas execuções o histórico da consulta tem.
func (h *Historico) Contar(chave, id string) (int, error) {
	db, err := h.usar(chave, id)
	if err != nil {
		return 0, err
	}
	defer h.mu.Unlock()
	var n int
	err = db.QueryRow(`SELECT COUNT(*) FROM execucoes WHERE chave = ? AND consulta = ?`, chave, id).Scan(&n)
	return n, err
}

// NaoVistas conta as agendadas e perdas com Seq > visto (as mesmas que Pagina marca Novo).
func (h *Historico) NaoVistas(chave, id string) (int, error) {
	db, err := h.usar(chave, id)
	if err != nil {
		return 0, err
	}
	defer h.mu.Unlock()
	var n int
	err = db.QueryRow(`SELECT COUNT(*) FROM execucoes
		WHERE chave = ? AND consulta = ? AND tipo <> 'manual'
		AND seq > COALESCE((SELECT visto FROM estado WHERE chave = ? AND consulta = ?), 0)`,
		chave, id, chave, id).Scan(&n)
	return n, err
}

// MarcarVisto leva o visto até o último seq.
func (h *Historico) MarcarVisto(chave, id string) error {
	db, err := h.usar(chave, id)
	if err != nil {
		return err
	}
	defer h.mu.Unlock()
	_, err = db.Exec(`UPDATE estado SET visto = seq WHERE chave = ? AND consulta = ?`, chave, id)
	return err
}

// MarcarRodada anota quando a perda de número seq foi rodada.
func (h *Historico) MarcarRodada(chave, id string, seq int64, em time.Time) error {
	db, err := h.usar(chave, id)
	if err != nil {
		return err
	}
	defer h.mu.Unlock()
	_, err = db.Exec(`INSERT INTO rodadas (chave, consulta, seq, em) VALUES (?, ?, ?, ?)
		ON CONFLICT (chave, consulta, seq) DO UPDATE SET em = excluded.em`, chave, id, seq, em.Format(time.RFC3339))
	return err
}

// Apagar remove o histórico e o estado da consulta; o seq recomeça.
func (h *Historico) Apagar(chave, id string) error {
	db, err := h.usar(chave, id)
	if err != nil {
		return err
	}
	defer h.mu.Unlock()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, tab := range []string{"execucoes", "estado", "rodadas"} {
		if _, err := tx.Exec(`DELETE FROM `+tab+` WHERE chave = ? AND consulta = ?`, chave, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Aberta devolve a última vez que a IA esteve aberta com este banco e com qualquer banco;
// zero se nunca.
func (h *Historico) Aberta(chave string) (banco, qualquer time.Time, err error) {
	db, err := h.usar(chave, "x")
	if err != nil {
		return
	}
	defer h.mu.Unlock()
	rows, err := db.Query(`SELECT chave, ate FROM aberta WHERE chave IN (?, '')`, chave)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var c, ate string
		if err = rows.Scan(&c, &ate); err != nil {
			return
		}
		t, errT := time.Parse(time.RFC3339Nano, ate)
		if errT != nil {
			continue
		}
		if c == "" {
			qualquer = t
		} else {
			banco = t
		}
	}
	err = rows.Err()
	return
}

// MarcarAberta grava t nas duas marcas.
func (h *Historico) MarcarAberta(chave string, t time.Time) error {
	db, err := h.usar(chave, "x")
	if err != nil {
		return err
	}
	defer h.mu.Unlock()
	return marcar(db, t, chave, "")
}

// MarcarAbertaSemBanco grava t só na marca de qualquer banco: a IA aberta sem banco conectado.
func (h *Historico) MarcarAbertaSemBanco(t time.Time) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	db, err := h.banco()
	if err != nil {
		return err
	}
	return marcar(db, t, "")
}

func marcar(db *sql.DB, t time.Time, chaves ...string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, c := range chaves {
		if _, err := tx.Exec(`INSERT INTO aberta (chave, ate) VALUES (?, ?)
			ON CONFLICT (chave) DO UPDATE SET ate = excluded.ate`, c, t.Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
