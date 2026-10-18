package origem

// Valores de STORAGE_BUS_TYPE (BusType do STORAGE_DEVICE_DESCRIPTOR do Windows).
const (
	busTypeUsb = 7  // BusTypeUsb
	busTypeSd  = 12 // BusTypeSd
	busTypeMmc = 13 // BusTypeMmc
)

// barramentoRemovivel diz se o barramento é de pen drive ou cartão: USB, SD
// e MMC contam todos como USB na Regra.
func barramentoRemovivel(bus uint32) bool {
	return bus == busTypeUsb || bus == busTypeSd || bus == busTypeMmc
}
