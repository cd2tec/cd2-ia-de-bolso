package agenda

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"os/user"
	"strings"
	"time"
	"unicode/utf8"
)

// Auditoria das ações sugeridas pelo resumo (História 5, CIDB-21): o que foi proposto, o que a pessoa
// decidiu e o que aconteceu, com data e hora. Só se acrescenta: nenhum evento é alterado nem apagado (nem
// quando o histórico da consulta é apagado). A execução é identificada por chave, consulta, seq e o
// horário em que rodou (exec_em): o seq recomeça depois de apagar o histórico, o horário não se repete.
const esquemaAuditoria = `
CREATE TABLE IF NOT EXISTS auditoria (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	chave TEXT NOT NULL, consulta TEXT NOT NULL, seq INTEGER NOT NULL, exec_em TEXT NOT NULL,
	evento TEXT NOT NULL, acao TEXT NOT NULL, em TEXT NOT NULL, por TEXT NOT NULL, detalhe TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS auditoria_execucao ON auditoria (chave, consulta, seq, exec_em);
`

// EventoAcao é uma linha da auditoria. Evento: "proposta" (gravada pelo agendador, com a execução) |
// "aprovada" | "recusada" (o clique da pessoa) | "executada" | "falhou" (o que a página conseguiu fazer).
// Por: quem; na proposta, "IA de Bolso"; nos demais, o usuário do sistema operacional.
type EventoAcao struct {
	Evento  string    `json:"evento"`
	Acao    string    `json:"acao"`
	Em      time.Time `json:"em"`
	Por     string    `json:"por"`
	Detalhe string    `json:"detalhe,omitempty"`
}

const (
	porIA         = "IA de Bolso"
	maxDetalheAud = 300
)

var (
	// ErrSemProposta: a execução não existe ou não sugeriu essa ação.
	ErrSemProposta = errors.New("agenda: a execução não tem essa ação sugerida")
	// ErrDecidida: a proposta já foi aprovada ou recusada.
	ErrDecidida = errors.New("agenda: a ação já foi decidida")
	// ErrSemAprovacao: resultado (executada/falhou) sem aprovação registrada, ou registrado de novo.
	ErrSemAprovacao = errors.New("agenda: a ação não foi aprovada")
	// ErrEvento: evento fora de aprovada | recusada | executada | falhou.
	ErrEvento = errors.New("agenda: evento inválido")
)

// usuarioAtual é quem decide, para a auditoria: o usuário do sistema operacional (o app não tem login).
var usuarioAtual = func() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	for _, v := range []string{"USER", "USERNAME"} {
		if s := os.Getenv(v); s != "" {
			return s
		}
	}
	return "desconhecido"
}

// agoraAud é o relógio da auditoria (os testes trocam).
var agoraAud = time.Now

func emAud(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func cortarDetalhe(s string) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= maxDetalheAud {
		return s
	}
	return string([]rune(s)[:maxDetalheAud-1]) + "…"
}

// proposta grava o evento "proposta" da execução recém-anexada (na mesma transação do Anexar).
func proposta(tx *sql.Tx, chave, id string, e Execucao) error {
	if e.Resumo == nil || e.Resumo.Acao == "" {
		return nil
	}
	_, err := tx.Exec(`INSERT INTO auditoria (chave, consulta, seq, exec_em, evento, acao, em, por, detalhe)
		VALUES (?, ?, ?, ?, 'proposta', ?, ?, ?, '')`, chave, id, e.Seq, emAud(e.Em), e.Resumo.Acao, emAud(e.Em), porIA)
	return err
}

type consultor interface {
	Query(q string, args ...any) (*sql.Rows, error)
}

func eventos(db consultor, chave, id string, seq int64, execEm string) ([]EventoAcao, error) {
	rows, err := db.Query(`SELECT evento, acao, em, por, detalhe FROM auditoria
		WHERE chave = ? AND consulta = ? AND seq = ? AND exec_em = ? ORDER BY id`, chave, id, seq, execEm)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EventoAcao
	for rows.Next() {
		var ev EventoAcao
		var em string
		if err := rows.Scan(&ev.Evento, &ev.Acao, &em, &ev.Por, &ev.Detalhe); err != nil {
			return nil, err
		}
		ev.Em, _ = time.Parse(time.RFC3339Nano, em)
		out = append(out, ev)
	}
	return out, rows.Err()
}

// Auditar registra a decisão da pessoa (aprovada | recusada) ou o resultado da ação aprovada (executada |
// falhou) da execução seq que rodou em em. As regras garantem que nenhuma ação roda sem confirmação
// registrada: decidir só o que foi proposto, uma vez; resultado só depois de aprovada, uma vez. Devolve
// a auditoria da execução, já com o evento novo.
func (h *Historico) Auditar(chave, id string, seq int64, em time.Time, evento, acao, detalhe string) ([]EventoAcao, error) {
	switch evento {
	case "aprovada", "recusada", "executada", "falhou":
	default:
		return nil, ErrEvento
	}
	db, err := h.usar(chave, id)
	if err != nil {
		return nil, err
	}
	defer h.mu.Unlock()
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var raw string
	err = tx.QueryRow(`SELECT json FROM execucoes WHERE chave = ? AND consulta = ? AND seq = ?`, chave, id, seq).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSemProposta
	}
	if err != nil {
		return nil, err
	}
	var e Execucao
	if json.Unmarshal([]byte(raw), &e) != nil || !e.Em.Equal(em) || e.Resumo == nil || e.Resumo.Acao == "" || e.Resumo.Acao != acao {
		return nil, ErrSemProposta
	}
	execEm := emAud(e.Em)
	evs, err := eventos(tx, chave, id, seq, execEm)
	if err != nil {
		return nil, err
	}
	tem := func(nomes ...string) bool {
		for _, ev := range evs {
			for _, n := range nomes {
				if ev.Evento == n {
					return true
				}
			}
		}
		return false
	}
	switch evento {
	case "aprovada", "recusada":
		if tem("aprovada", "recusada") {
			return nil, ErrDecidida
		}
	default:
		if !tem("aprovada") || tem("executada", "falhou") {
			return nil, ErrSemAprovacao
		}
	}
	ev := EventoAcao{Evento: evento, Acao: acao, Em: agoraDe(agoraAud()), Por: usuarioAtual(), Detalhe: cortarDetalhe(detalhe)}
	if _, err := tx.Exec(`INSERT INTO auditoria (chave, consulta, seq, exec_em, evento, acao, em, por, detalhe)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, chave, id, seq, execEm, ev.Evento, ev.Acao, emAud(ev.Em), ev.Por, ev.Detalhe); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return append(evs, ev), nil
}
