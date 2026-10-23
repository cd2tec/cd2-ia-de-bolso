// Package faketoolbox imita o MCP Toolbox nos testes do cd2-ctl. O binário de teste vira o
// toolbox falso quando roda com CD2_FAKE_TOOLBOX=1 (ver o TestMain dos pacotes toolbox e control).
package faketoolbox

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// EnvVar liga o modo toolbox falso.
const EnvVar = "CD2_FAKE_TOOLBOX"

// ReportVar, se definida, é o arquivo onde o falso grava os argumentos e o ambiente que recebeu.
const ReportVar = "CD2_FAKE_TOOLBOX_REPORT"

// Report é o que o falso recebeu do supervisor.
type Report struct {
	Args     []string `json:"args"`
	Engine   string   `json:"engine"`
	Host     string   `json:"host"`
	Password string   `json:"password"`
	HasKey   bool     `json:"has_key"`
	Sqlite   string   `json:"sqlite"`
}

// unescape desfaz o escape YAML (\\ e \"), para o falso imprimir também a senha crua.
var unescape = strings.NewReplacer(`\\`, `\`, `\"`, `"`)

// Main roda o toolbox falso e não volta. O comportamento vem da senha (já escapada, como o
// toolbox recebe em CD2_DB_PASSWORD):
//
//	morre...   imprime a senha crua, a escapada e uma linha com "Password", e sai com 1
//	lenta...   nunca abre a porta
//	iserror... sobe, mas db_connection_status devolve isError
//	demora...  sobe, mas db_connection_status leva 3 s para responder (banco atrás de VPN)
//	outra      sobe e responde como o toolbox de verdade
func Main() {
	fs := flag.NewFlagSet("toolbox", flag.ExitOnError)
	port := fs.Int("port", 0, "")
	fs.String("config", "", "")
	fs.String("address", "", "")
	fs.String("allowed-hosts", "", "")
	fs.String("allowed-origins", "", "")
	fs.Bool("disable-reload", false, "")
	_ = fs.Parse(os.Args[1:])
	pw := os.Getenv("CD2_DB_PASSWORD")
	if f := os.Getenv(ReportVar); f != "" {
		b, _ := json.Marshal(Report{
			Args: os.Args[1:], Engine: os.Getenv("CD2_DB_ENGINE"), Host: os.Getenv("CD2_DB_HOST"),
			Password: pw, HasKey: os.Getenv("CD2_CTL_KEY") != "", Sqlite: os.Getenv("CD2_DB_SQLITE"),
		})
		_ = os.WriteFile(f, b, 0o600)
	}
	fmt.Printf("INFO toolbox falso para %s\n", os.Getenv("CD2_DB_ENGINE"))
	switch {
	case strings.HasPrefix(pw, "morre"):
		fmt.Printf("ERROR conectando com a senha %s\n", unescape.Replace(pw))
		fmt.Printf("ERROR valor lido: %s\n", pw)
		fmt.Fprintln(os.Stderr, "Error: failed SASL auth: FATAL: Password authentication failed")
		fmt.Fprintln(os.Stderr, "Error: toolbox failed to initialize")
		os.Exit(1)
	case strings.HasPrefix(pw, "lenta"):
		time.Sleep(time.Hour)
		os.Exit(0)
	}
	isError := strings.HasPrefix(pw, "iserror")
	slow := strings.HasPrefix(pw, "demora")
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, "ok")
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var result any = map[string]any{}
		switch req.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "serverInfo": map[string]any{"name": "falso"}}
		case "tools/call":
			if slow {
				time.Sleep(3 * time.Second)
			}
			result = map[string]any{"content": []any{map[string]any{"type": "text", "text": `{"engine":"postgres"}`}}, "isError": isError}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	})
	err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", *port), mux)
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}
