// Programa de teste manual (não publicado): SÓ lista os discos. Mostra o que
// o sistema devolve, o que o filtro deixa passar e como a tela descreveria
// cada um. Não apaga nem ejeta nada.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/discos"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	brutos, err := discos.Padrao().Listar(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro ao listar:", err)
		os.Exit(1)
	}
	fmt.Printf("Brutos (%d):\n", len(brutos))
	for _, b := range brutos {
		fmt.Printf("  %+v\n", b)
	}

	visiveis := discos.Filtrar(brutos)
	fmt.Printf("Depois do filtro (%d):\n", len(visiveis))
	for _, b := range visiveis {
		d := discos.Descrever(b, discos.ContarArquivos)
		fmt.Printf("  %+v\n", d)
	}
}
