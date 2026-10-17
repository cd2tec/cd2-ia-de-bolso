// Package icone gera o cérebro em pixel art (PNG e ICO) usado na bandeja e
// como ícone do app. Os pixels são os de core/robo.txt; o teste
// TestPixelsIguaisAoRoboTxt garante que as cópias daqui não se afastem.
package icone

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
)

// Grades copiadas das seções [cerebro], [sono] e [acordado] de core/robo.txt.
var (
	cerebroGrade = []string{
		"..pppppp..",
		".piiiiiip.",
		"piiiiiiiip",
		"piiiiiiiip",
		"piiiiiiiip",
		"piiiiiiiip",
		".piiiiiip.",
		"..pppppp..",
	}
	sonoGrade = []string{
		"..........",
		"..........",
		"..........",
		"..kk..kk..",
	}
	acordadoGrade = []string{
		"..........",
		"..........",
		"..ww..ww..",
		"..wk..wk..",
		"..k....k..",
		"...kkkk...",
	}
)

var (
	corContorno = color.NRGBA{0xa1, 0xa6, 0xae, 255}
	corCheio    = color.NRGBA{0xdb, 0xff, 0x2a, 255}
	corVazio    = color.NRGBA{0x1b, 0x20, 0x50, 255}
	corEscuro   = color.NRGBA{0x0d, 0x12, 0x33, 255}
	corBranco   = color.NRGBA{0xff, 0xff, 0xff, 255}
	primeira    = 1 // primeira e última linha do miolo do cérebro (as de fora são só contorno)
)

// Cerebro devolve um PNG do cérebro. estado "acordada" é o cérebro cheio com
// rosto acordado; qualquer outro valor (em especial "carregando") é o cérebro
// meio cheio, só as 3 linhas de baixo do miolo, com o rosto dormindo.
func Cerebro(estado string, escala int) []byte {
	if escala < 1 {
		escala = 1
	}
	acordada := estado == "acordada"
	rosto := sonoGrade
	if acordada {
		rosto = acordadoGrade
	}

	altura, largura := len(cerebroGrade), len(cerebroGrade[0])
	img := image.NewNRGBA(image.Rect(0, 0, largura*escala, altura*escala))
	pintar := func(x, y int, c color.NRGBA) {
		for dy := 0; dy < escala; dy++ {
			for dx := 0; dx < escala; dx++ {
				img.SetNRGBA(x*escala+dx, y*escala+dy, c)
			}
		}
	}

	for y, linha := range cerebroGrade {
		for x := 0; x < len(linha); x++ {
			switch linha[x] {
			case 'p':
				pintar(x, y, corContorno)
			case 'i':
				if acordada || y >= primeira+3 {
					pintar(x, y, corCheio)
				} else {
					pintar(x, y, corVazio)
				}
			}
		}
	}
	// O rosto fica por cima, na mesma origem; '.' não pinta nada.
	for y, linha := range rosto {
		for x := 0; x < len(linha); x++ {
			switch linha[x] {
			case 'k':
				pintar(x, y, corEscuro)
			case 'w':
				pintar(x, y, corBranco)
			}
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img) // escrever em memória não falha
	return buf.Bytes()
}

// ICO embrulha um PNG num .ico de uma imagem só (o Windows aceita PNG dentro
// do ICO). Largura e altura 0 no diretório significam 256 ou mais.
func ICO(pngBytes []byte) []byte {
	var largura, altura byte
	if cfg, err := png.DecodeConfig(bytes.NewReader(pngBytes)); err == nil {
		if cfg.Width < 256 {
			largura = byte(cfg.Width)
		}
		if cfg.Height < 256 {
			altura = byte(cfg.Height)
		}
	}
	var b bytes.Buffer
	b.Write([]byte{0, 0, 1, 0, 1, 0}) // reservado, tipo 1 (ícone), 1 imagem
	b.Write([]byte{largura, altura, 0, 0})
	_ = binary.Write(&b, binary.LittleEndian, uint16(1))  // planos
	_ = binary.Write(&b, binary.LittleEndian, uint16(32)) // bits por pixel
	_ = binary.Write(&b, binary.LittleEndian, uint32(len(pngBytes)))
	_ = binary.Write(&b, binary.LittleEndian, uint32(22)) // 6 do cabeçalho + 16 da entrada
	b.Write(pngBytes)
	return b.Bytes()
}
