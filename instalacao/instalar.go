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
	"strings"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/copiar"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/guarda"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/sistema"
)

// ErrSemChave indica que a chave guardada não vale para este computador.
var ErrSemChave = errors.New("Falta a chave deste computador.")

// ErrAvisoAtalhos vem junto (errors.Join) quando a IA ficou instalada e
// registrada, mas algum atalho ou o início com o computador falhou. Quem
// chama trata como "instalada, com aviso", não como falha.
var ErrAvisoAtalhos = errors.New("A IA foi instalada, mas não consegui criar todos os atalhos.")

// Opcoes é o que a pessoa escolheu na tela de instalação.
type Opcoes struct {
	Raiz   string // raiz do pen drive
	Cache  string // pasta de cache, onde mora a chave
	Versao string
	// AreaDeTrabalho cria o atalho na área de trabalho; Inicio abre a IA de
	// Bolso junto com o computador.
	AreaDeTrabalho, Inicio bool
	// AoArquivo, se não for nil, recebe o caminho (relativo, com barras
	// normais) de cada arquivo logo antes de ele ser copiado.
	AoArquivo func(destino string)
}

// Em variáveis para os testes trocarem a máquina de verdade.
var (
	goos         = runtime.GOOS
	goarch       = runtime.GOARCH
	sistemaDo    = sistema.Do
	valida       = guarda.Valida
	planejar     = copiar.Planejar
	copiarPara   = copiar.CopiarAvisando
	pastaDestino = func() (string, error) {
		home, _ := os.UserHomeDir()
		return destinoDoSistema(goos, os.Getenv, home)
	}
)

// destinoDoSistema é o sistema.Destino com recusa quando não dá para saber
// a pasta do usuário (sem isso, o Mac cairia em /Applications).
func destinoDoSistema(goos string, env func(string) string, home string) (string, error) {
	if home == "" && (goos != "windows" || env == nil || env("LOCALAPPDATA") == "") {
		return "", sistema.ErrSemPastaPessoal
	}
	return sistema.Destino(goos, env, home), nil
}

// Instalar copia a IA de Bolso do pen drive para o computador e a deixa
// pronta para abrir. Sem chave válida, devolve ErrSemChave sem mexer em nada.
// Logo depois da cópia registra a instalação, para ela sempre poder ser
// desinstalada; falha em atalho ou no início vira aviso (ErrAvisoAtalhos).
func Instalar(ctx context.Context, o Opcoes, prog func(feitos, total int64)) error {
	if !valida(o.Cache) {
		return ErrSemChave
	}
	destino, err := pastaDestino()
	if err != nil {
		return err
	}
	itens, _, err := planejar(o.Raiz, goos, goarch)
	if err != nil {
		return err
	}
	if err := copiarPara(ctx, itens, destino, prog, o.AoArquivo); err != nil {
		return err
	}
	s := sistemaDo(goos)
	lancador := sistema.Lancador(destino, goos)
	if err := s.Registrar(lancador, o.Versao); err != nil {
		return err
	}
	var avisos []error
	if err := s.Atalhos(lancador, o.AreaDeTrabalho); err != nil {
		avisos = append(avisos, err)
	}
	if err := s.LigarInicio(lancador, o.Inicio); err != nil {
		avisos = append(avisos, err)
	}
	if len(avisos) > 0 {
		return errors.Join(append([]error{ErrAvisoAtalhos}, avisos...)...)
	}
	return nil
}

// Desinstalar desliga o início, tira atalhos e registro, apaga o db.env do
// cache, apaga as sobras de uma cópia interrompida (.novo, .velho*) e devolve o comando que apaga a pasta da instalação depois que o
// lançador (pidLancador) sair. A chave fica, para reinstalar sem comprar de
// novo. Pode ser chamada de novo depois de um erro: cada passo aceita o que
// já foi desfeito. Recusa, sem mexer em nada, uma pasta que não seja a da
// instalação.
func Desinstalar(cache string, pidLancador int) (sistema.Comando, error) {
	destino, err := pastaDestino()
	if err != nil {
		return sistema.Comando{}, err
	}
	if !filepath.IsAbs(destino) || filepath.Base(destino) != sistema.NomePasta(goos) {
		return sistema.Comando{}, fmt.Errorf("a pasta %q não parece a da instalação; não vou apagar", destino)
	}
	s := sistemaDo(goos)
	if err := s.LigarInicio(sistema.Lancador(destino, goos), false); err != nil {
		return sistema.Comando{}, err
	}
	if err := s.Remover(); err != nil {
		return sistema.Comando{}, err
	}
	if err := os.Remove(filepath.Join(cache, "db.env")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return sistema.Comando{}, fmt.Errorf("não consegui apagar a configuração do banco: %w", err)
	}
	apagarSobras(destino)
	c := s.PlanoRemocao(destino, pidLancador)
	if c.Exe == "" {
		return sistema.Comando{}, fmt.Errorf("não consegui montar a remoção da pasta %q", destino)
	}
	return c, nil
}

// apagarSobras tira as pastas que uma cópia interrompida deixa ao lado da
// instalação: destino.novo, destino.velho e destino.velho<números>. O
// lançador roda de destino, não delas, então dá para apagar já. É o melhor
// esforço: uma sobra presa (Windows) não impede a desinstalação.
func apagarSobras(destino string) {
	pai, base := filepath.Dir(destino), filepath.Base(destino)
	entradas, err := os.ReadDir(pai)
	if err != nil {
		return
	}
	for _, e := range entradas {
		n := e.Name()
		if !strings.HasPrefix(n, base+".") {
			continue
		}
		resto := n[len(base)+1:]
		ok := resto == "novo"
		if v, achou := strings.CutPrefix(resto, "velho"); achou {
			ok = strings.Trim(v, "0123456789") == ""
		}
		if ok {
			_ = os.RemoveAll(filepath.Join(pai, n))
		}
	}
}
