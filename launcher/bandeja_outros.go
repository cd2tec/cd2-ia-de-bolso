//go:build !darwin && !windows && !linux

package main

// O lançador existe para Windows, Mac e Linux; isto só deixa o pacote compilar
// (go vet, go test) em outros sistemas.

const (
	escondePeloAntesDeFechar = false
	marcaNoIcone             = false
)

func iniciarBandeja(*App)            {}
func ligarCliqueSimples(*App)        {}
func formatoIcone(png []byte) []byte { return png }
func trazerParaFrente()              {}
func avisarPrimeiroFechar() bool     { return false }
func fecharBandeja()                 {}
func notificar(_, _, _ string)       {}
