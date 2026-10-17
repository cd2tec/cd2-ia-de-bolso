//go:build !darwin

package navegador

// AreaUtil só faz sentido no Mac; fora dele devolve vazio.
func AreaUtil() string { return "" }
