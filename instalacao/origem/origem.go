// Package origem descobre de onde a IA está rodando: de um pen drive (ou
// disco externo) ou instalada no disco do computador.
package origem

import "strings"

// Origem é o lugar de onde a IA roda. As strings são consumidas pela web.
type Origem string

const (
	Pendrive     Origem = "pendrive"
	Instalada    Origem = "instalada"
	Desconhecida Origem = "desconhecida"
)

// Volume descreve o disco onde a pasta da IA está.
type Volume struct {
	DoSistema, Removivel, USB, Externo bool
}

// Regra decide a origem a partir do volume. Disco do sistema é sempre
// instalada; removível, USB ou externo é pen drive; qualquer outro (um
// segundo disco interno) é instalada.
func Regra(v Volume) Origem {
	if v.DoSistema {
		return Instalada
	}
	if v.Removivel || v.USB || v.Externo {
		return Pendrive
	}
	return Instalada
}

// lerVolume fica atrás de uma variável para o teste trocar.
var lerVolume = lerVolumeDoSistema

// Detectar devolve a origem da pasta raiz. Na dúvida (erro ao ler o
// volume), é Pendrive: nunca tranca quem está usando.
func Detectar(raiz string) Origem {
	v, err := lerVolume(raiz)
	if err != nil {
		return Pendrive
	}
	return Regra(v)
}

// montagemDoSistema diz se um ponto de montagem do Mac é o disco do sistema.
func montagemDoSistema(ponto string) bool {
	return ponto == "/" || strings.HasPrefix(ponto, "/System/Volumes/")
}
