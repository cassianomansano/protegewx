// Package sysexec centraliza a execucao de comandos do Windows.
//
// Todo comando executado pelo ProtegeWX passa por aqui, e todo comando
// executado e registrado no log. Isso e o que sustenta a promessa do painel:
// nada roda sem estar visivel e auditavel.
package sysexec

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// Resultado do que aconteceu ao rodar um comando.
type Resultado struct {
	Linha    string        `json:"linha"`
	Saida    string        `json:"saida"`
	ExitCode int           `json:"exitCode"`
	Duracao  time.Duration `json:"-"`
	Erro     string        `json:"erro,omitempty"`
}

// Ok informa se o comando terminou com sucesso.
func (r Resultado) Ok() bool { return r.ExitCode == 0 && r.Erro == "" }

// Logger recebe cada comando executado. Definido pelo main.
var Logger func(Resultado)

// Formatar devolve a linha de comando como o usuario a veria num terminal.
// E exatamente esta string que o painel mostra antes de executar qualquer coisa.
func Formatar(programa string, args ...string) string {
	var b strings.Builder
	b.WriteString(programa)
	for _, a := range args {
		b.WriteString(" ")
		// so cita o que precisa, para a linha ficar legivel no painel
		if strings.ContainsAny(a, " \t\"") {
			b.WriteString(`"` + strings.ReplaceAll(a, `"`, `\"`) + `"`)
		} else {
			b.WriteString(a)
		}
	}
	return b.String()
}

// Rodar executa um comando e devolve o resultado, sem nunca abrir janela de console.
func Rodar(programa string, args ...string) Resultado {
	inicio := time.Now()
	res := Resultado{Linha: Formatar(programa, args...)}

	cmd := exec.Command(programa, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	res.Saida = strings.TrimSpace(out.String())
	res.Duracao = time.Since(inicio)

	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			res.ExitCode = ee.ExitCode()
		} else {
			res.ExitCode = -1
			res.Erro = err.Error()
		}
	}

	if Logger != nil {
		Logger(res)
	}
	return res
}

// RodarErr e como Rodar, mas devolve erro quando o comando falha.
func RodarErr(programa string, args ...string) (Resultado, error) {
	r := Rodar(programa, args...)
	if !r.Ok() {
		msg := r.Erro
		if msg == "" {
			msg = fmt.Sprintf("codigo de saida %d", r.ExitCode)
		}
		detalhe := r.Saida
		if len(detalhe) > 400 {
			detalhe = detalhe[:400] + "..."
		}
		return r, fmt.Errorf("%s: %s%s", r.Linha, msg, formatarDetalhe(detalhe))
	}
	return r, nil
}

func formatarDetalhe(d string) string {
	if d == "" {
		return ""
	}
	return " -- " + strings.ReplaceAll(d, "\r\n", " ")
}

// Netsh e o atalho usado pelas regras de firewall.
func Netsh(args ...string) Resultado { return Rodar("netsh", args...) }

// PowerShell executa um trecho de PowerShell sem carregar o profile do usuario.
func PowerShell(script string) Resultado {
	return Rodar("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
}
