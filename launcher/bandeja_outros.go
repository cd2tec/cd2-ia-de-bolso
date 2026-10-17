//go:build !darwin && !windows

package main

// O lançador só existe para Windows e Mac; isto só deixa o pacote compilar
// (go vet, go test) em outros sistemas.

const escondePeloAntesDeFechar = false

func iniciarBandeja(*App)            {}
func ligarCliqueSimples(*App)        {}
func formatoIcone(png []byte) []byte { return png }
func trazerParaFrente()              {}
func avisarPrimeiroFechar() bool     { return false }
func fecharBandeja()                 {}
