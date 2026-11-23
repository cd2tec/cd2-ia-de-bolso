//go:build linux

package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/agenda"
)

// No Windows e no Mac quem mostra os avisos dos agendamentos é o lançador (ícone
// perto do relógio). No Linux não há lançador: o próprio cd2-ctl manda cada aviso
// novo para a área de notificações (notify-send ou, sem ele, gdbus).

// fonteAvisos é o que notificar usa do agendador (o teste troca).
type fonteAvisos interface {
	Avisos(desde int64) ([]agenda.Aviso, int64)
}

// enviarNotificacao usa o notify-send (pacote libnotify-bin) e, sem ele, o gdbus
// direto no serviço de notificações do desktop (GNOME, KDE, XFCE…). Em variável para o teste.
var enviarNotificacao = func(titulo, texto string) error {
	if caminho, err := exec.LookPath("notify-send"); err == nil {
		return exec.Command(caminho, "--app-name=IA de Bolso", "--icon=dialog-information", titulo, texto).Run()
	}
	return exec.Command("gdbus", argsGdbus(titulo, texto)...).Run()
}

// argsGdbus monta a chamada org.freedesktop.Notifications.Notify. O gdbus lê cada
// argumento como GVariant em texto: as strings vão entre aspas simples, escapadas.
func argsGdbus(titulo, texto string) []string {
	return []string{"call", "--session", "--dest", "org.freedesktop.Notifications",
		"--object-path", "/org/freedesktop/Notifications", "--method", "org.freedesktop.Notifications.Notify",
		gvTexto("IA de Bolso"), "uint32 0", gvTexto("dialog-information"), gvTexto(titulo), gvTexto(texto),
		"@as []", "@a{sv} {}", "int32 -1"}
}

func gvTexto(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

// notificar acompanha os avisos até ctx acabar. Os que já existiam ao começar não
// saem (são de antes desta abertura). Sem notify-send, avisa uma vez no log e segue.
func notificar(ctx context.Context, f fonteAvisos, intervalo time.Duration, log io.Writer) {
	_, desde := f.Avisos(0)
	avisouFalha := false
	t := time.NewTicker(intervalo)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		avs, ultimo := f.Avisos(desde)
		desde = ultimo
		for _, av := range avs {
			if err := enviarNotificacao(av.Titulo, av.Texto); err != nil && !avisouFalha {
				avisouFalha = true
				fmt.Fprintf(log, "cd2-ctl: notificacao indisponivel (instale o libnotify-bin): %v\n", err)
			}
		}
	}
}

// iniciarNotificacoes liga as notificações do Linux para o agendador.
func iniciarNotificacoes(ctx context.Context, ag *agenda.Agendador, log io.Writer) {
	go notificar(ctx, ag, 5*time.Second, log)
}
