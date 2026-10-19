// Package volta é o servidor local que recebe a chave quando a pessoa volta
// da página do pagamento: GET /?chave=..., só em 127.0.0.1, numa porta livre.
package volta

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/chave"
)

const (
	textoPronto  = "Pronto, a chave chegou. Pode fechar esta página e voltar para o instalador."
	textoRuim    = "Esta chave não está completa. Copie a chave inteira da página do pagamento e cole no instalador."
	textoHost    = "Endereço não permitido."
	textoNaoTem  = "Página não encontrada."
	tituloPagina = "CD2 IA de Bolso"
)

// Servidor escuta a volta do pagamento e entrega a chave uma vez.
type Servidor struct {
	srv    *http.Server
	porta  int
	hosts  map[string]bool
	chaves chan string
	uma    sync.Once
	fechar sync.Once
}

// Ouvir sobe o servidor em 127.0.0.1:0. Ele se fecha com Fechar ou quando ctx
// terminar.
func Ouvir(ctx context.Context) (*Servidor, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	porta := l.Addr().(*net.TCPAddr).Port
	p := strconv.Itoa(porta)
	s := &Servidor{
		porta:  porta,
		hosts:  map[string]bool{"127.0.0.1:" + p: true, "localhost:" + p: true},
		chaves: make(chan string, 1),
	}
	s.srv = &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second}
	go s.srv.Serve(l)
	go func() {
		<-ctx.Done()
		s.Fechar()
	}()
	return s, nil
}

// Porta é a porta em que o servidor escuta.
func (s *Servidor) Porta() int { return s.porta }

// Chaves entrega a chave já normalizada, uma vez.
func (s *Servidor) Chaves() <-chan string { return s.chaves }

// Fechar desliga o servidor. Pode ser chamado mais de uma vez.
func (s *Servidor) Fechar() {
	s.fechar.Do(func() { s.srv.Close() })
}

func (s *Servidor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	// Host: contra DNS rebinding
	if !s.hosts[r.Host] {
		pagina(w, http.StatusForbidden, textoHost)
		return
	}
	if r.URL.Path != "/" || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		pagina(w, http.StatusNotFound, textoNaoTem)
		return
	}
	c, ok := chave.Normalizar(r.URL.Query().Get("chave"))
	if !ok {
		pagina(w, http.StatusBadRequest, textoRuim)
		return
	}
	s.uma.Do(func() { s.chaves <- c })
	pagina(w, http.StatusOK, textoPronto)
}

// pagina responde com um HTML simples de uma frase.
func pagina(w http.ResponseWriter, cod int, texto string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(cod)
	fmt.Fprintf(w, `<!doctype html>
<html lang="pt-BR">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>%s</title></head>
<body style="font-family: system-ui, sans-serif; max-width: 32rem; margin: 4rem auto; padding: 0 1rem; line-height: 1.5">
<p>%s</p>
</body>
</html>
`, tituloPagina, texto)
}
