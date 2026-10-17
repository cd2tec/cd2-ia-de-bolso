package gravar

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"

	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/baixar"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/manifesto"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/pendrive"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/preparo"
)

// Frases mostradas na tela. O texto é fixo.
const (
	fraseConferindoArquivos = "Conferindo os arquivos"
	fraseApagando           = "Apagando o pen drive"
	fraseCopiando           = "Copiando para o pen drive"
	fraseConferindo         = "Conferindo o que foi gravado"
	fraseProntoOk           = "Pronto."
	fraseProntoSem          = "Pronto. Ejete o pen drive pelo sistema antes de tirar."

	fraseInternet      = "Não consegui baixar o que falta. Confira a internet e tente de novo. Nada foi apagado."
	fraseDefeito       = "Um arquivo chegou com defeito duas vezes. Tente de novo daqui a pouco. Nada foi apagado."
	fraseAntivirus     = "O antivírus removeu o motor da IA. Libere a pasta da IA de Bolso no antivírus e tente de novo. Nada foi apagado."
	fraseNaoCabe       = "Esse pen drive é pequeno demais para a IA de Bolso. Nada foi apagado."
	fraseProprio       = "O instalador está rodando deste pen drive. Copie o instalador para o computador e abra de lá."
	fraseDiscoMudou    = "O pen drive mudou desde que você escolheu. Escolha de novo. Nada foi apagado."
	fraseFormatar      = "Não consegui formatar o pen drive. Tire, plugue de novo e tente outra vez."
	fraseRemovido      = "O pen drive foi removido no meio da gravação. Plugue de novo e grave outra vez."
	fraseDiferente     = "A conferência encontrou diferença. Não use este pen drive. Grave de novo e, se repetir, troque de pen drive."
	fraseCancelouAntes = "Gravação cancelada. Nada foi apagado."
	fraseCancelouApos  = "Gravação cancelada. O pen drive ficou incompleto. Grave de novo antes de usar."
	fraseGenerica      = "Não deu certo. Tente de novo."
	fraseGenericaApos  = "Não deu certo. O pen drive ficou incompleto. Grave de novo antes de usar."
)

// nomeDoItem traduz o Item do preparo para o que a pessoa lê.
func nomeDoItem(item string) string {
	switch {
	case item == "zip":
		return "os arquivos da IA de Bolso"
	case item == "motor":
		return "o motor"
	case item == "modelo":
		return "o modelo"
	case strings.HasPrefix(item, "toolbox"):
		return "o toolbox"
	case strings.HasPrefix(item, "cd2-ctl"):
		return "o controle do banco"
	}
	return "os arquivos"
}

// gb escreve bytes em GB decimal, uma casa, com vírgula: "1,6".
func gb(b int64) string {
	return strings.Replace(fmt.Sprintf("%.1f", float64(b)/1e9), ".", ",", 1)
}

func fraseBaixando(item string, feitos, total int64) string {
	nome := nomeDoItem(item)
	if total <= 0 {
		return "Baixando " + nome
	}
	return fmt.Sprintf("Baixando %s (%s de %s GB)", nome, gb(feitos), gb(total))
}

// ehRede diz se o erro veio da rede: falha de conexão, corte no meio do
// corpo ou status HTTP inesperado.
func ehRede(err error) bool {
	var ue *url.Error
	var ne net.Error
	return errors.As(err, &ue) || errors.As(err, &ne) ||
		errors.Is(err, baixar.ErrHTTP) || errors.Is(err, io.ErrUnexpectedEOF)
}

// frasePreparo traduz um erro de Obter ou Montar (nada foi apagado ainda).
func frasePreparo(err error) string {
	switch {
	case errors.Is(err, baixar.ErrConferencia):
		return fraseDefeito
	case errors.Is(err, manifesto.ErrSemManifesto), errors.Is(err, preparo.ErrFaltaInternet), ehRede(err):
		return fraseInternet
	}
	return fraseGenerica
}

// fraseGravacao traduz um erro de Copiar ou Conferir (o disco já foi apagado).
func fraseGravacao(err error) string {
	switch {
	case errors.Is(err, pendrive.ErrRemovido):
		return fraseRemovido
	case errors.Is(err, pendrive.ErrDiferente):
		return fraseDiferente
	}
	return fraseGenericaApos
}
