package main

import (
	"reflect"
	"testing"
)

func TestArgsGdbusEscapa(t *testing.T) {
	got := argsGdbus("Título", `It's \ ok`)
	if got[11] != `'Título'` || got[12] != `'It\'s \\ ok'` {
		t.Errorf("args = %q", got)
	}
	if !reflect.DeepEqual(got[13:], []string{"@as []", "@a{sv} {}", "int32 -1"}) {
		t.Errorf("fim = %q", got[13:])
	}
}

func TestAvisoSemIconeNaoSai(t *testing.T) {
	antes := enviarNotificacao
	t.Cleanup(func() { enviarNotificacao = antes })
	chamou := false
	enviarNotificacao = func(string, string) error { chamou = true; return nil }
	if temBandeja() {
		t.Skip("esta sessão tem bandeja")
	}
	if avisarPrimeiroFechar() || chamou {
		t.Error("sem bandeja, o aviso não deveria sair")
	}
}
