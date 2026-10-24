package main

import "testing"

// Na instalada, dormir e fechar param os agendamentos e o menu avisa; no pen
// drive não há agendamento rodando.
func TestRotuloDormirFechar(t *testing.T) {
	casos := []struct {
		texto     string
		instalada bool
		quer      string
	}{
		{itemDormir, true, "Colocar a IA para dormir (os agendamentos param)"},
		{itemFechar, true, "Fechar (os agendamentos param)"},
		{itemDormir, false, "Colocar a IA para dormir"},
		{itemFechar, false, "Fechar"},
		{itemMostrar, true, itemMostrar},
	}
	for _, c := range casos {
		if got := rotuloItem(c.texto, c.instalada); got != c.quer {
			t.Errorf("rotuloItem(%q, %v) = %q, quer %q", c.texto, c.instalada, got, c.quer)
		}
	}
}

func TestTituloMarca(t *testing.T) {
	for total, quer := range map[int]string{0: "", 1: " 1", 3: " 3"} {
		if got := tituloMarca(total); got != quer {
			t.Errorf("tituloMarca(%d) = %q, quer %q", total, got, quer)
		}
	}
}
