// Package novidades pergunta ao cd2-ctl o que os agendamentos trouxeram de
// novo e monta os textos da bandeja. Não conhece a bandeja: o teste roda sem
// ela.
package novidades

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// MaxLinhas é quantas consultas cabem no menu da bandeja.
const MaxLinhas = 5

// Estado é a resposta do GET /agenda/novidades. Total soma os N das linhas;
// Ultimo é o maior seq de aviso que o ctl já deu; Agenda diz se os
// agendamentos rodam nesta cópia.
type Estado struct {
	Total  int
	Linhas []Linha
	Avisos []Aviso
	Ultimo int64
	Agenda bool
}

// Linha é uma consulta com N execuções ainda não vistas.
type Linha struct {
	Consulta, Nome string
	N              int
}

// Aviso é uma notificação do agendador. Consulta vazia é o aviso único das
// perdas ao abrir.
type Aviso struct {
	Seq                     int64
	Consulta, Titulo, Texto string
}

// resposta tem o formato do JSON do ctl.
type resposta struct {
	Avisos []struct {
		Seq      int64  `json:"seq"`
		Consulta string `json:"consulta"`
		Titulo   string `json:"titulo"`
		Texto    string `json:"texto"`
	} `json:"avisos"`
	Ultimo    int64 `json:"ultimo"`
	Novidades []struct {
		Consulta string `json:"consulta"`
		Nome     string `json:"nome"`
		N        int    `json:"n"`
	} `json:"novidades"`
	Total  int  `json:"total"`
	Agenda bool `json:"agenda"`
}

// Perguntar faz o GET /agenda/novidades?desde=<desde> no ctl, com a chave no
// Authorization. É GET: o ctl não exige Origin. O Host sai 127.0.0.1:<porta>
// da própria URL.
func Perguntar(ctx context.Context, porta int, chave string, desde int64) (Estado, error) {
	url := "http://127.0.0.1:" + strconv.Itoa(porta) + "/agenda/novidades?desde=" + strconv.FormatInt(desde, 10)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Estado{}, err
	}
	req.Header.Set("Authorization", "Bearer "+chave)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return Estado{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Estado{}, fmt.Errorf("novidades: status %d", res.StatusCode)
	}
	var r resposta
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return Estado{}, err
	}
	e := Estado{Total: r.Total, Ultimo: r.Ultimo, Agenda: r.Agenda}
	for _, n := range r.Novidades {
		e.Linhas = append(e.Linhas, Linha{Consulta: n.Consulta, Nome: n.Nome, N: n.N})
	}
	for _, a := range r.Avisos {
		e.Avisos = append(e.Avisos, Aviso{Seq: a.Seq, Consulta: a.Consulta, Titulo: a.Titulo, Texto: a.Texto})
	}
	return e, nil
}

// TextosMenu monta o topo ("3 resultados novos") e uma linha por consulta
// ("⏰ Vendas (2)"), até MaxLinhas. Sem nada novo, topo vazio e sem linhas.
func TextosMenu(e Estado) (topo string, linhas []string) {
	if e.Total <= 0 {
		return "", nil
	}
	topo = strconv.Itoa(e.Total) + " resultados novos"
	if e.Total == 1 {
		topo = "1 resultado novo"
	}
	for i, l := range e.Linhas {
		if i == MaxLinhas {
			break
		}
		linhas = append(linhas, fmt.Sprintf("⏰ %s (%d)", l.Nome, l.N))
	}
	return topo, linhas
}

// ParaAvisar escolhe os avisos que viram notificação: os de seq maior que
// ultimo. Na primeira pergunta depois de subir, só o das perdas (consulta
// vazia): os outros são de antes e já foram avisados.
func ParaAvisar(avisos []Aviso, ultimo int64, primeira bool) []Aviso {
	var out []Aviso
	for _, a := range avisos {
		if a.Seq > ultimo && (!primeira || a.Consulta == "") {
			out = append(out, a)
		}
	}
	return out
}
