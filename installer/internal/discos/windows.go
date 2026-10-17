package discos

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
)

var (
	reRotulo  = regexp.MustCompile(`^[A-Z0-9 _-]{1,11}$`)
	reLetra   = regexp.MustCompile(`^[A-Za-z]$`)
	reMontagW = regexp.MustCompile(`^[A-Za-z]:\\$`)
)

//go:embed listar.ps1
var listarPS1 string

//go:embed apagar.ps1
var apagarPS1 string

//go:embed ejetar.ps1
var ejetarPS1 string

type windowsSistema struct{ x Executor }

// NovoWindows devolve o Sistema que fala com o PowerShell através de x.
func NovoWindows(x Executor) Sistema { return windowsSistema{x: x} }

// powershell monta a chamada. O script vai dentro de & { ... }, com os
// parâmetros depois da chave, e segue por -EncodedCommand (base64 de
// UTF-16LE): assim aspas e quebras de linha nunca passam pelo parser de
// linha de comando do powershell.exe 5.1.
func (w windowsSistema) powershell(ctx context.Context, script string, params ...string) ([]byte, error) {
	cmd := "& { " + strings.TrimSpace(script) + " }"
	if len(params) > 0 {
		cmd += " " + strings.Join(params, " ")
	}
	return w.x(ctx, "powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-EncodedCommand", codificar(cmd))
}

func codificar(cmd string) string {
	u := utf16.Encode([]rune(cmd))
	b := make([]byte, 0, len(u)*2)
	for _, c := range u {
		b = append(b, byte(c), byte(c>>8))
	}
	return base64.StdEncoding.EncodeToString(b)
}

// aspas protege um texto para dentro de aspas simples do PowerShell.
func aspas(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

type windowsDisco struct {
	Number       *int64          `json:"Number"`
	FriendlyName string          `json:"FriendlyName"`
	Size         *int64          `json:"Size"`
	IsBoot       bool            `json:"IsBoot"`
	IsSystem     bool            `json:"IsSystem"`
	DriveLetters json.RawMessage `json:"DriveLetters"`
}

// lerDiscosWindows lê a saída do ConvertTo-Json do PowerShell 5.1. Com um
// disco só ele devolve um objeto, não uma lista; aceita as duas formas.
func lerDiscosWindows(js []byte) ([]Bruto, error) {
	t := strings.TrimSpace(strings.TrimPrefix(string(js), "\xef\xbb\xbf"))
	if t == "" || t == "null" {
		return nil, nil
	}
	var lista []windowsDisco
	if strings.HasPrefix(t, "{") {
		var um windowsDisco
		if err := json.Unmarshal([]byte(t), &um); err != nil {
			return nil, fmt.Errorf("lista de discos ilegível: %w", err)
		}
		lista = []windowsDisco{um}
	} else if err := json.Unmarshal([]byte(t), &lista); err != nil {
		return nil, fmt.Errorf("lista de discos ilegível: %w", err)
	}
	var saida []Bruto
	for _, d := range lista {
		// Sem número ou tamanho (leitor de cartão vazio, por exemplo) o disco
		// é pulado: não aparece e não derruba a lista dos outros.
		if d.Number == nil || d.Size == nil {
			log.Printf("discos: pulei %q, veio sem Number ou Size", d.FriendlyName)
			continue
		}
		b := Bruto{
			ID:      strconv.FormatInt(*d.Number, 10),
			Nome:    strings.TrimSpace(d.FriendlyName),
			Bytes:   *d.Size,
			Boot:    d.IsBoot,
			Sistema: d.IsSystem,
		}
		for _, l := range letras(d.DriveLetters) {
			b.Montagens = append(b.Montagens, l+`:\`)
		}
		saida = append(saida, b)
	}
	return saida, nil
}

// letras aceita DriveLetters como lista ou como texto solto.
func letras(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var lista []string
	if json.Unmarshal(raw, &lista) == nil {
		return limpaLetras(lista)
	}
	var uma string
	if json.Unmarshal(raw, &uma) == nil {
		return limpaLetras([]string{uma})
	}
	return nil
}

func limpaLetras(l []string) []string {
	var s []string
	for _, x := range l {
		if x = strings.TrimSpace(x); x != "" {
			s = append(s, strings.ToUpper(x[:1]))
		}
	}
	return s
}

func (w windowsSistema) Listar(ctx context.Context) ([]Bruto, error) {
	out, err := w.powershell(ctx, listarPS1)
	if err != nil {
		return nil, fmt.Errorf("não consegui listar os discos: %w", err)
	}
	return lerDiscosWindows(out)
}

func (w windowsSistema) Apagar(ctx context.Context, id, rotulo string) (string, error) {
	// O ID vira argumento de PowerShell: só número, nada mais.
	n, err := strconv.Atoi(id)
	if err != nil || n < 0 {
		return "", fmt.Errorf("número de disco inválido: %q", id)
	}
	if !reRotulo.MatchString(rotulo) {
		return "", fmt.Errorf("rótulo inválido: %q", rotulo)
	}
	out, err := w.powershell(ctx, apagarPS1, "-DiskNumber", strconv.Itoa(n), "-Label", aspas(rotulo))
	if err != nil {
		return "", fmt.Errorf("a formatação falhou: %w", err)
	}
	letra := ultimaLinha(string(out))
	if !reLetra.MatchString(letra) {
		return "", fmt.Errorf("formatei, mas a saída não trouxe uma letra de unidade: %q", letra)
	}
	return strings.ToUpper(letra) + `:\`, nil
}

func (w windowsSistema) Ejetar(ctx context.Context, _, montagem string) error {
	if !reMontagW.MatchString(montagem) {
		return fmt.Errorf("letra de unidade inválida para ejetar: %q", montagem)
	}
	letra := strings.ToUpper(montagem[:1])
	if _, err := w.powershell(ctx, ejetarPS1, "-Letter", aspas(letra)); err != nil {
		return fmt.Errorf("não consegui ejetar %s: %w", letra, err)
	}
	return nil
}

// ultimaLinha devolve a última linha não vazia: o script imprime só a
// letra, mas qualquer aviso solto antes dela não pode atrapalhar.
func ultimaLinha(s string) string {
	linhas := strings.Split(strings.ReplaceAll(s, "\r", ""), "\n")
	for i := len(linhas) - 1; i >= 0; i-- {
		if t := strings.TrimSpace(linhas[i]); t != "" {
			return t
		}
	}
	return ""
}
