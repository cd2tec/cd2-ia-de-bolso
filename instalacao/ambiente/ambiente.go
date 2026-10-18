//go:build !sandbox

// Package ambiente diz para qual Worker a compra vai e se este é o build de
// teste. O build de teste é feito com -tags sandbox (ver ambiente_sandbox.go).
package ambiente

// Sandbox é verdadeiro só no build de teste.
const Sandbox = false

// Worker é o endereço que cria o pagamento e assina a chave.
const Worker = "https://instalariadebolso.cd2.com.br"
