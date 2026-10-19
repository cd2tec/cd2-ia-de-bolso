package ajudante

import (
	"context"
	"errors"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cd2tec/cd2-ia-de-bolso/installer/internal/discos"
)

// falso é o Sistema do lado do ajudante: anota as chamadas e devolve o combinado.
type falso struct {
	mu       sync.Mutex
	chamadas []string
	brutos   []discos.Bruto
	erro     error
}

func (f *falso) anotar(s string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chamadas = append(f.chamadas, s)
}

func (f *falso) Listar(context.Context) ([]discos.Bruto, error) {
	f.anotar("listar")
	return f.brutos, f.erro
}

func (f *falso) Apagar(_ context.Context, id, rotulo string) (string, error) {
	f.anotar("apagar " + id + " " + rotulo)
	return `E:\`, f.erro
}

func (f *falso) Ejetar(_ context.Context, id, montagem string) error {
	f.anotar("ejetar " + id + " " + montagem)
	return f.erro
}

// abrirNaGoroutine faz o papel do UAC: em vez de um processo elevado, roda o
// ajudante numa goroutine com os mesmos argumentos.
func abrirNaGoroutine(t *testing.T, s discos.Sistema, aberturas *int) func([]string) error {
	return func(args []string) error {
		*aberturas++
		endereco, codigo, ok := LerArgs(args)
		if !ok {
			t.Fatalf("argumentos do ajudante errados: %q", args)
		}
		go func() { _ = Servir(endereco, codigo, s) }()
		return nil
	}
}

// Listar, Apagar e Ejetar chegam ao Sistema do ajudante com os mesmos dados,
// e o ajudante abre uma vez só.
func TestClienteRepassaAsTresOperacoes(t *testing.T) {
	f := &falso{brutos: []discos.Bruto{{ID: "2", Nome: "Pen", Bytes: 16e9, Montagens: []string{`E:\`}}}}
	var aberturas int
	c := NovoCliente(abrirNaGoroutine(t, f, &aberturas))
	defer c.Fechar()
	ctx := context.Background()

	got, err := c.Listar(ctx)
	if err != nil || !reflect.DeepEqual(got, f.brutos) {
		t.Fatalf("Listar = %v, %v", got, err)
	}
	m, err := c.Apagar(ctx, "2", "CD2-IA")
	if err != nil || m != `E:\` {
		t.Fatalf("Apagar = %q, %v", m, err)
	}
	if err := c.Ejetar(ctx, "2", `E:\`); err != nil {
		t.Fatalf("Ejetar: %v", err)
	}
	want := []string{"listar", "apagar 2 CD2-IA", `ejetar 2 E:\`}
	if !reflect.DeepEqual(f.chamadas, want) {
		t.Fatalf("chamadas = %q, quero %q", f.chamadas, want)
	}
	if aberturas != 1 {
		t.Fatalf("o ajudante abriu %d vezes, quero 1", aberturas)
	}
}

// O erro do ajudante volta com o texto (é o que explica o problema no log).
func TestClienteDevolveOErroDoAjudante(t *testing.T) {
	f := &falso{erro: errors.New("powershell: exit status 1: disco ocupado")}
	var aberturas int
	c := NovoCliente(abrirNaGoroutine(t, f, &aberturas))
	defer c.Fechar()
	_, err := c.Apagar(context.Background(), "2", "CD2-IA")
	if err == nil || !strings.Contains(err.Error(), "disco ocupado") {
		t.Fatalf("erro = %v", err)
	}
}

// Quem disse "Não" no UAC não vê a pergunta de novo a cada busca da tela
// (1,5 s): o cliente espera a pausa antes de abrir de novo.
func TestClienteRespeitaAPausaDepoisDaRecusa(t *testing.T) {
	var aberturas int
	agora := time.Unix(1000, 0)
	c := NovoCliente(func([]string) error { aberturas++; return ErrRecusou })
	c.agora = func() time.Time { return agora }
	defer c.Fechar()
	for i := 0; i < 3; i++ {
		if _, err := c.Listar(context.Background()); !errors.Is(err, ErrRecusou) {
			t.Fatalf("erro = %v, quero ErrRecusou", err)
		}
	}
	if aberturas != 1 {
		t.Fatalf("perguntou %d vezes dentro da pausa, quero 1", aberturas)
	}
	agora = agora.Add(PausaRecusa + time.Second)
	c.Listar(context.Background())
	if aberturas != 2 {
		t.Fatalf("depois da pausa perguntou %d vezes, quero 2", aberturas)
	}
}

// Se o ajudante morrer, a próxima chamada abre outro.
func TestClienteReabreQuandoOAjudanteCai(t *testing.T) {
	f := &falso{}
	var aberturas int
	c := NovoCliente(abrirNaGoroutine(t, f, &aberturas))
	defer c.Fechar()
	if _, err := c.Listar(context.Background()); err != nil {
		t.Fatal(err)
	}
	c.mu.Lock()
	c.conn.Close()
	c.mu.Unlock()
	// A chamada em que a conexão caiu falha; a seguinte tem de abrir outro.
	_, _ = c.Listar(context.Background())
	if _, err := c.Listar(context.Background()); err != nil {
		t.Fatalf("depois de cair: %v", err)
	}
	if aberturas != 2 {
		t.Fatalf("aberturas = %d, quero 2", aberturas)
	}
}

// Quem conecta sem o código certo não manda nada no ajudante: o cliente
// recusa a conexão e espera a de verdade.
func TestClienteRecusaQuemNaoTemOCodigo(t *testing.T) {
	f := &falso{}
	var aberturas int
	c := NovoCliente(func(args []string) error {
		aberturas++
		endereco, codigo, _ := LerArgs(args)
		intruso, err := net.Dial("tcp", endereco)
		if err != nil {
			t.Fatal(err)
		}
		intruso.Write([]byte(`{"codigo":"errado"}` + "\n"))
		go func() {
			time.Sleep(50 * time.Millisecond)
			_ = Servir(endereco, codigo, f)
		}()
		return nil
	})
	defer c.Fechar()
	if _, err := c.Listar(context.Background()); err != nil {
		t.Fatalf("Listar: %v", err)
	}
	if len(f.chamadas) != 1 {
		t.Fatalf("chamadas = %q", f.chamadas)
	}
}

// LerArgs só aceita os dois argumentos do ajudante, nessa ordem.
func TestLerArgs(t *testing.T) {
	a := Args("127.0.0.1:5000", "abc")
	if e, c, ok := LerArgs(a); !ok || e != "127.0.0.1:5000" || c != "abc" {
		t.Fatalf("LerArgs(%q) = %q %q %v", a, e, c, ok)
	}
	for _, ruim := range [][]string{nil, {"--instalar"}, {ArgAjudante + "x"}, {ArgAjudante + "1.2.3.4:5", ArgCodigo + "abc"}} {
		if _, _, ok := LerArgs(ruim); ok {
			t.Errorf("LerArgs(%q) aceitou", ruim)
		}
	}
}
