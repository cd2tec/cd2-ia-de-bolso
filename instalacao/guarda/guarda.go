// Package guarda cuida de onde a chave de ativação mora: um arquivo na pasta
// de cache do usuário, só leitura e escrita para ele.
package guarda

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/chave"
	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/codigo"
)

const nomeArquivo = "instalacao.chave"

// Atrás de variáveis para os testes não dependerem da máquina de verdade.
var (
	lerCodigo = codigo.Ler
	conferir  = chave.Conferir
)

// Ler devolve a chave normalizada guardada em cache, ou "" se não houver
// arquivo ou se o conteúdo não for uma chave.
func Ler(cache string) string {
	b, err := os.ReadFile(filepath.Join(cache, nomeArquivo))
	if err != nil {
		return ""
	}
	norm, ok := chave.Normalizar(string(b))
	if !ok {
		return ""
	}
	return norm
}

// Gravar normaliza a chave e a grava com permissão 0600, criando a pasta
// (0700) se for preciso. Uma chave que não normaliza não é gravada.
func Gravar(cache, k string) error {
	norm, ok := chave.Normalizar(k)
	if !ok {
		return errors.New("a chave não tem o formato certo")
	}
	if err := os.MkdirAll(cache, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cache, nomeArquivo), []byte(norm+"\n"), 0o600)
}

// Valida diz se a chave guardada vale para este computador.
func Valida(cache string) bool {
	k := Ler(cache)
	if k == "" {
		return false
	}
	cod, err := lerCodigo()
	if err != nil {
		return false
	}
	return conferir(k, cod)
}
