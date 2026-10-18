//go:build sandbox

package ambiente

// Sandbox é verdadeiro só no build de teste.
const Sandbox = true

// Worker é o Worker de teste: Stripe em sandbox e assinatura com a chave do
// vetor de teste.
const Worker = "https://instalariadebolso-teste.cd2.com.br"
