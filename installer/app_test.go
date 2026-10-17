package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/discos"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/gravar"
)

// emissor guarda os eventos emitidos para a tela.
type emissor struct {
	mu     sync.Mutex
	nomes  []string
	dados  []any
	fimDeu chan Fim
}

func novoEmissor() *emissor { return &emissor{fimDeu: make(chan Fim, 1)} }

func (e *emissor) emitir(_ context.Context, nome string, dados ...any) {
	e.mu.Lock()
	e.nomes = append(e.nomes, nome)
	e.dados = append(e.dados, dados[0])
	e.mu.Unlock()
	if nome == "fim" {
		e.fimDeu <- dados[0].(Fim)
	}
}

func (e *emissor) esperarFim(t *testing.T) Fim {
	t.Helper()
	select {
	case f := <-e.fimDeu:
		return f
	case <-time.After(3 * time.Second):
		t.Fatal("o evento fim não chegou")
		return Fim{}
	}
}

func appDeTeste(e *emissor, g func(context.Context, gravar.Opcoes, func(gravar.Evento)) error) *App {
	a := NovaApp()
	a.ctx = context.Background()
	a.emitir = e.emitir
	a.gravar = g
	a.pastaCache = func() (string, error) { return "/tmp/cache-de-teste", nil }
	return a
}

var disco = discos.Disco{ID: "disk4", Nome: "SanDisk", Bytes: 32e9, Situacao: discos.Ok, Rotulo: "SanDisk 32 GB"}

func TestGravarEmiteProgressoEFimComAFraseDoPronto(t *testing.T) {
	e := novoEmissor()
	var recebeu gravar.Opcoes
	a := appDeTeste(e, func(_ context.Context, o gravar.Opcoes, ev func(gravar.Evento)) error {
		recebeu = o
		ev(gravar.Evento{Etapa: "baixando", Frase: "Baixando", Fracao: 0.2, Restante: -1})
		ev(gravar.Evento{Etapa: "pronto", Frase: "Pronto. Ejete o pen drive pelo sistema antes de tirar.", Fracao: 1})
		return nil
	})
	if err := a.Gravar(disco); err != nil {
		t.Fatal(err)
	}
	f := e.esperarFim(t)
	if !f.Ok || f.Frase != "Pronto. Ejete o pen drive pelo sistema antes de tirar." {
		t.Fatalf("fim = %+v", f)
	}
	if recebeu.Disco != disco || recebeu.Cache != "/tmp/cache-de-teste" || recebeu.URLManifesto != gravar.URLManifesto {
		t.Fatalf("opções = %+v", recebeu)
	}
	if recebeu.HTTP == nil || recebeu.HTTP.Timeout != 0 {
		t.Fatalf("HTTP precisa existir e não ter timeout total: %+v", recebeu.HTTP)
	}
	if recebeu.Sistema == nil {
		t.Fatal("sem Sistema")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.nomes) != 3 || e.nomes[0] != "progresso" || e.nomes[1] != "progresso" || e.nomes[2] != "fim" {
		t.Fatalf("eventos = %v", e.nomes)
	}
	if ev, ok := e.dados[0].(gravar.Evento); !ok || ev.Frase != "Baixando" {
		t.Fatalf("progresso = %#v", e.dados[0])
	}
}

func TestGravarComErroDevolveAFraseDoErro(t *testing.T) {
	e := novoEmissor()
	a := appDeTeste(e, func(context.Context, gravar.Opcoes, func(gravar.Evento)) error {
		return &gravar.Erro{Frase: "Não consegui baixar o que falta. Confira a internet e tente de novo. Nada foi apagado.", Causa: errors.New("rede")}
	})
	if err := a.Gravar(disco); err != nil {
		t.Fatal(err)
	}
	f := e.esperarFim(t)
	if f.Ok || f.Frase != "Não consegui baixar o que falta. Confira a internet e tente de novo. Nada foi apagado." {
		t.Fatalf("fim = %+v", f)
	}
}

func TestGravarComErroQueNaoEhErroDoGravarUsaFraseGenerica(t *testing.T) {
	e := novoEmissor()
	a := appDeTeste(e, func(context.Context, gravar.Opcoes, func(gravar.Evento)) error {
		return errors.New("qualquer coisa")
	})
	_ = a.Gravar(disco)
	if f := e.esperarFim(t); f.Ok || f.Frase != fraseGenerica {
		t.Fatalf("fim = %+v", f)
	}
}

func TestSegundoGravarEnquantoUmRodaDevolveErroECancelarPara(t *testing.T) {
	e := novoEmissor()
	comecou := make(chan struct{})
	a := appDeTeste(e, func(ctx context.Context, _ gravar.Opcoes, _ func(gravar.Evento)) error {
		close(comecou)
		<-ctx.Done()
		return &gravar.Erro{Frase: "Gravação cancelada. Nada foi apagado.", Causa: ctx.Err()}
	})
	if err := a.Gravar(disco); err != nil {
		t.Fatal(err)
	}
	<-comecou
	if err := a.Gravar(disco); err == nil {
		t.Fatal("o segundo Gravar deveria devolver erro")
	}
	a.Cancelar()
	if f := e.esperarFim(t); f.Ok || f.Frase != "Gravação cancelada. Nada foi apagado." {
		t.Fatalf("fim = %+v", f)
	}
	// Depois do fim, dá para gravar de novo.
	b := make(chan struct{})
	a.gravar = func(context.Context, gravar.Opcoes, func(gravar.Evento)) error { close(b); return nil }
	if err := a.Gravar(disco); err != nil {
		t.Fatalf("depois do fim, Gravar deveria aceitar: %v", err)
	}
	<-b
	e.esperarFim(t)
}

func TestGravandoRefletePeloEstadoDaGravacao(t *testing.T) {
	e := novoEmissor()
	solta := make(chan struct{})
	a := appDeTeste(e, func(context.Context, gravar.Opcoes, func(gravar.Evento)) error {
		<-solta
		return nil
	})
	if a.Gravando() {
		t.Fatal("sem gravação, Gravando deveria ser false")
	}
	if err := a.Gravar(disco); err != nil {
		t.Fatal(err)
	}
	if !a.Gravando() {
		t.Fatal("com gravação rodando, Gravando deveria ser true")
	}
	close(solta)
	e.esperarFim(t)
	if a.Gravando() {
		t.Fatal("depois do fim, Gravando deveria voltar a false")
	}
}

func TestCancelarSemGravacaoNaoFazNada(t *testing.T) {
	a := appDeTeste(novoEmissor(), nil)
	a.Cancelar()
}

func TestGravarSemPastaDeCacheDevolveErro(t *testing.T) {
	e := novoEmissor()
	a := appDeTeste(e, func(context.Context, gravar.Opcoes, func(gravar.Evento)) error {
		t.Fatal("não deveria gravar")
		return nil
	})
	a.pastaCache = func() (string, error) { return "", errors.New("sem pasta") }
	if err := a.Gravar(disco); err == nil {
		t.Fatal("esperava erro")
	}
	// E não fica preso como se estivesse gravando.
	a.pastaCache = func() (string, error) { return "/tmp/x", nil }
	a.gravar = func(context.Context, gravar.Opcoes, func(gravar.Evento)) error { return nil }
	if err := a.Gravar(disco); err != nil {
		t.Fatal(err)
	}
	e.esperarFim(t)
}
