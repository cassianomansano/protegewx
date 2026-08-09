//go:build !comunidade

// Package marca define a identidade da compilacao.
//
// Existem duas variantes, escolhidas por build tag:
//
//	(padrao)          versao padrao, com os dados completos
//	-tags comunidade  versao para distribuicao, sem logo propria e com os
//	                  numeros de serie dos dongles mascarados
//
// A separacao e por build tag, e nao por opcao de execucao, justamente para que
// o binario distribuido nao carregue dentro de si a marca nem os dados de ninguem.
package marca

const (
	// Comunidade indica se esta e a compilacao de distribuicao publica.
	Comunidade = false

	Nome       = "ProtegeWX"
	Subtitulo  = "Isolamento de rede dos dongles Sentinel e dos programas PC SOFT"
	Assinatura = "ProtegeWX"
)

// Mascarar devolve o identificador como deve aparecer na interface.
// Na versao padrao, o identificador aparece por inteiro.
func Mascarar(id string) string { return id }

