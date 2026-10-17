package motor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestMain faz o papel do llamafile: o próprio binário de teste é re-executado
// com MOTOR_FALSO (ok, gpu-falha ou morre) e responde antes de o flag do go
// test ver os argumentos do motor.
func TestMain(m *testing.M) {
	modo := os.Getenv("MOTOR_FALSO")
	if modo == "" {
		os.Exit(m.Run())
	}
	cpu := false
	for _, a := range os.Args {
		if a == "disable" {
			cpu = true
		}
	}
	switch {
	case modo == "morre", modo == "gpu-falha" && !cpu:
		os.Stdout.WriteString("falha-gpu\n")
		os.Stderr.WriteString("erro-gpu\n")
		os.Exit(1)
	default:
		os.Stdout.WriteString("no-ar cpu=" + map[bool]string{true: "sim", false: "nao"}[cpu] + "\n")
		time.Sleep(time.Minute)
	}
}

func rapido(t *testing.T) {
	t.Helper()
	p, i := Prazo, Intervalo
	Prazo, Intervalo = 5*time.Second, 10*time.Millisecond
	t.Cleanup(func() { Prazo, Intervalo = p, i })
}

func cfg(t *testing.T) Config {
	t.Helper()
	return Config{Exe: os.Args[0], Modelo: "m.gguf", Web: "web", LogDir: t.TempDir(), Porta: 8081, Ctx: "8192", GOOS: "darwin"}
}

func TestArgsCaminhoComEspacoERaizDeDrive(t *testing.T) {
	a := Args(Config{Modelo: `E:\models\m.gguf`, Web: `E:\web`, Porta: 8081, Ctx: "8192"}, true)
	esperado := []string{"-m", `E:\models\m.gguf`, "--server", "--host", "127.0.0.1", "--port", "8081",
		"--jinja", "-c", "8192", "--path", `E:\web`, "-ngl", "999"}
	if !reflect.DeepEqual(a, esperado) {
		t.Fatalf("args = %q", a)
	}
}

func TestArgsCPU(t *testing.T) {
	a := Args(Config{Modelo: "m", Web: "w", Porta: 1, Ctx: "2"}, false)
	if got := strings.Join(a[len(a)-4:], " "); got != "--gpu disable -ngl 0" {
		t.Fatalf("final = %q", got)
	}
}

// getLog devolve 200 quando o server.log mostra que o motor falso está no ar.
func getLog(dir string) func(string) (int, error) {
	return func(string) (int, error) {
		b, err := os.ReadFile(filepath.Join(dir, "server.log"))
		if err == nil && strings.Contains(string(b), "no-ar") {
			return 200, nil
		}
		return 503, nil
	}
}

func TestSubirCaiParaCPU(t *testing.T) {
	rapido(t)
	t.Setenv("MOTOR_FALSO", "gpu-falha")
	c := cfg(t)
	var avisos []string
	p, err := Subir(context.Background(), c, getLog(c.LogDir), func(s string) { avisos = append(avisos, s) })
	if err != nil {
		t.Fatal(err)
	}
	defer p.Parar()
	if len(avisos) != 1 || avisos[0] != "Aceleração por GPU indisponível, tentando só com CPU..." {
		t.Fatalf("avisos = %q", avisos)
	}
	gpu, err := os.ReadFile(filepath.Join(c.LogDir, "server-gpu.log"))
	if err != nil || !strings.Contains(string(gpu), "falha-gpu") {
		t.Fatalf("server-gpu.log = %q, %v", gpu, err)
	}
	cpu, _ := os.ReadFile(filepath.Join(c.LogDir, "server.log"))
	if !strings.Contains(string(cpu), "cpu=sim") {
		t.Fatalf("server.log = %q", cpu)
	}
}

func TestSubirComGPUNaoAvisa(t *testing.T) {
	rapido(t)
	t.Setenv("MOTOR_FALSO", "ok")
	c := cfg(t)
	n := 0
	p, err := Subir(context.Background(), c, getLog(c.LogDir), func(string) { n++ })
	if err != nil {
		t.Fatal(err)
	}
	defer p.Parar()
	if n != 0 {
		t.Fatalf("avisou %d vezes", n)
	}
	if _, err := os.Stat(filepath.Join(c.LogDir, "server-gpu.log")); err == nil {
		t.Fatal("server-gpu.log não devia existir")
	}
}

func TestSubirDesiste(t *testing.T) {
	rapido(t)
	t.Setenv("MOTOR_FALSO", "morre")
	c := cfg(t)
	p, err := Subir(context.Background(), c, func(string) (int, error) { return 0, errors.New("sem conexão") }, func(string) {})
	if !errors.Is(err, ErrNaoSubiu) || p != nil {
		t.Fatalf("p = %v, err = %v", p, err)
	}
}

func TestSubirPrazoEstouraParaOProcesso(t *testing.T) {
	rapido(t)
	Prazo = 150 * time.Millisecond
	t.Setenv("MOTOR_FALSO", "ok")
	c := cfg(t)
	var pids []int
	get := func(string) (int, error) { return 503, nil }
	_, err := Subir(context.Background(), c, get, func(string) { pids = append(pids, 0) })
	if !errors.Is(err, ErrNaoSubiu) || len(pids) != 1 {
		t.Fatalf("err = %v, avisos = %d", err, len(pids))
	}
}

func TestEsperarMorreuNaoEspera(t *testing.T) {
	rapido(t)
	Intervalo = time.Hour
	t.Setenv("MOTOR_FALSO", "morre")
	c := cfg(t)
	p, err := Iniciar(c, true)
	if err != nil {
		t.Fatal(err)
	}
	ini := time.Now()
	err = Esperar(context.Background(), "http://x", p, time.Minute, func(string) (int, error) { return 0, errors.New("x") })
	if !errors.Is(err, ErrMorreu) || time.Since(ini) > 5*time.Second {
		t.Fatalf("err = %v em %v", err, time.Since(ini))
	}
}

func TestEsperarPrazo(t *testing.T) {
	rapido(t)
	t.Setenv("MOTOR_FALSO", "ok")
	c := cfg(t)
	p, err := Iniciar(c, true)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Parar()
	err = Esperar(context.Background(), "http://x", p, 100*time.Millisecond, func(string) (int, error) { return 503, nil })
	if !errors.Is(err, ErrPrazo) {
		t.Fatalf("err = %v", err)
	}
}

func TestEsperarPedeHealthEFicaProntoCom200(t *testing.T) {
	rapido(t)
	t.Setenv("MOTOR_FALSO", "ok")
	c := cfg(t)
	p, err := Iniciar(c, true)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Parar()
	var n int32
	var url atomic.Value
	get := func(u string) (int, error) {
		url.Store(u)
		if atomic.AddInt32(&n, 1) < 3 {
			return 0, errors.New("recusou")
		}
		return 200, nil
	}
	if err := Esperar(context.Background(), "http://127.0.0.1:8081", p, 5*time.Second, get); err != nil {
		t.Fatal(err)
	}
	if url.Load() != "http://127.0.0.1:8081/health" {
		t.Fatalf("url = %v", url.Load())
	}
}

func TestEsperarCancelado(t *testing.T) {
	rapido(t)
	t.Setenv("MOTOR_FALSO", "ok")
	c := cfg(t)
	p, err := Iniciar(c, true)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Parar()
	ctx, cancela := context.WithCancel(context.Background())
	cancela()
	err = Esperar(ctx, "http://x", p, time.Minute, func(string) (int, error) { return 503, nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestPararIdempotente(t *testing.T) {
	rapido(t)
	t.Setenv("MOTOR_FALSO", "ok")
	p, err := Iniciar(cfg(t), true)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Parar(); err != nil {
		t.Fatal(err)
	}
	if err := p.Parar(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.Morreu():
	default:
		t.Fatal("Morreu() devia estar fechado")
	}
}

func TestLogsDoWindowsSeparados(t *testing.T) {
	rapido(t)
	t.Setenv("MOTOR_FALSO", "morre")
	c := cfg(t)
	c.GOOS = "windows"
	p, err := Iniciar(c, true)
	if err != nil {
		t.Fatal(err)
	}
	<-p.Morreu()
	s, _ := os.ReadFile(filepath.Join(c.LogDir, "server.log"))
	e, _ := os.ReadFile(filepath.Join(c.LogDir, "server.err.log"))
	if !strings.Contains(string(s), "falha-gpu") || !strings.Contains(string(e), "erro-gpu") || strings.Contains(string(s), "erro-gpu") {
		t.Fatalf("server.log = %q, server.err.log = %q", s, e)
	}
}
