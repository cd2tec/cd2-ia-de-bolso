// Package instalacao junta as peças da instalação paga: confere a chave,
// copia do pen drive para o computador, cria os atalhos, liga ou desliga o
// início com o computador e registra. Também desfaz tudo isso.
package instalacao

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/copiar"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/guarda"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/sistema"
)

// ErrSemChave indica que a chave guardada não vale para este computador.
var ErrSemChave = errors.New("Falta a chave deste computador.")

// Opcoes é o que a pessoa escolheu na tela de instalação.
type Opcoes struct {
	Raiz   string // raiz do pen drive
	Cache  string // pasta de cache, onde mora a chave
	Versao string
	// AreaDeTrabalho cria o atalho na área de trabalho; Inicio abre a IA de
	// Bolso junto com o computador.
	AreaDeTrabalho, Inicio bool
}

// Em variáveis para os testes trocarem a máquina de verdade.
var (
	goos         = runtime.GOOS
	goarch       = runtime.GOARCH
	sistemaDo    = sistema.Do
	valida       = guarda.Valida
	planejar     = copiar.Planejar
	copiarPara   = copiar.Copiar
	pastaDestino = func() string {
		home, _ := os.UserHomeDir()
		return sistema.Destino(goos, os.Getenv, home)
	}
)

// Instalar copia a IA de Bolso do pen drive para o computador e a deixa
// pronta para abrir. Sem chave válida, devolve ErrSemChave sem mexer em nada.
func Instalar(ctx context.Context, o Opcoes, prog func(feitos, total int64)) error {
	if !valida(o.Cache) {
		return ErrSemChave
	}
	itens, _, err := planejar(o.Raiz, goos, goarch)
	if err != nil {
		return err
	}
	destino := pastaDestino()
	if err := copiarPara(ctx, itens, destino, prog); err != nil {
		return err
	}
	s := sistemaDo(goos)
	lancador := sistema.Lancador(destino, goos)
	if err := s.Atalhos(lancador, o.AreaDeTrabalho); err != nil {
		return err
	}
	if err := s.LigarInicio(lancador, o.Inicio); err != nil {
		return err
	}
	return s.Registrar(lancador, o.Versao)
}

// Desinstalar desliga o início, tira atalhos e registro, apaga o db.env do
// cache e devolve o comando que apaga a pasta da instalação depois que o
// lançador (pidLancador) sair. A chave fica, para reinstalar sem comprar de
// novo. Pode ser chamada de novo depois de um erro: cada passo aceita o que
// já foi desfeito.
func Desinstalar(cache string, pidLancador int) (sistema.Comando, error) {
	s := sistemaDo(goos)
	destino := pastaDestino()
	if err := s.LigarInicio(sistema.Lancador(destino, goos), false); err != nil {
		return sistema.Comando{}, err
	}
	if err := s.Remover(); err != nil {
		return sistema.Comando{}, err
	}
	if err := os.Remove(filepath.Join(cache, "db.env")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return sistema.Comando{}, fmt.Errorf("não consegui apagar a configuração do banco: %w", err)
	}
	return s.PlanoRemocao(destino, pidLancador), nil
}
