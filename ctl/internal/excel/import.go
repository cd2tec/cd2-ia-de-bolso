package excel

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/richardlehane/mscfb"
	"github.com/xuri/excelize/v2"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/copia"
)

const (
	MotivoOculta   = "aba oculta"
	MotivoNaoAbriu = "o arquivo não abriu"
	MotivoSenha    = "arquivo protegido por senha"
)

// Ignorada é uma aba (ou um arquivo inteiro, com Aba vazia) que não virou tabela, e o porquê.
type Ignorada struct {
	Arquivo string `json:"arquivo"`
	Aba     string `json:"aba"`
	Motivo  string `json:"motivo"`
}

// Relatorio conta o que a leitura fez, para mostrar ao usuário o que ficou de fora.
type Relatorio struct {
	Arquivos  int        `json:"arquivos"`
	Tabelas   int        `json:"tabelas"`
	Ignoradas []Ignorada `json:"ignoradas"`
	LidoEm    time.Time  `json:"lido_em"`
}

// Import lê os .xlsx e .xlsm de pasta, uma tabela por aba, e regrava o SQLite em destino. O relatório
// vem também com erro, para dizer o que foi ignorado; só é nulo quando a pasta não existe ou não é pasta.
func Import(pasta, destino string) (*Relatorio, error) {
	info, err := os.Stat(pasta)
	if err != nil {
		return nil, fmt.Errorf("Não achei a pasta das planilhas.")
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("A base das planilhas é uma pasta com os arquivos .xlsx, não um arquivo.")
	}
	rel := &Relatorio{Ignoradas: []Ignorada{}, LidoEm: time.Now()}
	nomes, antigos, err := arquivos(pasta)
	if err != nil {
		return rel, err
	}
	rel.Arquivos = len(nomes)
	if len(nomes) == 0 {
		msg := "Não achei planilha .xlsx ou .xlsm nessa pasta."
		if antigos {
			msg += " Encontrei arquivos .xls/.csv, que ainda não leio. Abra cada um no Excel e salve como .xlsx."
		}
		return rel, errors.New(msg)
	}
	usados := map[string]int{}
	var tabelas []copia.Tabela
	for _, nome := range nomes {
		lidas, ignoradas := lerArquivo(pasta, nome, usados)
		tabelas = append(tabelas, lidas...)
		rel.Ignoradas = append(rel.Ignoradas, ignoradas...)
	}
	rel.Tabelas = len(tabelas)
	if len(tabelas) == 0 {
		return rel, fmt.Errorf("Nenhuma aba das planilhas dessa pasta tem um cabeçalho que eu reconheça.")
	}
	if err := copia.Gravar(destino, tabelas); err != nil {
		return rel, err
	}
	return rel, nil
}

// arquivos lista os .xlsx/.xlsm em ordem alfabética, sem a trava "~$" que o Excel deixa com o arquivo
// aberto nem os ocultos; antigos diz se há .xls ou .csv, que não leio, para a mensagem dar a dica.
func arquivos(pasta string) (nomes []string, antigos bool, err error) {
	entradas, err := os.ReadDir(pasta)
	if err != nil {
		return nil, false, fmt.Errorf("Não consegui ler a pasta das planilhas.")
	}
	for _, e := range entradas {
		nome := e.Name()
		if e.IsDir() || strings.HasPrefix(nome, "~$") || strings.HasPrefix(nome, ".") {
			continue
		}
		switch strings.ToLower(filepath.Ext(nome)) {
		case ".xlsx", ".xlsm":
			nomes = append(nomes, nome)
		case ".xls", ".csv":
			antigos = true
		}
	}
	sort.Strings(nomes)
	return nomes, antigos, nil
}

// lerArquivo devolve uma tabela por aba visível que tenha cabeçalho. O nome leva o da aba só quando o
// arquivo tem mais de uma aba visível: planilha de uma aba só vira a tabela com o nome do arquivo.
func lerArquivo(pasta, nome string, usados map[string]int) (tabelas []copia.Tabela, ignoradas []Ignorada) {
	caminho := filepath.Join(pasta, nome)
	f, err := excelize.OpenFile(caminho)
	if err != nil {
		motivo := MotivoNaoAbriu
		if errors.Is(err, excelize.ErrWorkbookPassword) || protegidoPorSenha(caminho) {
			motivo = MotivoSenha
		}
		return nil, []Ignorada{{Arquivo: nome, Motivo: motivo}}
	}
	defer f.Close()
	date1904 := false
	if props, err := f.GetWorkbookProps(); err == nil && props.Date1904 != nil {
		date1904 = *props.Date1904
	}

	abas := f.GetSheetList()
	visiveis := make([]bool, len(abas))
	nVisiveis := 0
	for i, aba := range abas {
		vis, err := f.GetSheetVisible(aba)
		visiveis[i] = err != nil || vis
		if visiveis[i] {
			nVisiveis++
		}
	}
	base := copia.Identificador(strings.TrimSuffix(nome, filepath.Ext(nome)))
	leitor := &leitorDeCelula{f: f, date1904: date1904, ehData: map[int]bool{}}
	for i, aba := range abas {
		if !visiveis[i] {
			ignoradas = append(ignoradas, Ignorada{Arquivo: nome, Aba: aba, Motivo: MotivoOculta})
			continue
		}
		colunas, linhas, motivo := analisarAba(leitor.grade(aba))
		if motivo != "" {
			ignoradas = append(ignoradas, Ignorada{Arquivo: nome, Aba: aba, Motivo: motivo})
			continue
		}
		tabela := base
		if nVisiveis >= 2 {
			tabela += "__" + copia.Identificador(aba)
		}
		tabelas = append(tabelas, copia.Tabela{Nome: copia.Unico(tabela, usados), Colunas: colunas, Linhas: linhas})
	}
	return tabelas, ignoradas
}

// protegidoPorSenha: o .xlsx com senha não é um zip, é um contêiner OLE com os fluxos EncryptionInfo e
// EncryptedPackage. A excelize, aberta sem senha, devolve só o erro do zip, igual ao de arquivo
// corrompido; por isso olho o fluxo. Um .xls renomeado também é OLE, mas não tem EncryptionInfo.
func protegidoPorSenha(caminho string) bool {
	arq, err := os.Open(caminho)
	if err != nil {
		return false
	}
	defer arq.Close()
	doc, err := mscfb.New(arq)
	if err != nil {
		return false
	}
	for entrada, err := doc.Next(); err == nil; entrada, err = doc.Next() {
		if entrada.Name == "EncryptionInfo" {
			return true
		}
	}
	return false
}

// leitorDeCelula converte as células de um arquivo aberto, guardando por estilo se o formato é de data.
type leitorDeCelula struct {
	f        *excelize.File
	date1904 bool
	ehData   map[int]bool
}

// grade monta a aba como o núcleo espera. Lê duas vezes: o texto formatado (o que o Excel mostra) e o
// valor cru (o número por trás da data ou do "R$ 1.500,00"). As duas matrizes podem ter comprimentos
// diferentes, porque a excelize corta a cauda vazia de cada uma; o que falta conta como vazio.
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
	case excelize.CellTypeNumber, excelize.CellTypeFormula, excelize.CellTypeUnset:
		num, err := strconv.ParseFloat(cru, 64)
		if err != nil || math.IsNaN(num) || math.IsInf(num, 0) {
			break
		}
		if l.formatoDaCelulaEhData(aba, ref) {
			if data, err := excelize.ExcelDateToTime(num, l.date1904); err == nil {
				return Celula{Tipo: Data, Texto: formatado, Data: data}
			}
		}
		return Celula{Tipo: Numero, Texto: formatado, Num: num}
	}
	return Celula{Tipo: Texto, Texto: formatado}
}

func (l *leitorDeCelula) formatoDaCelulaEhData(aba, ref string) bool {
	id, err := l.f.GetCellStyle(aba, ref)
	if err != nil {
		return false
	}
	if eh, ok := l.ehData[id]; ok {
		return eh
	}
	eh := false
	if estilo, err := l.f.GetStyle(id); err == nil && estilo != nil {
		custom := ""
		if estilo.CustomNumFmt != nil {
			custom = *estilo.CustomNumFmt
		}
		eh = formatoDeData(estilo.NumFmt, custom)
	}
	l.ehData[id] = eh
	return eh
}

// formatoDeData diz se o formato numérico mostra o número como data ou hora. Os embutidos de data do
// Excel são 14–22, 27–36, 45–47 e 50–58; no formato próprio, procuro y, m, d, h ou s fora de texto
// literal ("m" entre aspas é metro), de colchete ([Red], [$-416]) e de caractere escapado com \.
func formatoDeData(numFmt int, custom string) bool {
	switch {
	case numFmt >= 14 && numFmt <= 22, numFmt >= 27 && numFmt <= 36,
		numFmt >= 45 && numFmt <= 47, numFmt >= 50 && numFmt <= 58:
		return true
	}
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
	return strings.ContainsAny(strings.ToLower(b.String()), "ymdhs")
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
