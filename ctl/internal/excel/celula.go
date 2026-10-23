package excel

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// formato é como o estilo da célula mostra um número: como número mesmo, como data ou só como hora.
type formato int

const (
	formatoNumero formato = iota
	formatoData
	formatoHora
)

// leitorDeCelula converte as células de um arquivo aberto, guardando o formato de cada estilo já visto.
type leitorDeCelula struct {
	f        *excelize.File
	date1904 bool
	formatos map[int]formato
}

// grade monta a aba como o núcleo espera. Lê duas vezes: o texto formatado (o que o Excel mostra) e o
// valor cru (o número por trás da data ou do "R$ 1.500,00"). As duas matrizes podem ter comprimentos
// diferentes, porque a excelize corta a cauda vazia de cada uma; o que falta conta como vazio. Erro na
// leitura (folha de gráfico, por exemplo) devolve grade nula, que o núcleo trata como aba vazia.
func (l *leitorDeCelula) grade(aba string) [][]Celula {
	formatadas, err := l.f.GetRows(aba)
	if err != nil {
		return nil
	}
	cruas, err := l.f.GetRows(aba, excelize.Options{RawCellValue: true})
	if err != nil {
		return nil
	}
	grade := make([][]Celula, max(len(formatadas), len(cruas)))
	for i := range grade {
		lf, lc := linhaOuNada(formatadas, i), linhaOuNada(cruas, i)
		grade[i] = make([]Celula, max(len(lf), len(lc)))
		for j := range grade[i] {
			grade[i][j] = l.celula(aba, i, j, valorOuNada(lf, j), valorOuNada(lc, j))
		}
	}
	return grade
}

func (l *leitorDeCelula) celula(aba string, i, j int, formatado, cru string) Celula {
	if formatado == "" && cru == "" {
		return Celula{}
	}
	ref, err := excelize.CoordinatesToCellName(j+1, i+1)
	if err != nil {
		return Celula{Tipo: Texto, Texto: formatado}
	}
	tipo, _ := l.f.GetCellType(aba, ref)
	return converter(tipo, formatado, cru, l.date1904, func() formato { return l.formatoDaCelula(aba, ref) })
}

// converter decide o tipo da célula a partir do tipo gravado no arquivo, do texto formatado e do valor
// cru. formatoDoEstilo só é chamado para número, que é quando o estilo importa. Texto é o formatado,
// salvo em Data, que leva o ISO: o formatado da excelize sai no formato americano (14 vira mm-dd-yy) e
// 03/02 seria lido como 2 de março.
func converter(tipo excelize.CellType, formatado, cru string, date1904 bool, formatoDoEstilo func() formato) Celula {
	switch tipo {
	case excelize.CellTypeError:
		return Celula{Tipo: Erro, Texto: formatado}
	case excelize.CellTypeBool:
		c := Celula{Tipo: Bool, Texto: formatado}
		if cru == "1" || strings.EqualFold(cru, "TRUE") {
			c.Num = 1
		}
		return c
	}
	if cru == "" {
		return Celula{}
	}
	switch tipo {
	case excelize.CellTypeDate:
		// t="d": alguns geradores gravam a data como texto ISO em vez de número de série.
		for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
			if data, err := time.Parse(layout, cru); err == nil {
				return Celula{Tipo: Data, Texto: textoDaData(data), Data: data}
			}
		}
	case excelize.CellTypeNumber, excelize.CellTypeFormula, excelize.CellTypeUnset:
		num, err := strconv.ParseFloat(cru, 64)
		if err != nil || math.IsNaN(num) || math.IsInf(num, 0) {
			break
		}
		switch formatoDoEstilo() {
		case formatoData:
			if data, err := excelize.ExcelDateToTime(num, date1904); err == nil {
				return Celula{Tipo: Data, Texto: textoDaData(data), Data: data}
			}
		case formatoHora:
			// Hora ou duração ("36:00") não é data nem quantidade que se some como o Excel mostra.
			return Celula{Tipo: Texto, Texto: formatado}
		}
		return Celula{Tipo: Numero, Texto: formatado, Num: num}
	}
	return Celula{Tipo: Texto, Texto: formatado}
}

// textoDaData é o mesmo texto que aba.go grava em coluna DATE, para a data valer igual em coluna TEXT.
func textoDaData(d time.Time) string {
	if d.Hour() != 0 || d.Minute() != 0 || d.Second() != 0 {
		return d.Format("2006-01-02 15:04:05")
	}
	return d.Format("2006-01-02")
}

func (l *leitorDeCelula) formatoDaCelula(aba, ref string) formato {
	id, err := l.f.GetCellStyle(aba, ref)
	if err != nil {
		return formatoNumero
	}
	if fm, ok := l.formatos[id]; ok {
		return fm
	}
	fm := formatoNumero
	if estilo, err := l.f.GetStyle(id); err == nil && estilo != nil {
		custom := ""
		if estilo.CustomNumFmt != nil {
			custom = *estilo.CustomNumFmt
		}
		switch {
		case formatoDeData(estilo.NumFmt, custom):
			fm = formatoData
		case formatoDeHora(estilo.NumFmt, custom):
			fm = formatoHora
		}
	}
	l.formatos[id] = fm
	return fm
}

// formatoDeData diz se o formato mostra uma data (com ou sem hora). Embutidos de data do Excel: 14–17,
// 22, 27–36 e 50–58. No formato próprio, procuro y ou d no que sobra de limparFormato.
func formatoDeData(numFmt int, custom string) bool {
	switch {
	case numFmt >= 14 && numFmt <= 17, numFmt == 22, numFmt >= 27 && numFmt <= 36, numFmt >= 50 && numFmt <= 58:
		return true
	}
	return strings.ContainsAny(limparFormato(custom), "yd")
}

// formatoDeHora diz se o formato mostra só hora ou duração, sem data. Embutidos: 18–21 e 45–47. No
// formato próprio, h, m ou s sem y nem d ("m" sozinho é minuto aqui, porque mês sem dia nem ano é raro).
func formatoDeHora(numFmt int, custom string) bool {
	switch {
	case numFmt >= 18 && numFmt <= 21, numFmt >= 45 && numFmt <= 47:
		return true
	}
	limpo := limparFormato(custom)
	return strings.ContainsAny(limpo, "hms") && !strings.ContainsAny(limpo, "yd")
}

// limparFormato tira do formato o que não é código de data e hora: texto entre aspas ("m" de metro),
// colchetes ([Red], [$-416], [h] de duração) e o caractere escapado com \. Devolve em minúsculas.
func limparFormato(custom string) string {
	var b strings.Builder
	aspas, colchete, escapa := false, false, false
	for _, r := range custom {
		switch {
		case escapa:
			escapa = false
		case aspas:
			aspas = r != '"'
		case colchete:
			colchete = r != ']'
		case r == '\\':
			escapa = true
		case r == '"':
			aspas = true
		case r == '[':
			colchete = true
		default:
			b.WriteRune(r)
		}
	}
	return strings.ToLower(b.String())
}

func linhaOuNada(m [][]string, i int) []string {
	if i < len(m) {
		return m[i]
	}
	return nil
}

func valorOuNada(l []string, j int) string {
	if j < len(l) {
		return l[j]
	}
	return ""
}
