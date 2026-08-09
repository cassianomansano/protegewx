//go:build !comunidade

// Package protegewx expoe os arquivos do painel embutidos no executavel.
//
// O embed precisa morar na raiz do modulo porque a diretiva //go:embed nao
// consegue subir diretorios a partir de cmd/protegewx.
//
// Esta variante embute web/, que contem a identidade padrao. A variante de
// distribuicao publica esta em web_comunidade.go e embute webcom/ - assim a
// logo propria nao acompanha o binario distribuido.
package protegewx

import (
	"embed"
	"io/fs"
)

//go:embed all:web
var arquivos embed.FS

// Painel devolve o conteudo do painel pronto para ser servido.
func Painel() (fs.FS, error) { return fs.Sub(arquivos, "web") }

