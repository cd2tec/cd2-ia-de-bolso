package preparo

// AjustarMinimosParaTeste troca os tamanhos mínimos do modelo e do toolbox e
// devolve a função que os restaura. Existe só para testes de outros pacotes,
// que não conseguem servir um modelo de 1 GB.
func AjustarMinimosParaTeste(modelo, toolbox int64) (restaurar func()) {
	antigo := minimos
	minimos.Modelo, minimos.Toolbox = modelo, toolbox
	return func() { minimos = antigo }
}
