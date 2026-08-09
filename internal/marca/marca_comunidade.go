//go:build comunidade

package marca

const (
	Comunidade = true

	Nome       = "ProtegeWX"
	Subtitulo  = "Isolamento de rede dos dongles Sentinel e dos programas PC SOFT"
	Assinatura = "Comunidade Mais Guerreira do Mundo — Programadores"
)

// Mascarar esconde o numero de serie do dongle, preservando apenas o suficiente
// para a pessoa distinguir uma chave da outra na propria tela.
//
// Mostra so os dois primeiros digitos, e nenhum do fim: revelar as duas pontas
// de um identificador curto deixa poucas posicoes desconhecidas no meio, o que
// torna o numero adivinhavel por forca bruta a partir de um print de tela.
//
// A quantidade de pontos e fixa e nao acompanha o tamanho do identificador,
// para nao entregar nem quantos digitos ele tem.
func Mascarar(id string) string {
	const visivel = 2
	const oculto = "••••••••"

	r := []rune(id)
	if len(r) <= visivel {
		return oculto
	}
	return string(r[:visivel]) + oculto
}
