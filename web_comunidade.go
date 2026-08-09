//go:build comunidade

package protegewx

import (
	"embed"
	"io/fs"
)

//go:embed all:webcom
var arquivos embed.FS

// Painel devolve o conteudo do painel pronto para ser servido.
func Painel() (fs.FS, error) { return fs.Sub(arquivos, "webcom") }
