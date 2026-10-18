package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"sync"

	"fyne.io/systray"

	"github.com/cd2tec/cd2-ia-de-bolso/instalacao/origem"

	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/icone"
	"github.com/cd2tec/cd2-ia-de-bolso/launcher/internal/sessao"
)

// Textos fixos do ícone da bandeja (barra de menus no Mac, perto do relógio
// no Windows).
const (
	nomeBandeja  = "IA de Bolso"
	itemMostrar  = "Mostrar a IA de Bolso"
	itemNova     = "Abrir uma nova janela da conversa"
	itemInstalar = "Instalar neste computador"

	// Só na instalada.
	itemInicio      = "Acordar quando o computador ligar"
	itemDesinstalar = "Desinstalar..."

	itemDormir   = "Colocar a IA para dormir"
	itemFechar   = "Fechar"
	ladoBandeja  = 44 // px; o Mac reduz para 16x16 pontos, o Windows para o tamanho da bandeja
	iconeCarrega = "carregando"
	iconeAcorda  = "acordada"
)

// item é uma linha do menu da bandeja. Marcado só vale para o item com
// marcação (o acordar com o computador).
type item struct {
	Texto   string
	Ligado  bool
	Marcado bool
}

// rotuloBandeja é o tooltip e o cabeçalho do menu.
func rotuloBandeja(e sessao.Estado) string {
	switch e.Fase {
	case sessao.Lendo, sessao.Subindo:
		return fmt.Sprintf("%s · carregando %d%%", nomeBandeja, e.Pct)
	case sessao.Pronta:
		return nomeBandeja + " · acordada"
	case sessao.Erro:
		return nomeBandeja + " · com problema"
	}
	return nomeBandeja
}

// itensBandeja diz quais itens valem agora, na ordem do menu. Mostrar vale
// sempre; a nova janela, só com a IA acordada; instalar, só acordada e do
// pen drive; dormir, até ela começar a dormir (no Erro, é o mesmo que o
// Fechar da janela).
func itensBandeja(e sessao.Estado, pendrive bool) []item {
	indoDormir := e.Fase == sessao.Dormindo || e.Fase == sessao.Dormiu
	itens := []item{
		{Texto: itemMostrar, Ligado: true},
		{Texto: itemNova, Ligado: e.Fase == sessao.Pronta},
	}
	if pendrive && e.Fase == sessao.Pronta {
		itens = append(itens, item{Texto: itemInstalar, Ligado: true})
	}
	return append(itens, item{Texto: itemDormir, Ligado: !indoDormir})
}

// itensBandejaInstalada é o menu da cópia instalada, na ordem do rascunho:
// o de sempre (sem instalar) com o acordar com o computador e o desinstalar
// antes do dormir. O acordar liga e desliga a qualquer hora; desinstalar,
// até a IA começar a dormir.
func itensBandejaInstalada(e sessao.Estado, inicioLigado bool) []item {
	base := itensBandeja(e, false)
	dormir := base[len(base)-1]
	indoDormir := e.Fase == sessao.Dormindo || e.Fase == sessao.Dormiu
	return append(base[:len(base)-1],
		item{Texto: itemInicio, Ligado: true, Marcado: inicioLigado},
		item{Texto: itemDesinstalar, Ligado: !indoDormir},
		dormir)
}

// itensDoMenu escolhe o menu de agora: trancada (ou com a desinstalação
// que falhou) só mostra ou fecha; a instalada tem os itens dela.
func (a *App) itensDoMenu(e sessao.Estado) []item {
	a.mu.Lock()
	trancada, ficou, o, ligado := a.trancada, a.ficou, a.origem, a.inicioLigado
	a.mu.Unlock()
	switch {
	case trancada || ficou:
		return itensBandejaTrancada()
	case o == origem.Instalada:
		return itensBandejaInstalada(e, ligado)
	}
	return itensBandeja(e, o == origem.Pendrive)
}

// iconeDoEstado escolhe o desenho: o cérebro só fica cheio e acordado em
// Pronta. No Erro fica o meio cheio, dormindo; o rótulo diz que há problema.
func iconeDoEstado(e sessao.Estado) string {
	if e.Fase == sessao.Pronta {
		return iconeAcorda
	}
	return iconeCarrega
}

// iconePNG centraliza o cérebro (10x8 pixels de arte) num quadrado de lado
// px, com fundo transparente: a bandeja do Mac e o ícone do app são quadrados
// e esticariam o desenho.
func iconePNG(estado string, lado int) []byte {
	escala := lado / 10
	if escala < 1 {
		escala = 1
	}
	src, err := png.Decode(bytes.NewReader(icone.Cerebro(estado, escala)))
	if err != nil {
		return icone.Cerebro(estado, escala)
	}
	dst := image.NewNRGBA(image.Rect(0, 0, lado, lado))
	b := src.Bounds()
	x, y := (lado-b.Dx())/2, (lado-b.Dy())/2
	draw.Draw(dst, image.Rect(x, y, x+b.Dx(), y+b.Dy()), src, b.Min, draw.Src)
	var buf bytes.Buffer
	_ = png.Encode(&buf, dst) // escrever em memória não falha
	return buf.Bytes()
}

// bandeja guarda os itens do menu depois que o ícone existe.
type bandeja struct {
	mu        sync.Mutex
	pronta    bool
	cabecalho *systray.MenuItem
	itens     map[string]*systray.MenuItem // pelo texto: o menu muda com a trava
	icone     string
}

// montarMenu roda quando o systray fica pronto. Cria o cabeçalho e os
// itens, aplica o estado de agora e passa a ouvir os cliques.
func (a *App) montarMenu() {
	b := &a.bandeja
	b.mu.Lock()
	b.cabecalho = systray.AddMenuItem(nomeBandeja, "")
	b.cabecalho.Disable()
	systray.AddSeparator()
	// Todos os itens existem desde o começo; o atualizarBandeja esconde os
	// que não valem (com a trava, só Mostrar e Fechar).
	// Na instalada entram o acordar com o computador e o desinstalar, entre
	// separadores (o separador não se esconde: a origem não muda depois).
	b.itens = map[string]*systray.MenuItem{}
	for _, texto := range []string{itemMostrar, itemNova, itemInstalar} {
		b.itens[texto] = systray.AddMenuItem(texto, "")
	}
	var inicioCh, desinstalarCh chan struct{} // nil no pen drive: nunca dispara
	if a.instalada() {
		systray.AddSeparator()
		a.mu.Lock()
		ligado := a.inicioLigado
		a.mu.Unlock()
		b.itens[itemInicio] = systray.AddMenuItemCheckbox(itemInicio, "", ligado)
		b.itens[itemDesinstalar] = systray.AddMenuItem(itemDesinstalar, "")
		systray.AddSeparator()
		inicioCh, desinstalarCh = b.itens[itemInicio].ClickedCh, b.itens[itemDesinstalar].ClickedCh
	}
	for _, texto := range []string{itemDormir, itemFechar} {
		b.itens[texto] = systray.AddMenuItem(texto, "")
	}
	b.pronta = true
	b.mu.Unlock()
	ligarCliqueSimples(a)
	a.atualizarBandeja(a.s.Atual())

	mostrar, nova, instalar := b.itens[itemMostrar], b.itens[itemNova], b.itens[itemInstalar]
	dormir, fechar := b.itens[itemDormir], b.itens[itemFechar]
	go func() {
		for {
			select {
			case <-mostrar.ClickedCh:
				a.chamar()
			case <-nova.ClickedCh:
				_ = a.s.AbrirConversa()
			case <-instalar.ClickedCh:
				_ = a.AbrirInstalar()
			case <-inicioCh:
				a.alternarInicio()
			case <-desinstalarCh:
				a.PedirDesinstalar()
			case <-dormir.ClickedCh:
				a.mostrar()
				a.Dormir()
			case <-fechar.ClickedCh:
				a.Fechar()
			}
		}
	}()
}

// atualizarBandeja põe o estado no tooltip, no cabeçalho, nos itens e no
// desenho. Antes de o systray ficar pronto não faz nada: o montarMenu aplica
// o estado de agora quando ficar.
func (a *App) atualizarBandeja(e sessao.Estado) {
	// Antes do b.mu: nunca segurar os dois locks juntos.
	r, itens := rotuloBandeja(e), a.itensDoMenu(e)
	if a.estaTrancada() {
		r = nomeBandeja
	}
	b := &a.bandeja
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.pronta {
		return
	}
	systray.SetTooltip(r)
	b.cabecalho.SetTitle(r)
	valem := map[string]bool{}
	for _, it := range itens {
		valem[it.Texto] = true
		m := b.itens[it.Texto]
		if m == nil {
			continue
		}
		m.Show()
		if it.Ligado {
			m.Enable()
		} else {
			m.Disable()
		}
		if it.Texto == itemInicio {
			if it.Marcado {
				m.Check()
			} else {
				m.Uncheck()
			}
		}
	}
	for texto, m := range b.itens {
		if !valem[texto] {
			m.Hide()
		}
	}
	if ic := iconeDoEstado(e); ic != b.icone {
		b.icone = ic
		systray.SetIcon(formatoIcone(iconePNG(ic, ladoBandeja)))
	}
}
