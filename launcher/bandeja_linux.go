package main

import (
	"os/exec"
	"strings"

	"fyne.io/systray"
	"github.com/godbus/dbus/v5"
)

// No Linux o ícone vai para a bandeja pelo StatusNotifierItem (DBus), o que o
// Ubuntu (extensão AppIndicator), o KDE, o XFCE e outros mostram. O GNOME sem
// essa extensão (Debian, por exemplo) não mostra: aí o X da janela põe a IA
// para dormir (temIcone, em antesDeFechar), como o Cmd+Q no Mac.

// O X da janela passa pelo OnBeforeClose, como no Windows: esconde com o
// ícone à vista e dorme sem ele.
const escondePeloAntesDeFechar = true

// Texto da notificação da primeira vez que a janelinha fecha (a confirmar).
const textoAvisoBandeja = "Continuo acordada no ícone da barra de cima. Me chama quando quiser."

// iniciarBandeja põe o ícone na bandeja. No Linux o systray só fala DBus,
// sem laço de mensagens próprio: roda numa goroutine, sem disputar com o GTK
// do Wails. O onExit roda no systray.Quit: a IA dorme antes de o processo acabar.
func iniciarBandeja(a *App) {
	temIcone = temBandeja
	go systray.Run(a.montarMenu, a.s.Dormir)
}

// temBandeja diz se há quem mostre o ícone: o StatusNotifierWatcher no DBus
// da sessão. Conferido a cada fechar, porque a extensão pode ser ligada depois.
func temBandeja() bool {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return false
	}
	defer conn.Close()
	var dono bool
	err = conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, "org.kde.StatusNotifierWatcher").Store(&dono)
	return err == nil && dono
}

// No KDE e no XFCE o clique simples traz a janela e o botão direito abre o
// menu; no Ubuntu os dois abrem o menu. O chamar vai numa goroutine para não
// segurar o DBus.
func ligarCliqueSimples(a *App) { systray.SetOnTapped(func() { go a.chamar() }) }

// O systray do Linux aceita PNG.
func formatoIcone(png []byte) []byte { return png }

// O WindowShow do Wails já traz a janela para a frente.
func trazerParaFrente() {}

// O systray some no Quit.
func fecharBandeja() { systray.Quit() }

// A bandeja do Linux não mostra texto ao lado do ícone: a marca é o círculo
// vermelho no desenho, como no Windows.
const marcaNoIcone = true

// No Linux os avisos dos agendamentos já saem pelo cd2-ctl
// (ctl/notificar_linux.go), que roda os agendamentos só na cópia instalada.
// O lançador não repete o aviso, senão ele sairia duas vezes.
func notificar(_, _, _ string) {}

// avisarPrimeiroFechar avisa que a IA continua no ícone. Devolve se o aviso
// saiu; sem ícone o X nem esconde (dorme), então não há o que avisar.
func avisarPrimeiroFechar() bool {
	if !temBandeja() {
		return false
	}
	return enviarNotificacao(nomeBandeja, textoAvisoBandeja) == nil
}

// enviarNotificacao usa o notify-send (pacote libnotify-bin) e, sem ele, o
// gdbus direto no serviço de notificações do desktop. É o mesmo caminho do
// cd2-ctl. Em variável para o teste.
var enviarNotificacao = func(titulo, texto string) error {
	if caminho, err := exec.LookPath("notify-send"); err == nil {
		return exec.Command(caminho, "--app-name="+nomeBandeja, "--icon=dialog-information", titulo, texto).Run()
	}
	return exec.Command("gdbus", argsGdbus(titulo, texto)...).Run()
}

// argsGdbus monta a chamada org.freedesktop.Notifications.Notify. O gdbus lê
// cada argumento como GVariant em texto: as strings vão entre aspas simples, escapadas.
func argsGdbus(titulo, texto string) []string {
	return []string{"call", "--session", "--dest", "org.freedesktop.Notifications",
		"--object-path", "/org/freedesktop/Notifications", "--method", "org.freedesktop.Notifications.Notify",
		gvTexto(nomeBandeja), "uint32 0", gvTexto("dialog-information"), gvTexto(titulo), gvTexto(texto),
		"@as []", "@a{sv} {}", "int32 -1"}
}

func gvTexto(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}
