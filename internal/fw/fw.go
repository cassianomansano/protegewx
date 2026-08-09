// Package fw cria e remove regras do Firewall do Windows.
//
// Toda regra criada pelo ProtegeWX leva o prefixo "PROTEGEWX - ", o que torna a
// reversao e a auditoria triviais: basta listar ou apagar por esse prefixo.
//
// Ponto importante do desenho: bloquear a saida do hasplms.exe NAO afeta os
// dongles. O Windows Filtering Platform nao filtra trafego de loopback, e a API
// do WinDev fala com o License Manager em 127.0.0.1:1947. O bloqueio atinge
// apenas o que tentaria sair da maquina.
package fw

import (
	"fmt"
	"strings"

	"protegewx/internal/sysexec"
)

// Prefixo identifica todas as regras gerenciadas por esta ferramenta.
const Prefixo = "PROTEGEWX - "

// Regra descreve uma regra de firewall que o ProtegeWX sabe criar e remover.
type Regra struct {
	Nome      string // sem o prefixo
	Direcao   string // in | out
	Protocolo string // TCP | UDP | any
	PortaLoc  string // porta local (regras de entrada)
	PortaRem  string // porta remota (regras de saida)
	Programa  string // caminho completo do executavel
	Descricao string
}

func (r Regra) nomeCompleto() string { return Prefixo + r.Nome }

func (r Regra) args() []string {
	a := []string{
		"advfirewall", "firewall", "add", "rule",
		"name=" + r.nomeCompleto(),
		"dir=" + r.Direcao,
		"action=block",
		"enable=yes",
		"profile=any",
	}
	if r.Protocolo != "" && r.Protocolo != "any" {
		a = append(a, "protocol="+r.Protocolo)
	}
	if r.PortaLoc != "" {
		a = append(a, "localport="+r.PortaLoc)
	}
	if r.PortaRem != "" {
		a = append(a, "remoteport="+r.PortaRem)
	}
	if r.Programa != "" {
		a = append(a, "program="+r.Programa)
	}
	if r.Descricao != "" {
		a = append(a, "description="+r.Descricao)
	}
	return a
}

// Comando devolve a linha de comando exata, para exibicao no painel antes de executar.
func (r Regra) Comando() string { return sysexec.Formatar("netsh", r.args()...) }

// Criar adiciona a regra. Remove antes uma eventual regra homonima, para a
// operacao ser idempotente.
func Criar(r Regra) error {
	_ = Remover(r.Nome)
	_, err := sysexec.RodarErr("netsh", r.args()...)
	return err
}

// Remover apaga a regra pelo nome (sem prefixo).
func Remover(nome string) error {
	res := sysexec.Netsh("advfirewall", "firewall", "delete", "rule", "name="+Prefixo+nome)
	if res.Ok() {
		return nil
	}
	if naoEncontrada(res.Saida) {
		return nil // ja nao existia
	}
	return fmt.Errorf("%s: %s", res.Linha, res.Saida)
}

// Existe informa se a regra esta presente no firewall.
func Existe(nome string) bool {
	res := sysexec.Netsh("advfirewall", "firewall", "show", "rule", "name="+Prefixo+nome)
	return res.Ok() && !naoEncontrada(res.Saida)
}

// naoEncontrada reconhece a resposta "nenhuma regra corresponde" do netsh.
// O netsh e localizado, entao checamos tambem o codigo de saida em Existe/Remover
// e aqui cobrimos as mensagens de pt-BR e en-US.
func naoEncontrada(saida string) bool {
	s := strings.ToLower(saida)
	return strings.Contains(s, "no rules match") ||
		strings.Contains(s, "nenhuma regra") ||
		strings.Contains(s, "corresponde aos criterios") ||
		strings.Contains(s, "corresponde aos critérios")
}

// ---------------------------------------------------------------- regras pre-existentes

// Habilitar liga ou desliga uma regra que ja existia na maquina (nao criada por nos).
// Usamos isso para neutralizar as regras de entrada da 1947 sem apaga-las,
// preservando a possibilidade de revert.
func Habilitar(nomeExato string, ligada bool) error {
	estado := "no"
	if ligada {
		estado = "yes"
	}
	res := sysexec.Netsh("advfirewall", "firewall", "set", "rule",
		"name="+nomeExato, "new", "enable="+estado)
	if res.Ok() {
		return nil
	}
	if naoEncontrada(res.Saida) {
		return nil // a regra nao existe nesta maquina; nada a fazer
	}
	return fmt.Errorf("%s: %s", res.Linha, res.Saida)
}

// ComandoHabilitar devolve a linha exibida no painel.
func ComandoHabilitar(nomeExato string, ligada bool) string {
	estado := "no"
	if ligada {
		estado = "yes"
	}
	return sysexec.Formatar("netsh", "advfirewall", "firewall", "set", "rule",
		"name="+nomeExato, "new", "enable="+estado)
}

// RegraLigada informa se uma regra pre-existente esta habilitada.
// Devolve (ligada, existe).
func RegraLigada(nomeExato string) (bool, bool) {
	res := sysexec.PowerShell(
		`$r = Get-NetFirewallRule -DisplayName '` + strings.ReplaceAll(nomeExato, "'", "''") +
			`' -ErrorAction SilentlyContinue; if ($r) { ($r | Select-Object -First 1).Enabled } else { 'AUSENTE' }`)
	s := strings.TrimSpace(res.Saida)
	if s == "AUSENTE" || s == "" {
		return false, false
	}
	return strings.EqualFold(s, "True"), true
}

// Listar devolve os nomes das regras criadas pelo ProtegeWX.
func Listar() []string {
	res := sysexec.PowerShell(
		`Get-NetFirewallRule -ErrorAction SilentlyContinue | ` +
			`Where-Object { $_.DisplayName -like '` + Prefixo + `*' } | ` +
			`Select-Object -ExpandProperty DisplayName`)
	var nomes []string
	for _, l := range strings.Split(res.Saida, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			nomes = append(nomes, l)
		}
	}
	return nomes
}
