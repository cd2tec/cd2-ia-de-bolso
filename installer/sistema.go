package main

import (
	"os"
	goruntime "runtime"

	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/ajudante"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/discos"
	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/elevacao"
)

// sistemaPadrao devolve quem lista, apaga e ejeta o pen drive. No Windows sem
// administrador (o normal: a janela abre como o usuário logado), é o
// ajudante elevado, que o Windows só pede na primeira vez que a tela procura
// os discos. Aberto como administrador, ou fora do Windows, é direto.
func sistemaPadrao() discos.Sistema {
	if goruntime.GOOS != "windows" || elevacao.Elevado() {
		return discos.Padrao()
	}
	exe, err := os.Executable()
	if err != nil {
		return discos.Padrao()
	}
	return ajudante.NovoCliente(func(args []string) error { return elevacao.AbrirComoAdmin(exe, args) })
}

// servirAjudante roda o instalador no papel de ajudante, sem janela, se ele
// foi aberto com os argumentos do ajudante. Diz se foi.
func servirAjudante(args []string) bool {
	endereco, codigo, ok := ajudante.LerArgs(args)
	if !ok {
		return false
	}
	_ = ajudante.Servir(endereco, codigo, discos.Padrao())
	return true
}
