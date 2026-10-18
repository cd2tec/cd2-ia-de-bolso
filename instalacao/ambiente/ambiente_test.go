package ambiente

import (
	"regexp"
	"testing"
)

// O endereço precisa casar com a lista que o web aceita (instalar.js).
func TestWorkerNoDominioDaCD2(t *testing.T) {
	if !regexp.MustCompile(`^https://[a-z0-9-]+\.cd2\.com\.br$`).MatchString(Worker) {
		t.Fatalf("Worker fora do domínio da CD2: %q", Worker)
	}
	teste := regexp.MustCompile(`-teste\.`).MatchString(Worker)
	if teste != Sandbox {
		t.Fatalf("Sandbox=%v mas Worker=%q", Sandbox, Worker)
	}
}
