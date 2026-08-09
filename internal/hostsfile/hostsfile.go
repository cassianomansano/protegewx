// Package hostsfile gerencia um bloco delimitado dentro do arquivo hosts do Windows.
//
// Tudo que o ProtegeWX escreve fica entre marcadores proprios, de modo que a
// reversao remove exatamente o que foi adicionado e nao toca em mais nada.
//
// Limites honestos desta camada, exibidos tambem no painel:
//   - o hosts nao aceita curinga: nao existe "*.pcsoft.fr"
//   - nao impede conexao feita direto por endereco IP
//   - e ignorado por navegadores que usam DNS-over-HTTPS
//
// Por isso o hosts aqui e reforco. A camada principal e o firewall.
package hostsfile

import (
	"os"
	"path/filepath"
	"strings"

	"protegewx/internal/marca"
)

const (
	inicio = "# >>> PROTEGEWX >>>"
	fim    = "# <<< PROTEGEWX <<<"
)

// Caminho devolve o hosts do sistema.
func Caminho() string {
	raiz := os.Getenv("SystemRoot")
	if raiz == "" {
		raiz = `C:\Windows`
	}
	return filepath.Join(raiz, "System32", "drivers", "etc", "hosts")
}

// Grupo e um conjunto nomeado de dominios a bloquear.
type Grupo struct {
	Nome     string
	Dominios []string
}

func ler() (string, error) {
	b, err := os.ReadFile(Caminho())
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

// semBloco devolve o conteudo do hosts sem o trecho gerenciado pelo ProtegeWX.
func semBloco(conteudo string) string {
	i := strings.Index(conteudo, inicio)
	if i < 0 {
		return conteudo
	}
	j := strings.Index(conteudo[i:], fim)
	if j < 0 {
		return strings.TrimRight(conteudo[:i], "\r\n") + "\n"
	}
	resto := conteudo[i+j+len(fim):]
	return strings.TrimRight(conteudo[:i], "\r\n") + "\n" + strings.TrimLeft(resto, "\r\n")
}

// Aplicado informa se o bloco esta presente no hosts.
func Aplicado() bool {
	c, err := ler()
	return err == nil && strings.Contains(c, inicio)
}

// DominiosAtivos devolve os dominios atualmente bloqueados pelo nosso bloco.
func DominiosAtivos() []string {
	c, err := ler()
	if err != nil {
		return nil
	}
	i := strings.Index(c, inicio)
	if i < 0 {
		return nil
	}
	j := strings.Index(c[i:], fim)
	if j < 0 {
		return nil
	}
	var ds []string
	for _, l := range strings.Split(c[i:i+j], "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if campos := strings.Fields(l); len(campos) >= 2 {
			ds = append(ds, campos[1])
		}
	}
	return ds
}

// Montar produz o texto do bloco para os grupos informados, sem grava-lo.
// E o que o painel mostra antes de aplicar.
func Montar(grupos []Grupo) string {
	var b strings.Builder
	b.WriteString(inicio + "\n")
	b.WriteString("# Bloqueio de telemetria - " + marca.Nome + "\n")
	b.WriteString("# Remova este bloco inteiro para reverter.\n")
	for _, g := range grupos {
		if len(g.Dominios) == 0 {
			continue
		}
		b.WriteString("#\n# " + g.Nome + "\n")
		for _, d := range g.Dominios {
			b.WriteString("0.0.0.0 " + d + "\n")
			// muitos clientes tentam www. antes do dominio nu
			if !strings.HasPrefix(d, "www.") && strings.Count(d, ".") == 1 {
				b.WriteString("0.0.0.0 www." + d + "\n")
			}
		}
	}
	b.WriteString(fim + "\n")
	return b.String()
}

// Aplicar substitui o bloco pelo conteudo correspondente aos grupos informados.
func Aplicar(grupos []Grupo) error {
	atual, err := ler()
	if err != nil {
		return err
	}
	limpo := semBloco(atual)
	if !strings.HasSuffix(limpo, "\n") {
		limpo += "\n"
	}
	novo := limpo + "\n" + Montar(grupos)
	if err := os.WriteFile(Caminho(), []byte(novo), 0o644); err != nil {
		return err
	}
	limparCacheDNS()
	return nil
}

// Remover tira o bloco do hosts, deixando o resto do arquivo intacto.
func Remover() error {
	atual, err := ler()
	if err != nil {
		return err
	}
	if !strings.Contains(atual, inicio) {
		return nil
	}
	if err := os.WriteFile(Caminho(), []byte(semBloco(atual)), 0o644); err != nil {
		return err
	}
	limparCacheDNS()
	return nil
}
