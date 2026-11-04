package main

import (
	"bytes"
	"errors"
	"log"
	"testing"
)

// stderrSemConsole imita o stderr de um exe de janela no Windows: toda
// escrita falha (handle inválido).
type stderrSemConsole struct{}

func (stderrSemConsole) Write([]byte) (int, error) { return 0, errors.New("The handle is invalid.") }

func TestRegistroGravaNoArquivoMesmoSemConsole(t *testing.T) {
	var arquivo bytes.Buffer
	l := log.New(saidaRegistro(&arquivo, stderrSemConsole{}), "", 0)
	l.Printf("falhou na etapa %q", "baixando")
	if got := arquivo.String(); got != "falhou na etapa \"baixando\"\n" {
		t.Fatalf("arquivo = %q; o stderr quebrado não pode impedir a gravação", got)
	}
}
