package sessao

import "errors"

// Fase é o momento da sessão que a janelinha mostra.
type Fase string

const (
	Lendo    Fase = "lendo"    // lendo o modelo do pen drive
	Subindo  Fase = "subindo"  // esperando o motor responder
	Pronta   Fase = "pronta"   // conversa aberta
	Dormindo Fase = "dormindo" // encerrando o motor e o cd2-ctl
	Dormiu   Fase = "dormiu"   // tudo encerrado: pode tirar o pen drive
	Erro     Fase = "erro"
)

// Estado é o que a sessão anuncia a cada mudança. URL só vem em Pronta. Erro
// vem em Erro e, no Dormiu, só se algo não parou (TextoNaoDesligou). Pct
// vai de 0 a 100: a leitura do modelo ocupa de 0 a 90 e a espera pelo motor
// de 90 a 99.
type Estado struct {
	Fase  Fase
	Pct   int
	Aviso string
	Erro  string
	URL   string
}

// Textos fixos de erro e de aviso.
const (
	TextoSemModelo  = "O modelo de IA não está neste pen drive. Grave o pen drive de novo."
	TextoSemMotor   = "O motor da IA não está neste pen drive. Grave o pen drive de novo."
	TextoNaoAcordou = "A IA não conseguiu acordar."
	// TextoAindaFechando: a IA de uma abertura anterior foi encerrada, mas
	// o log dela não soltou a tempo.
	TextoAindaFechando = "A IA anterior ainda está fechando. Espere alguns segundos e abra de novo."
	TextoParou         = "A IA parou de responder."
	TextoSemBanco      = "O controle do banco não subiu. Seguindo sem banco de dados (Demonstração e Archeo continuam)."
	// TextoNaoDesligou vai no Erro do Dormiu quando algum processo não parou:
	// aí não é seguro tirar o pen drive.
	TextoNaoDesligou = "Não consegui desligar tudo. Reinicie o computador antes de tirar o pen drive."
	pctFimLeitura    = 90
	pctTetoEspera    = 99
	pctPronta        = 100
)

// ErrSemMotor é o que PrepararCache devolve quando o motor não está no pen drive.
var ErrSemMotor = errors.New(TextoSemMotor)

// ErrNaoPronta é o erro do AbrirConversa fora de Pronta.
var ErrNaoPronta = errors.New("a IA não está acordada")
