package main

import (
	"testing"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/origem"
	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/sessao"
)

// Sem a instalação no computador (Linux), o pen drive não convida a instalar:
// nem o item do ícone, nem a janelinha.
func TestSemInstalacaoNaoConvida(t *testing.T) {
	antes := instalavel
	t.Cleanup(func() { instalavel = antes })
	for _, pode := range []bool{true, false} {
		instalavel = pode
		a := &App{origem: origem.Pendrive}
		tem := false
		for _, it := range a.itensDoMenu(sessao.Estado{Fase: sessao.Pronta}) {
			tem = tem || it.Texto == itemInstalar
		}
		if tem != pode {
			t.Errorf("instalavel=%v: item de instalar = %v", pode, tem)
		}
	}
}
