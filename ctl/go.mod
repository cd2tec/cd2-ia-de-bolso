module github.com/cd2tec/cd2-ia-de-bolso/ctl

go 1.23

require github.com/cd2tec/cd2-ia-de-bolso/instalacao v0.0.0

require (
	golang.org/x/sys v0.30.0 // indirect
	howett.net/plist v1.0.1 // indirect
)

replace github.com/cd2tec/cd2-ia-de-bolso/instalacao => ../instalacao
