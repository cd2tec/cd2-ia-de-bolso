package elevacao

import "testing"

func TestLerArgs(t *testing.T) {
	casos := []struct {
		args []string
		quer bool
	}{
		{nil, false},
		{[]string{}, false},
		{[]string{"--instalar"}, true},
		{[]string{"-psn_0_12345", "--instalar"}, true},
		{[]string{"--instalar=1"}, false},
		{[]string{"instalar"}, false},
		{[]string{"--outra", "x"}, false},
	}
	for _, c := range casos {
		if got := LerArgs(c.args); got != c.quer {
			t.Errorf("LerArgs(%q) = %v, quer %v", c.args, got, c.quer)
		}
	}
}

func TestComandoSemAdmin(t *testing.T) {
	exe := `C:\Users\João Silva\Downloads\Instalador IA (1).exe`
	got, err := parametrosSemAdmin(exe, []string{"--instalar"})
	if err != nil {
		t.Fatal(err)
	}
	quer := `/d /c set __COMPAT_LAYER=RunAsInvoker&& start "" "C:\Users\João Silva\Downloads\Instalador IA (1).exe" "--instalar"`
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
