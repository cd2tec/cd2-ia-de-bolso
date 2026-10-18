package sistema

// nenhum é o Sistema de onde a instalação não existe (Linux e outros).
type nenhum struct{}

func (nenhum) Atalhos(string, bool) error       { return ErrSistema }
func (nenhum) LigarInicio(string, bool) error   { return ErrSistema }
func (nenhum) InicioLigado() bool               { return false }
func (nenhum) Registrar(string, string) error   { return ErrSistema }
func (nenhum) Remover() error                   { return ErrSistema }
func (nenhum) PlanoRemocao(string, int) Comando { return Comando{} }
