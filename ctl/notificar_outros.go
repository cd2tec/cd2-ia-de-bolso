//go:build !linux

package main

import (
	"context"
	"io"

	"github.com/cd2tec/cd2-ia-de-bolso/ctl/internal/agenda"
)

// No Windows e no Mac quem notifica é o lançador: aqui não faz nada.
func iniciarNotificacoes(context.Context, *agenda.Agendador, io.Writer) {}
