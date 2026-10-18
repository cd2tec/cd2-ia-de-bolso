//go:build windows

package origem

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	ioctlStorageQueryProperty = 0x002D1400 // IOCTL_STORAGE_QUERY_PROPERTY
	storageDeviceProperty     = 0          // StorageDeviceProperty
	propertyStandardQuery     = 0          // PropertyStandardQuery
	// BusType fica no byte 28 de STORAGE_DEVICE_DESCRIPTOR: Version, Size,
	// quatro bytes de tipo e fila, e quatro deslocamentos de 32 bits.
	deslocBusType = 28
)

// storagePropertyQuery é o STORAGE_PROPERTY_QUERY (com um byte de parâmetro).
type storagePropertyQuery struct {
	PropertyID uint32
	QueryType  uint32
	Extra      [1]byte
}

func lerVolumeDoSistema(raiz string) (Volume, error) {
	abs, err := filepath.Abs(raiz)
	if err != nil {
		return Volume{}, err
	}
	letra := filepath.VolumeName(abs)
	if len(letra) != 2 || letra[1] != ':' {
		return Volume{}, errors.New("a pasta não está em uma letra de unidade")
	}
	sistema := os.Getenv("SystemDrive")
	if sistema == "" {
		return Volume{}, errors.New("SystemDrive não definido")
	}
	if strings.EqualFold(letra, sistema) {
		return Volume{DoSistema: true}, nil
	}
	raizVolume, err := windows.UTF16PtrFromString(letra + `\`)
	if err != nil {
		return Volume{}, err
	}
	v := Volume{Removivel: windows.GetDriveType(raizVolume) == windows.DRIVE_REMOVABLE}
	usb, err := ehUSB(letra)
	if err != nil {
		return Volume{}, err
	}
	v.USB = usb
	return v, nil
}

// ehUSB abre o volume sem pedir acesso e pergunta o tipo de barramento
// (USB, SD ou MMC contam como USB).
func ehUSB(letra string) (bool, error) {
	nome, err := windows.UTF16PtrFromString(`\\.\` + letra)
	if err != nil {
		return false, err
	}
	h, err := windows.CreateFile(nome, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(h)
	q := storagePropertyQuery{PropertyID: storageDeviceProperty, QueryType: propertyStandardQuery}
	var buf [1024]byte
	var n uint32
	err = windows.DeviceIoControl(h, ioctlStorageQueryProperty,
		(*byte)(unsafe.Pointer(&q)), uint32(unsafe.Sizeof(q)),
		&buf[0], uint32(len(buf)), &n, nil)
	if err != nil {
		return false, err
	}
	if n < deslocBusType+4 {
		return false, errors.New("resposta curta do STORAGE_DEVICE_DESCRIPTOR")
	}
	bus := *(*uint32)(unsafe.Pointer(&buf[deslocBusType]))
	return barramentoRemovivel(bus), nil
}
