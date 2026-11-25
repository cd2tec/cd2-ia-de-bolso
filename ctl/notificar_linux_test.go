//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/agenda"
)

type avisosFalsos struct {
	mu      sync.Mutex
	avs     []agenda.Aviso
	comecou chan struct{} // fechado na primeira leitura (o ponto de partida do notificar)
	uma     sync.Once
}

func novosAvisos(avs ...agenda.Aviso) *avisosFalsos {
	return &avisosFalsos{avs: avs, comecou: make(chan struct{})}
}

func (f *avisosFalsos) Avisos(desde int64) ([]agenda.Aviso, int64) {
	f.uma.Do(func() { close(f.comecou) })
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []agenda.Aviso
	var ultimo int64
	for _, a := range f.avs {
		if a.Seq > desde {
			out = append(out, a)
		}
		ultimo = a.Seq
	}
	return out, ultimo
}

func (f *avisosFalsos) por(a agenda.Aviso) { f.mu.Lock(); f.avs = append(f.avs, a); f.mu.Unlock() }

func TestNotificarSoOsNovos(t *testing.T) {
	f := novosAvisos(agenda.Aviso{Seq: 1, Titulo: "antigo", Texto: "de antes"})
	var mu sync.Mutex
	var enviados []string
	antes := enviarNotificacao
	enviarNotificacao = func(titulo, texto string) error { mu.Lock(); enviados = append(enviados, titulo+"|"+texto); mu.Unlock(); return nil }
	defer func() { enviarNotificacao = antes }()

	ctx, cancel := context.WithCancel(context.Background())
	fim := make(chan struct{})
	go func() { notificar(ctx, f, 5*time.Millisecond, &bytes.Buffer{}); close(fim) }()
	<-f.comecou
	f.por(agenda.Aviso{Seq: 2, Titulo: "IA de Bolso · Faturamento", Texto: "Avisou: caiu 19%"})
	time.Sleep(60 * time.Millisecond)
	cancel()
	<-fim

	mu.Lock()
	defer mu.Unlock()
	if len(enviados) != 1 || enviados[0] != "IA de Bolso · Faturamento|Avisou: caiu 19%" {
		t.Fatalf("enviados = %q", enviados)
	}
}

func TestNotificarSemNotifySendAvisaUmaVez(t *testing.T) {
	f := novosAvisos()
	antes := enviarNotificacao
	enviarNotificacao = func(string, string) error { return errors.New("executable file not found") }
	defer func() { enviarNotificacao = antes }()

	var log bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	fim := make(chan struct{})
	go func() { notificar(ctx, f, 5*time.Millisecond, &log); close(fim) }()
	<-f.comecou
	f.por(agenda.Aviso{Seq: 1, Titulo: "a", Texto: "1"})
	time.Sleep(30 * time.Millisecond)
	f.por(agenda.Aviso{Seq: 2, Titulo: "b", Texto: "2"})
	time.Sleep(30 * time.Millisecond)
	cancel()
	<-fim
	if n := bytes.Count(log.Bytes(), []byte("notificacao indisponivel")); n != 1 {
		t.Fatalf("avisou %d vezes no log: %q", n, log.String())
	}
}

func TestArgsGdbusEscapaAspas(t *testing.T) {
	a := argsGdbus("IA de Bolso · Venda d'água", `caiu 19% \ ontem`)
	if a[11] != `'IA de Bolso · Venda d\'água'` || a[12] != `'caiu 19% \\ ontem'` {
		t.Fatalf("args = %q", a)
	}
}
