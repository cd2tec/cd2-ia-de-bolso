package elevacao

import "testing"

func TestLerArgs(t *testing.T) {
	const n = "0123456789abcdef0123456789abcdef"
	casos := []struct {
		args  []string
		quer  bool
		sinal string
	}{
		{nil, false, ""},
		{[]string{}, false, ""},
		{[]string{"--instalar"}, true, ""},
		{[]string{"-psn_0_12345", "--instalar"}, true, ""},
		{[]string{"--instalar", "--sinal=" + n}, true, n},
		{[]string{"--sinal=" + n, "--instalar"}, true, n},
		{[]string{"--instalar", "--sinal=xyz"}, true, ""},
		{[]string{"--instalar", "--sinal=" + n + "0"}, true, ""},
		{[]string{"--instalar", "--sinal=0123456789ABCDEF0123456789ABCDEF"}, true, ""},
		{[]string{"--sinal=" + n}, false, n},
		{[]string{"--instalar=1"}, false, ""},
		{[]string{"instalar"}, false, ""},
		{[]string{"--outra", "x"}, false, ""},
	}
	for _, c := range casos {
		got, sinal := LerArgs(c.args)
		if got != c.quer || sinal != c.sinal {
			t.Errorf("LerArgs(%q) = %v, %q; quer %v, %q", c.args, got, sinal, c.quer, c.sinal)
		}
	}
}

func TestNovoSinal(t *testing.T) {
	a, err := NovoSinal()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NovoSinal()
	if a == b {
		t.Fatal("dois sinais iguais")
	}
	if _, s := LerArgs([]string{ArgSinal + a}); s != a {
		t.Fatalf("o sinal %q não volta pelo LerArgs", a)
	}
}

func TestComandoSemAdmin(t *testing.T) {
	exe := `C:\Users\João Silva\Downloads\Instalador IA (1).exe`
	got, err := parametrosSemAdmin(exe, []string{"--instalar"})
	if err != nil {
		t.Fatal(err)
	}
	quer := `/d /v:off /c set __COMPAT_LAYER=RunAsInvoker&& start "" "C:\Users\João Silva\Downloads\Instalador IA (1).exe" "--instalar"`
	if got != quer {
		t.Fatalf("parâmetros:\n got %s\nquer %s", got, quer)
	}
	for _, ruim := range []string{`C:\a"b.exe`, `C:\100%\x.exe`, "C:\\a\nb.exe", ""} {
		if _, err := parametrosSemAdmin(ruim, nil); err == nil {
			t.Errorf("aceitou %q", ruim)
		}
	}
	if _, err := parametrosSemAdmin(`C:\x.exe`, []string{`a"b`}); err == nil {
		t.Error("aceitou argumento com aspas")
	}
}

func TestNomeSinal(t *testing.T) {
	if got := nomeSinal("ab"); got != `Local\CD2IADeBolso-ab` {
		t.Fatalf("nome = %q", got)
	}
}
