package excel

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	abertos := 0
	for _, nome := range nomes {
		lidas, ignoradas, abriu := lerArquivo(pasta, nome, usados)
		tabelas = append(tabelas, lidas...)
		rel.Ignoradas = append(rel.Ignoradas, ignoradas...)
		if abriu {
			abertos++
		}
	}
	if len(tabelas) == 0 {
		if abertos == 0 {
			return rel, fmt.Errorf("Não consegui abrir nenhuma planilha dessa pasta. Veja o motivo de cada arquivo em Configurações.")
		}
		return rel, fmt.Errorf("Nenhuma aba das planilhas dessa pasta tem um cabeçalho que eu reconheça.")
	}
	if err := copia.Gravar(destino, tabelas); err != nil {
		return rel, err
	}
	rel.Tabelas = len(tabelas)
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

// lerArquivo devolve uma tabela por aba visível que tenha cabeçalho, e se o arquivo abriu. O nome leva
// o da aba só quando o arquivo tem mais de uma aba visível com conteúdo: planilha de uma aba só vira a
// tabela com o nome do arquivo, e folha de gráfico ou aba em branco ao lado não mudam isso.
func lerArquivo(pasta, nome string, usados map[string]int) (tabelas []copia.Tabela, ignoradas []Ignorada, abriu bool) {
	caminho := filepath.Join(pasta, nome)
	f, err := excelize.OpenFile(caminho)
	if err != nil {
		motivo := MotivoNaoAbriu
		if errors.Is(err, excelize.ErrWorkbookPassword) || protegidoPorSenha(caminho) {
			motivo = MotivoSenha
		}
		return nil, []Ignorada{{Arquivo: nome, Motivo: motivo}}, false
	}
	defer f.Close()
	date1904 := false
	if props, err := f.GetWorkbookProps(); err == nil && props.Date1904 != nil {
		date1904 = *props.Date1904
	}

	leitor := &leitorDeCelula{f: f, date1904: date1904, formatos: map[int]formato{}}
	abas := f.GetSheetList()
	grades := make([][][]Celula, len(abas))
	visiveis := make([]bool, len(abas))
	comConteudo := 0
	for i, aba := range abas {
		vis, err := f.GetSheetVisible(aba)
		visiveis[i] = err != nil || vis
		if !visiveis[i] {
			continue
		}
		grades[i] = leitor.grade(aba)
		if len(grades[i]) > 0 {
			comConteudo++
		}
	}
	base := copia.Identificador(strings.TrimSuffix(nome, filepath.Ext(nome)))
	for i, aba := range abas {
		if !visiveis[i] {
			ignoradas = append(ignoradas, Ignorada{Arquivo: nome, Aba: aba, Motivo: MotivoOculta})
			continue
		}
		colunas, linhas, motivo := analisarAba(grades[i])
		if motivo != "" {
			ignoradas = append(ignoradas, Ignorada{Arquivo: nome, Aba: aba, Motivo: motivo})
			continue
		}
		tabela := base
		if comConteudo >= 2 {
			tabela += "__" + copia.Identificador(aba)
		}
		tabelas = append(tabelas, copia.Tabela{Nome: copia.Unico(tabela, usados), Colunas: colunas, Linhas: linhas})
	}
	return tabelas, ignoradas, true
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
