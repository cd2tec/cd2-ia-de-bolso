// Package excel lê as abas de uma planilha .xlsx como tabelas. Este arquivo é só a lógica pura, sobre
// uma grade de células em memória: decide qual linha é o cabeçalho, quais são dados e o tipo de cada
// coluna. Fica separado da leitura do arquivo porque planilha real é bagunçada (título em cima, linha
// em branco, total no fim) e essas regras precisam de teste sem depender de um .xlsx.
package excel

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/copia"
)

// Tipo é o que a célula contém, já decidido por quem leu o arquivo.
type Tipo int

const (
	Vazia Tipo = iota
	Texto
	Numero
	Data
	Bool
	Erro
)

// Celula guarda o valor tipado e o Texto como o Excel mostra. Num vale em Numero e em Bool (1 ou 0);
// Data vale em Data.
type Celula struct {
	Tipo  Tipo
	Texto string
	Num   float64
	Data  time.Time
}

const (
	MotivoSemCabecalho = "sem cabeçalho que eu reconheça"
	MotivoVazia        = "aba vazia"
	MotivoSoCabecalho  = "aba só com cabeçalho, sem dados"
)

// linhasDeBusca é até onde procuro o cabeçalho: título e linhas em branco no topo são comuns, mas
// depois disso é mais provável que a aba não seja uma tabela do que o cabeçalho estar lá no fundo.
const linhasDeBusca = 30

// analisarAba devolve as colunas e linhas da aba; motivo vem preenchido quando ela não vira tabela.
func analisarAba(grade [][]Celula) (colunas []copia.Coluna, linhas [][]any, motivo string) {
	if !temConteudo(grade) {
		return nil, nil, MotivoVazia
	}
	cab := acharCabecalho(grade)
	if cab < 0 {
		if soCabecalho(grade) {
			return nil, nil, MotivoSoCabecalho
		}
		return nil, nil, MotivoSemCabecalho
	}
	ini, fim := extensao(grade[cab])

	usados := map[string]int{}
	nomes := make([]string, 0, fim-ini+1)
	for j := ini; j <= fim; j++ {
		nomes = append(nomes, copia.Unico(nomeDaColuna(celula(grade[cab], j), j), usados))
	}

	var dados [][]Celula
	for _, l := range grade[cab+1:] {
		recorte := make([]Celula, fim-ini+1)
		for j := ini; j <= fim; j++ {
			recorte[j-ini] = celula(l, j)
		}
		if linhaSemDado(recorte) || linhaDeTotal(recorte) {
			continue
		}
		dados = append(dados, recorte)
	}

	tipos := make([]string, len(nomes))
	for j := range nomes {
		tipos[j] = tipoDaColuna(dados, j)
		colunas = append(colunas, copia.Coluna{Nome: nomes[j], Tipo: tipos[j]})
	}
	for _, l := range dados {
		saida := make([]any, len(nomes))
		for j, c := range l {
			saida[j] = valor(c, tipos[j])
		}
		linhas = append(linhas, saida)
	}
	return colunas, linhas, ""
}

func temConteudo(grade [][]Celula) bool {
	for _, l := range grade {
		for _, c := range l {
			if c.Tipo != Vazia {
				return true
			}
		}
	}
	return false
}

// celula devolve a célula j da linha, ou vazia quando a linha é mais curta (o leitor corta a cauda).
func celula(l []Celula, j int) Celula {
	if j < 0 || j >= len(l) {
		return Celula{}
	}
	return l[j]
}

// extensao dá o índice da primeira e da última célula não vazia da linha.
func extensao(l []Celula) (ini, fim int) {
	ini, fim = -1, -1
	for j, c := range l {
		if c.Tipo != Vazia {
			if ini < 0 {
				ini = j
			}
			fim = j
		}
	}
	return ini, fim
}

// nomeDaColuna é o identificador do cabeçalho na coluna j, ou Coluna_<letra> quando a célula não dá
// nome que sirva: vazia, Erro ("#REF!" viraria "REF") ou só símbolos ("#" viraria "T_").
func nomeDaColuna(c Celula, j int) string {
	pelaLetra := "Coluna_" + letraDaColuna(j+1)
	nome := strings.TrimSpace(c.Texto)
	if c.Tipo == Vazia || c.Tipo == Erro || nome == "" {
		return pelaLetra
	}
	id := copia.Identificador(nome)
	if strings.TrimPrefix(id, "T_") == "" {
		return pelaLetra
	}
	return id
}

// acharCabecalho pega a primeira candidata e, enquanto a próxima linha preenchida (a até 2 linhas)
// também for candidata e mais larga, desce para ela: título de duas células ([Controle de Vendas,
// 2026]) passa por candidata, mas o cabeçalho de verdade vem logo abaixo e ocupa mais colunas.
func acharCabecalho(grade [][]Celula) int {
	for i := 0; i < len(grade) && i < linhasDeBusca; i++ {
		if !ehCandidata(grade, i) {
			continue
		}
		for {
			k := proximaPreenchida(grade, i)
			if k < 0 || k >= linhasDeBusca || !ehCandidata(grade, k) || largura(grade[k]) <= largura(grade[i]) {
				return i
			}
			i = k
		}
	}
	return -1
}

// proximaPreenchida é a primeira linha não vazia logo abaixo de i, a até 2 linhas; -1 se não há.
func proximaPreenchida(grade [][]Celula, i int) int {
	for k := i + 1; k <= i+2 && k < len(grade); k++ {
		if !linhaVazia(grade[k]) {
			return k
		}
	}
	return -1
}

// largura conta da primeira à última célula preenchida da linha.
func largura(l []Celula) int {
	ini, fim := extensao(l)
	if ini < 0 {
		return 0
	}
	return fim - ini + 1
}

// caraDeCabecalho: a linha tem pelo menos 2 células e 2 delas com cara de título (ver
// pareceParteDeTitulo).
func caraDeCabecalho(l []Celula) bool {
	naoVazias, textos := 0, 0
	for _, c := range l {
		if c.Tipo == Vazia {
			continue
		}
		naoVazias++
		if pareceParteDeTitulo(c) {
			textos++
		}
	}
	return naoVazias >= 2 && textos >= 2
}

// ehCandidata: a linha tem cara de cabeçalho e logo abaixo (até 2 linhas de distância, para tolerar
// uma linha em branco) vem uma linha com dado dentro da largura dela.
func ehCandidata(grade [][]Celula, i int) bool {
	if !caraDeCabecalho(grade[i]) {
		return false
	}
	k := proximaPreenchida(grade, i)
	if k < 0 {
		return false
	}
	// Só a primeira linha não vazia decide: se o dado dela está fora da largura, as de baixo não valem.
	ini, fim := extensao(grade[i])
	for j := ini; j <= fim; j++ {
		if celula(grade[k], j).Tipo != Vazia {
			return true
		}
	}
	return false
}

// soCabecalho: nenhuma linha virou candidata, mas a última linha preenchida da aba, dentro da busca,
// tem cara de cabeçalho. É a tabela recém-criada, que ainda não recebeu nenhum registro.
func soCabecalho(grade [][]Celula) bool {
	for i := len(grade) - 1; i >= 0; i-- {
		if linhaVazia(grade[i]) {
			continue
		}
		return i < linhasDeBusca && caraDeCabecalho(grade[i])
	}
	return false
}

// pareceParteDeTitulo diz se a célula pode ser nome de coluna na hora de candidatar a linha. Além de
// Texto, aceita Data e inteiro de 1900 a 2100: planilha de comparação por ano ([Produto, 2024, 2025]) e
// controle diário ([Vendedor, 01/09, 02/09]) têm só uma célula de texto no cabeçalho, e o Excel guarda
// ano como número e dia como data. Número qualquer ([Vendedor, 100, 200]) continua sendo dado.
func pareceParteDeTitulo(c Celula) bool {
	switch c.Tipo {
	case Texto, Data:
		return true
	case Numero:
		return len(c.Texto) == 4 && c.Num >= 1900 && c.Num <= 2100 && c.Num == math.Trunc(c.Num)
	}
	return false
}

func linhaVazia(l []Celula) bool {
	for _, c := range l {
		if c.Tipo != Vazia {
			return false
		}
	}
	return true
}

// linhaSemDado: só células vazias ou Erro. Fórmula arrastada até o fim da folha deixa linhas inteiras
// de #N/A, que não são registro.
func linhaSemDado(l []Celula) bool {
	for _, c := range l {
		if c.Tipo != Vazia && c.Tipo != Erro {
			return false
		}
	}
	return true
}

// linhaDeTotal: a primeira célula não vazia é texto que, sem acento e em minúsculas, começa com
// "total" ou "subtotal". Começar com (e não ser igual a) pega "Total geral" e "Subtotal Centro"; o
// preço é pegar também "Totalmente ok", que aceitei porque total deixado na tabela estraga somas.
func linhaDeTotal(l []Celula) bool {
	for _, c := range l {
		if c.Tipo == Vazia {
			continue
		}
		if c.Tipo != Texto {
			return false
		}
		t := strings.ToLower(copia.SemAcento(strings.TrimSpace(c.Texto)))
		return strings.HasPrefix(t, "total") || strings.HasPrefix(t, "subtotal")
	}
	return false
}

// tipoDaColuna olha as células não vazias da coluna (Erro conta como vazia); mistura vira TEXT.
func tipoDaColuna(dados [][]Celula, j int) string {
	var n, numeros, datas, bools int
	todasInteiras := true
	for _, l := range dados {
		c := l[j]
		switch c.Tipo {
		case Vazia, Erro:
			continue
		case Numero:
			numeros++
			if !inteira(c.Num) {
				todasInteiras = false
			}
		case Data:
			datas++
		case Bool:
			bools++
		}
		n++
	}
	switch {
	case n == 0:
		return "TEXT"
	case numeros == n && todasInteiras:
		return "INTEGER"
	case numeros == n:
		return "NUMERIC"
	case datas == n:
		return "DATE"
	case bools == n:
		return "INTEGER"
	}
	return "TEXT"
}

// inteira exige caber em int64: float64 acima disso não converte sem estourar.
func inteira(f float64) bool {
	return f == math.Trunc(f) && f >= -9.223372036854775808e18 && f < 9.223372036854775808e18
}

// valor converte a célula para o que o SQLite recebe na coluna de tipo dado. Erro grava nulo mesmo em
// coluna TEXT: "#DIV/0!" não é dado do usuário.
func valor(c Celula, tipo string) any {
	switch c.Tipo {
	case Vazia, Erro:
		return nil
	}
	if tipo == "TEXT" {
		return textoCanonico(c)
	}
	switch c.Tipo {
	case Numero:
		if tipo == "INTEGER" {
			return int64(c.Num)
		}
		return c.Num
	case Data:
		return textoDaData(c.Data)
	case Bool:
		if c.Num != 0 {
			return int64(1)
		}
		return int64(0)
	}
	return c.Texto
}

// textoCanonico é o que vai para coluna TEXT. Número não usa o texto formatado: a excelize formata no
// padrão americano ("R$ 1,500.50"), e CAST(Valor AS REAL) leria 0 e estragaria a soma. Vai a forma
// canônica (1500.5), a data em ISO e o booleano em "1"/"0"; só o texto mesmo fica como está.
func textoCanonico(c Celula) string {
	switch c.Tipo {
	case Numero:
		return strconv.FormatFloat(c.Num, 'f', -1, 64)
	case Data:
		return textoDaData(c.Data)
	case Bool:
		if c.Num != 0 {
			return "1"
		}
		return "0"
	}
	return c.Texto
}

// letraDaColuna converte o número 1-based em letras do Excel (1=A, 26=Z, 27=AA). Cópia local pequena
// para este arquivo não depender da excelize.
func letraDaColuna(n int) string {
	var b []byte
	for n > 0 {
		n--
		b = append([]byte{byte('A' + n%26)}, b...)
		n /= 26
	}
	return string(b)
}
