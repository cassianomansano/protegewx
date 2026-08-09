package actions

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"protegewx/internal/fw"
	"protegewx/internal/hostsfile"
	"protegewx/internal/sentinel"
	"protegewx/internal/sysexec"
)

// ---------------------------------------------------------------- caminhos

const tarefaNome = "ProtegeWX Monitor"

// raizesPCSoft sao os lugares onde a PC SOFT costuma instalar seus produtos.
var raizesPCSoft = []string{
	`C:\PC SOFT`,
	os.Getenv("ProgramFiles") + `\PC SOFT`,
	os.Getenv("ProgramFiles(x86)") + `\PC SOFT`,
}

// produtosPCSoft descobre os produtos instalados nesta maquina.
//
// A lista e montada por varredura, e nao fixada no codigo: assim o programa
// funciona em qualquer maquina, com qualquer versao (27, 28, 2024...) e com
// qualquer combinacao de WINDEV, WEBDEV e WINDEV Mobile instalada.
func produtosPCSoft() []string {
	var v []string
	for _, raiz := range raizesPCSoft {
		if raiz == `\PC SOFT` {
			continue // variavel de ambiente vazia
		}
		entradas, err := os.ReadDir(raiz)
		if err != nil {
			continue
		}
		for _, e := range entradas {
			if !e.IsDir() {
				continue
			}
			caminho := filepath.Join(raiz, e.Name())
			// so interessa a pasta que de fato tem um produto de desenvolvimento
			if temAlgumBinario(caminho) {
				v = append(v, caminho)
			}
		}
	}
	sort.Strings(v)
	return v
}

// relativosAtualizacao sao os executaveis que falam com a PC SOFT ou que
// aplicam atualizacao ao runtime e ao dongle, relativos a pasta do produto.
var relativosAtualizacao = []string{
	`Install\AutomaticUpdate\AutomaticUpdate.exe`,
	`Programmes\haspdinst.exe`,
	`Programmes\WDUpdate.exe`,
	`Install\haspdinst.exe`,
}

func temAlgumBinario(pastaProduto string) bool {
	for _, rel := range relativosAtualizacao {
		if _, err := os.Stat(filepath.Join(pastaProduto, rel)); err == nil {
			return true
		}
	}
	return false
}

// regrasEntradaExistentes sao regras que ja existiam na maquina abrindo a 1947.
// Nos as desabilitamos em vez de apagar, para a reversao ser fiel.
var regrasEntradaExistentes = []string{
	"HASP PCSOFT TCP",
	"HASP PCSOFT UDP",
	"Sentinel License Manager",
}

func binariosLM() []string {
	var v []string
	for _, n := range []string{"hasplms.exe", "hasplmv.exe"} {
		p := filepath.Join(sentinel.PastaLM(), n)
		if _, err := os.Stat(p); err == nil {
			v = append(v, p)
		}
	}
	return v
}

// binariosAtualizacao lista os executaveis que falam com a PC SOFT ou que
// aplicam atualizacao ao runtime/dongle, em todos os produtos encontrados.
func binariosAtualizacao() []string {
	var v []string
	for _, prod := range produtosPCSoft() {
		for _, rel := range relativosAtualizacao {
			p := filepath.Join(prod, rel)
			if _, err := os.Stat(p); err == nil {
				v = append(v, p)
			}
		}
	}
	return v
}

// ---------------------------------------------------------------- grupos

func Grupos() []Grupo {
	return []Grupo{
		{
			ID:        "A",
			Nome:      "Sentinel License Manager",
			Resumo:    "Faz o gerenciador de licencas atender somente esta maquina, parando de anunciar e de procurar chaves na rede.",
			Principal: true,
		},
		{
			ID:        "B",
			Nome:      "Firewall do Windows",
			Resumo:    "Fecha a porta 1947 para a rede e impede que os programas de licenca e de atualizacao saiam da maquina.",
			Ressalva:  "O trafego de loopback (127.0.0.1) nao e filtrado pelo Windows, entao os dongles continuam funcionando normalmente.",
			Principal: true,
		},
		{
			ID:       "C",
			Nome:     "Bloqueio de dominios (hosts)",
			Resumo:   "Aponta os dominios de telemetria da PC SOFT e da Thales para o vazio.",
			Ressalva: "Camada de reforco apenas. O arquivo hosts nao aceita curinga, nao impede conexao por IP direto e e ignorado por navegadores com DNS-over-HTTPS. A protecao efetiva vem do firewall.",
		},
		{
			ID:        "D",
			Nome:      "Protecao contra atualizacao de licenca",
			Resumo:    "Impede a aplicacao de um arquivo V2C - o unico caminho real de desativacao das suas chaves - e vigia mudancas no conteudo das licencas.",
			Principal: true,
		},
	}
}

// ---------------------------------------------------------------- grupo A

// aplicarAjuste altera uma opcao de isolamento, regrava o INI e confere se o
// efeito aconteceu de verdade.
//
// As opcoes vem do estado persistido, nunca deduzidas do ACC: ha opcoes que o
// painel do Sentinel nao reflete de volta, e deduzi-las faria cada acao apagar
// silenciosamente o que a anterior tinha pedido.
func aplicarAjuste(c *Ctx, ajuste func(*OpcoesLM)) error {
	lm, err := c.AjustarLM(ajuste)
	if err != nil {
		return err
	}
	o := sentinel.OpcoesIsolamento{
		SomenteLoopback: lm.SomenteLoopback && !c.BindLocalIndisponivel(),
		RecusarRemoto:   lm.RecusarRemoto,
		NaoProcurar:     lm.NaoProcurar,
		SemBroadcast:    lm.SemBroadcast,
		Logs:            lm.Logs,
		SenhaACC:        c.SenhaACC(),
	}
	return aplicarLM(c, o)
}

// aplicarLM grava o INI, reinicia o servico e confere se o efeito aconteceu de
// verdade. Se nao aconteceu, restaura o estado anterior e informa a falha.
func aplicarLM(c *Ctx, o sentinel.OpcoesIsolamento) error {
	anterior, errBackup := os.ReadFile(sentinel.CaminhoINI())
	existia := errBackup == nil

	if err := sentinel.Aplicar(o); err != nil {
		return err
	}

	divs, err := sentinel.Verificar(o)
	if err != nil {
		return fmt.Errorf("configuracao aplicada, mas nao foi possivel conferir o resultado: %w", err)
	}

	// O pedido de escutar so em loopback nao existe em todos os runtimes.
	// Quando este e o unico ponto que nao pegou, registramos a limitacao,
	// mantemos o resto da configuracao (que funcionou) e explicamos o caso -
	// em vez de descartar tudo por causa de uma opcao que este Sentinel nao tem.
	if len(divs) == 1 && strings.Contains(divs[0].Opcao, "loopback") {
		if err := c.MarcarBindLocalIndisponivel(); err != nil {
			return err
		}
		return fmt.Errorf("este runtime Sentinel (%s) ignora o pedido de escutar apenas em 127.0.0.1: "+
			"a opcao nao existe nesta versao. As demais opcoes foram aplicadas. "+
			"O bloqueio da porta para a rede fica a cargo do firewall (acoes B1 e B2), que ja cobre este ponto",
			divs[0].Observado)
	}

	if len(divs) == 0 {
		return nil
	}

	// nao pegou: desfaz para nao deixar a maquina num meio-termo silencioso
	if existia {
		_ = os.WriteFile(sentinel.CaminhoINI(), anterior, 0o644)
	} else {
		_ = os.Remove(sentinel.CaminhoINI())
	}
	_ = sentinel.ReiniciarServico()

	var msg []string
	for _, d := range divs {
		msg = append(msg, fmt.Sprintf("%s (esperado: %s / observado: %s)", d.Opcao, d.Esperado, d.Observado))
	}
	return fmt.Errorf("esta versao do runtime Sentinel nao aceitou a configuracao; "+
		"o estado anterior foi restaurado. Nao aplicado: %s", strings.Join(msg, "; "))
}

func estadoLM(leitura func(sentinel.Config) bool) func(*Ctx) Estado {
	return func(*Ctx) Estado {
		cfg, err := sentinel.LerConfig()
		if err != nil {
			return Indisponivel
		}
		if leitura(cfg) {
			return Aplicado
		}
		return NaoAplicado
	}
}

func acoesGrupoA() []Acao {
	cmdINI := func(nota string) func(*Ctx) []string {
		return func(*Ctx) []string {
			return []string{
				"escrever " + sentinel.CaminhoINI() + "  (" + nota + ")",
				"sc stop hasplms",
				"sc start hasplms",
				"conferir o resultado no proprio Admin Control Center",
			}
		}
	}

	return []Acao{
		{
			ID: "A1", Grupo: "A", Risco: RiscoBaixo, Padrao: true,
			Titulo:  "Escutar somente em 127.0.0.1",
			OQueFaz: "Pede ao License Manager que abra a porta 1947 apenas no loopback, em vez de em todas as interfaces de rede.",
			PorQue:  "A 1947 escuta em 0.0.0.0, ou seja, o socket esta aberto para a rede. Nem todo runtime Sentinel oferece esta opcao; quando ela nao existe, o fechamento da porta fica por conta do firewall (acoes B1 e B2), que produz o mesmo efeito pratico.",
			Reverte: "Apagar o hasplm.ini e reiniciar o servico hasplms.",
			Comandos: cmdINI("bindlocalonly = 1"),
			Aplicar: func(c *Ctx) error {
				if c.BindLocalIndisponivel() {
					return fmt.Errorf("este runtime Sentinel nao possui a opcao de escutar apenas em loopback; " +
						"a porta ja esta bloqueada para a rede pelas acoes B1 e B2")
				}
				return aplicarAjuste(c, func(o *OpcoesLM) { o.SomenteLoopback = true })
			},
			Reverter: func(c *Ctx) error {
				return aplicarAjuste(c, func(o *OpcoesLM) { o.SomenteLoopback = false })
			},
			Ler: func(c *Ctx) Estado {
				if c.BindLocalIndisponivel() {
					return Indisponivel
				}
				exposta, _, err := sentinel.EscutaExposta()
				if err != nil {
					return Indisponivel
				}
				if exposta {
					return NaoAplicado
				}
				return Aplicado
			},
		},
		{
			ID: "A2", Grupo: "A", Risco: RiscoBaixo, Padrao: true,
			Titulo:  "Recusar clientes remotos",
			OQueFaz: "Impede que outras maquinas consumam licenca deste dongle ou abram o painel de administracao.",
			PorQue:  "Voce usa os dongles somente nesta maquina. Servir licenca para fora e superficie de ataque sem contrapartida.",
			Reverte: "Apagar o hasplm.ini e reiniciar o servico hasplms.",
			Comandos: cmdINI("accremote = 0"),
			Aplicar: func(c *Ctx) error {
				return aplicarAjuste(c, func(o *OpcoesLM) { o.RecusarRemoto = true })
			},
			Reverter: func(c *Ctx) error {
				return aplicarAjuste(c, func(o *OpcoesLM) { o.RecusarRemoto = false })
			},
			Ler: estadoLM(func(cfg sentinel.Config) bool { return !cfg.AceitaRemoto }),
		},
		{
			ID: "A3", Grupo: "A", Risco: RiscoBaixo, Padrao: true,
			Titulo:  "Parar de procurar licencas remotas",
			OQueFaz: "Desliga a busca por outros License Managers na rede.",
			PorQue:  "Elimina conexoes de saida que voce nao pediu e que hoje aparecem no log do servico.",
			Reverte: "Apagar o hasplm.ini e reiniciar o servico hasplms.",
			Comandos: cmdINI("accesstoremote = 0"),
			Aplicar: func(c *Ctx) error {
				return aplicarAjuste(c, func(o *OpcoesLM) { o.NaoProcurar = true })
			},
			Reverter: func(c *Ctx) error {
				return aplicarAjuste(c, func(o *OpcoesLM) { o.NaoProcurar = false })
			},
			Ler: estadoLM(func(cfg sentinel.Config) bool { return !cfg.ProcuraRemoto }),
		},
		{
			ID: "A4", Grupo: "A", Risco: RiscoBaixo, Padrao: true,
			Titulo:  "Desligar o broadcast na rede",
			OQueFaz: "Para de disparar broadcast UDP na porta 1947 anunciando este License Manager.",
			PorQue:  "E o que gera as tentativas de conexao registradas hoje no error.log do servico. Desligando, esse ruido cessa.",
			Reverte: "Apagar o hasplm.ini e reiniciar o servico hasplms.",
			Comandos: cmdINI("broadcastsearch = 0"),
			Aplicar: func(c *Ctx) error {
				return aplicarAjuste(c, func(o *OpcoesLM) { o.SemBroadcast = true })
			},
			Reverter: func(c *Ctx) error {
				return aplicarAjuste(c, func(o *OpcoesLM) { o.SemBroadcast = false })
			},
			Ler: estadoLM(func(cfg sentinel.Config) bool { return !cfg.Broadcast }),
		},
		{
			ID: "A5", Grupo: "A", Risco: RiscoBaixo, Padrao: false,
			Titulo:  "Exigir senha no Admin Control Center",
			OQueFaz: "Passa a pedir senha para abrir o painel do Sentinel em http://127.0.0.1:1947.",
			PorQue:  "Hoje o painel abre sem autenticacao. Com ele aberto, desabilitar uma chave sao dois cliques.",
			Reverte: "Apagar o hasplm.ini e reiniciar o servico hasplms. A senha fica registrada em estado.json e no backup.",
			Comandos: func(c *Ctx) []string {
				return []string{
					"escrever " + sentinel.CaminhoINI() + "  (password = ******)",
					"sc stop hasplms", "sc start hasplms",
				}
			},
			Aplicar: func(c *Ctx) error {
				if c.SenhaACC() == "" {
					return fmt.Errorf("defina a senha no painel antes de aplicar esta acao")
				}
				return aplicarAjuste(c, func(*OpcoesLM) {})
			},
			Reverter: func(c *Ctx) error {
				if err := c.DefinirSenhaACC(""); err != nil {
					return err
				}
				return aplicarAjuste(c, func(*OpcoesLM) {})
			},
			Ler: estadoLM(func(cfg sentinel.Config) bool { return cfg.ACCComSenha }),
		},
		{
			ID: "A6", Grupo: "A", Risco: RiscoBaixo, Padrao: true,
			Titulo:  "Ligar registro de acessos",
			OQueFaz: "Ativa o log de requisicoes e de erros do License Manager.",
			PorQue:  "Deixa rastro de quem pediu licenca e quando - util para perceber acesso indevido.",
			Reverte: "Apagar o hasplm.ini e reiniciar o servico hasplms.",
			Comandos: cmdINI("requestlog = 1, errorlog = 1"),
			Aplicar: func(c *Ctx) error {
				return aplicarAjuste(c, func(o *OpcoesLM) { o.Logs = true })
			},
			Reverter: func(c *Ctx) error {
				return aplicarAjuste(c, func(o *OpcoesLM) { o.Logs = false })
			},
			Ler: func(c *Ctx) Estado {
				if c.OpcoesLM().Logs {
					return Aplicado
				}
				return NaoAplicado
			},
		},
	}
}

// ---------------------------------------------------------------- grupo B

// acaoRegras cria uma acao que instala um conjunto de regras de bloqueio.
func acaoRegras(id, titulo, oQueFaz, porQue string, risco Risco, padrao bool, montar func() []fw.Regra) Acao {
	return Acao{
		ID: id, Grupo: "B", Titulo: titulo, OQueFaz: oQueFaz, PorQue: porQue,
		Risco: risco, Padrao: padrao,
		Reverte: "Apagar as regras de firewall com o prefixo \"" + fw.Prefixo + "\".",
		Comandos: func(*Ctx) []string {
			var cs []string
			for _, r := range montar() {
				cs = append(cs, r.Comando())
			}
			if len(cs) == 0 {
				cs = append(cs, "(nenhum executavel correspondente encontrado nesta maquina)")
			}
			return cs
		},
		Aplicar: func(*Ctx) error {
			for _, r := range montar() {
				if err := fw.Criar(r); err != nil {
					return err
				}
			}
			return nil
		},
		Reverter: func(*Ctx) error {
			for _, r := range montar() {
				if err := fw.Remover(r.Nome); err != nil {
					return err
				}
			}
			return nil
		},
		Ler: func(*Ctx) Estado {
			rs := montar()
			if len(rs) == 0 {
				return Indisponivel
			}
			n := 0
			for _, r := range rs {
				if fw.Existe(r.Nome) {
					n++
				}
			}
			switch {
			case n == 0:
				return NaoAplicado
			case n == len(rs):
				return Aplicado
			default:
				return Parcial
			}
		},
	}
}

func acoesGrupoB() []Acao {
	return []Acao{
		{
			ID: "B1", Grupo: "B", Risco: RiscoBaixo, Padrao: true,
			Titulo:  "Desativar as regras que abrem a 1947 para a rede",
			OQueFaz: "Desabilita as regras de entrada ja existentes nesta maquina que liberam a porta 1947: " + strings.Join(regrasEntradaExistentes, ", ") + ".",
			PorQue:  "Sao elas que hoje permitem que outra maquina alcance o gerenciador de licencas. Ficam desabilitadas, nao apagadas, para poderem ser religadas.",
			Reverte: "Reabilitar as mesmas regras (enable=yes).",
			Comandos: func(*Ctx) []string {
				var cs []string
				for _, n := range regrasEntradaExistentes {
					cs = append(cs, fw.ComandoHabilitar(n, false))
				}
				return cs
			},
			Aplicar: func(*Ctx) error {
				for _, n := range regrasEntradaExistentes {
					if err := fw.Habilitar(n, false); err != nil {
						return err
					}
				}
				return nil
			},
			Reverter: func(*Ctx) error {
				for _, n := range regrasEntradaExistentes {
					if err := fw.Habilitar(n, true); err != nil {
						return err
					}
				}
				return nil
			},
			Ler: func(*Ctx) Estado {
				presentes, desligadas := 0, 0
				for _, n := range regrasEntradaExistentes {
					ligada, existe := fw.RegraLigada(n)
					if !existe {
						continue
					}
					presentes++
					if !ligada {
						desligadas++
					}
				}
				switch {
				case presentes == 0:
					return Indisponivel
				case desligadas == 0:
					return NaoAplicado
				case desligadas == presentes:
					return Aplicado
				default:
					return Parcial
				}
			},
		},

		acaoRegras("B2", "Bloquear entrada na porta 1947",
			"Cria regras de bloqueio de entrada para a porta 1947 em TCP e UDP, em todos os perfis de rede.",
			"Bloqueio vence liberacao no Firewall do Windows. Mesmo que alguma regra de liberacao reapareca depois, a porta continua fechada para a rede.",
			RiscoBaixo, true,
			func() []fw.Regra {
				return []fw.Regra{
					{Nome: "Entrada 1947 TCP", Direcao: "in", Protocolo: "TCP", PortaLoc: "1947",
						Descricao: "ProtegeWX: impede acesso externo ao Sentinel License Manager"},
					{Nome: "Entrada 1947 UDP", Direcao: "in", Protocolo: "UDP", PortaLoc: "1947",
						Descricao: "ProtegeWX: impede acesso externo ao Sentinel License Manager"},
				}
			}),

		acaoRegras("B3", "Impedir que o gerenciador de licencas saia da maquina",
			"Bloqueia o trafego de saida dos executaveis hasplms.exe e hasplmv.exe.",
			"Corta qualquer comunicacao do componente de licenciamento com a internet. Nao afeta os dongles: o Windows nao filtra loopback, e a API do WinDev fala com o servico em 127.0.0.1.",
			RiscoBaixo, true,
			func() []fw.Regra {
				var rs []fw.Regra
				for _, p := range binariosLM() {
					rs = append(rs, fw.Regra{
						Nome: "Saida " + nomeDoArquivo(p), Direcao: "out", Programa: p,
						Descricao: "ProtegeWX: impede que o License Manager fale com a internet",
					})
				}
				return rs
			}),

		acaoRegras("B4", "Bloquear saida para a porta 1947 de qualquer host",
			"Cria regras de bloqueio de saida para a porta remota 1947, em TCP e UDP.",
			"Impede que qualquer programa desta maquina consulte um gerenciador de licencas em outro lugar.",
			RiscoBaixo, true,
			func() []fw.Regra {
				return []fw.Regra{
					{Nome: "Saida 1947 TCP", Direcao: "out", Protocolo: "TCP", PortaRem: "1947",
						Descricao: "ProtegeWX: nenhuma consulta a License Manager remoto"},
					{Nome: "Saida 1947 UDP", Direcao: "out", Protocolo: "UDP", PortaRem: "1947",
						Descricao: "ProtegeWX: nenhuma consulta a License Manager remoto"},
				}
			}),

		acaoRegras("B5", "Bloquear os atualizadores da PC SOFT",
			"Bloqueia o trafego de saida do AutomaticUpdate.exe e do haspdinst.exe de todos os produtos PC SOFT encontrados nesta maquina.",
			"Sao os programas que falam com a PC SOFT e que poderiam trazer uma atualizacao de licenca. As IDEs em si continuam com rede liberada, entao deploy, HFSQL e geracao mobile seguem funcionando.",
			RiscoMedio, true,
			func() []fw.Regra {
				var rs []fw.Regra
				for _, p := range binariosAtualizacao() {
					rs = append(rs, fw.Regra{
						Nome: "Saida " + rotuloProduto(p), Direcao: "out", Programa: p,
						Descricao: "ProtegeWX: bloqueia atualizador PC SOFT",
					})
				}
				return rs
			}),
	}
}

func nomeDoArquivo(p string) string { return filepath.Base(p) }

// rotuloProduto transforma o caminho num nome curto e unico para a regra,
// por exemplo "AutomaticUpdate.exe (WINDEV 27)". O nome do produto vem do
// proprio caminho, entao funciona com qualquer versao instalada.
func rotuloProduto(p string) string {
	nome := filepath.Base(p)
	for _, prod := range produtosPCSoft() {
		if strings.HasPrefix(p, prod+string(filepath.Separator)) {
			return nome + " (" + filepath.Base(prod) + ")"
		}
	}
	return nome
}

// ---------------------------------------------------------------- grupo C

// Listas de dominios.
//
// Todos os nomes abaixo foram confirmados por consulta DNS: cada um resolve de
// verdade. Nomes plausiveis que nao existem (update.pcsoft.fr, stats.pcsoft.fr,
// licensing.thalesgroup.com e outros) foram retirados de proposito - bloquear
// dominio inexistente nao protege nada e so faz a lista parecer maior do que e.
var (
	dominiosTelemetria = []string{
		"telemetrie.pcsoft.fr", // servidor de telemetria da PC SOFT
		"api.pcsoft.fr",
		"ftp.pcsoft.fr",
		"sentinelcloud.com", // Thales / Sentinel
		"sentinelup.com",
		"safenet-inc.com",
	}
	dominiosSite = []string{
		"pcsoft.fr", "www.pcsoft.fr",
		"doc.pcsoft.fr", "forum.pcsoft.fr", "support.pcsoft.fr",
		"windev.com", "pcsoft.com.br", "pcsoft-windev-webdev.com",
	}
)

func gruposHosts(incluirSite bool) []hostsfile.Grupo {
	gs := []hostsfile.Grupo{{Nome: "Telemetria e atualizacao (PC SOFT / Thales)", Dominios: dominiosTelemetria}}
	if incluirSite {
		gs = append(gs, hostsfile.Grupo{Nome: "Site, documentacao e forum PC SOFT", Dominios: dominiosSite})
	}
	return gs
}

// siteBloqueado detecta se o bloco atual do hosts inclui tambem os dominios do site.
func siteBloqueado() bool {
	for _, d := range hostsfile.DominiosAtivos() {
		if d == "doc.pcsoft.fr" {
			return true
		}
	}
	return false
}

func acoesGrupoC() []Acao {
	return []Acao{
		{
			ID: "C1", Grupo: "C", Risco: RiscoBaixo, Padrao: true,
			Titulo:  "Bloquear dominios de telemetria e atualizacao",
			OQueFaz: "Aponta para 0.0.0.0 os enderecos de atualizacao e telemetria da PC SOFT e da Thales/Sentinel.",
			PorQue:  "Fecha o caminho por nome para os servidores que poderiam entregar uma atualizacao de licenca.",
			Reverte: "Remover o bloco marcado com >>> PROTEGEWX >>> do arquivo hosts.",
			Comandos: func(*Ctx) []string {
				return []string{
					"escrever bloco delimitado em " + hostsfile.Caminho(),
					"dominios: " + strings.Join(dominiosTelemetria, ", "),
					"ipconfig /flushdns",
				}
			},
			Aplicar:  func(*Ctx) error { return hostsfile.Aplicar(gruposHosts(siteBloqueado())) },
			Reverter: func(*Ctx) error { return hostsfile.Remover() },
			Ler: func(*Ctx) Estado {
				if hostsfile.Aplicado() {
					return Aplicado
				}
				return NaoAplicado
			},
		},
		{
			ID: "C2", Grupo: "C", Risco: RiscoMedio, Padrao: false,
			Titulo:  "Bloquear tambem site, documentacao e forum",
			OQueFaz: "Acrescenta ao bloqueio os dominios do site da PC SOFT, incluindo doc.pcsoft.fr e o forum.",
			PorQue:  "Isolamento maximo por nome. Deixado desmarcado porque derruba a documentacao online do WinDev, que costuma ser usada no dia a dia.",
			Reverte: "Reaplicar o grupo C apenas com os dominios de telemetria.",
			Comandos: func(*Ctx) []string {
				return []string{
					"acrescentar ao bloco em " + hostsfile.Caminho(),
					"dominios: " + strings.Join(dominiosSite, ", "),
					"ipconfig /flushdns",
				}
			},
			Aplicar:  func(*Ctx) error { return hostsfile.Aplicar(gruposHosts(true)) },
			Reverter: func(*Ctx) error { return hostsfile.Aplicar(gruposHosts(false)) },
			Ler: func(*Ctx) Estado {
				if siteBloqueado() {
					return Aplicado
				}
				return NaoAplicado
			},
		},
	}
}

// ---------------------------------------------------------------- grupo D

// sidTodos e o SID universal do grupo "Todos"/"Everyone". Usar o SID evita
// depender do idioma do Windows.
const sidTodos = `*S-1-1-0`

func acoesGrupoD() []Acao {
	return []Acao{
		{
			ID: "D1", Grupo: "D", Risco: RiscoBaixo, Padrao: true,
			Titulo:  "Registrar o estado atual das licencas",
			OQueFaz: "Grava um retrato das chaves e das licencas em baseline/chaves-baseline.json.",
			PorQue:  "E a referencia contra a qual o monitor compara todo dia. Sem ele nao ha como provar o que a chave continha antes.",
			Reverte: "Apagar o arquivo de baseline (nao ha efeito no sistema).",
			Comandos: func(c *Ctx) []string {
				return []string{"ler as chaves em " + sentinel.BaseACC + " e gravar " + c.Base + `\baseline\chaves-baseline.json`}
			},
			Aplicar:  func(c *Ctx) error { return salvarBaseline(c) },
			Reverter: func(c *Ctx) error { return os.Remove(caminhoBaseline(c)) },
			Ler: func(c *Ctx) Estado {
				if _, err := os.Stat(caminhoBaseline(c)); err == nil {
					return Aplicado
				}
				return NaoAplicado
			},
		},
		{
			ID: "D2", Grupo: "D", Risco: RiscoBaixo, Padrao: true,
			Titulo:  "Preservar uma copia do runtime Sentinel",
			OQueFaz: "Copia a pasta do License Manager para backup, junto da versao instalada.",
			PorQue:  "Se uma atualizacao futura trocar o runtime por um que se recuse a funcionar, esta copia permite voltar a versao que funciona hoje.",
			Reverte: "Apagar a pasta de copia (nao ha efeito no sistema).",
			Comandos: func(c *Ctx) []string {
				return []string{`copiar "` + sentinel.PastaLM() + `" para "` + c.Base + `\backup\runtime-atual"`}
			},
			Aplicar: func(c *Ctx) error {
				destino := c.Base + `\backup\runtime-atual`
				if err := os.MkdirAll(destino, 0o755); err != nil {
					return err
				}
				_, err := sysexec.RodarErr("robocopy", sentinel.PastaLM(), destino, "/E", "/NFL", "/NDL", "/NJH", "/NJS", "/NP")
				// robocopy usa codigos de saida < 8 para sucesso
				if err != nil && robocopyOk(sentinel.PastaLM(), destino) {
					return nil
				}
				return err
			},
			Reverter: func(c *Ctx) error { return os.RemoveAll(c.Base + `\backup\runtime-atual`) },
			Ler: func(c *Ctx) Estado {
				if _, err := os.Stat(c.Base + `\backup\runtime-atual\hasplms.exe`); err == nil {
					return Aplicado
				}
				return NaoAplicado
			},
		},
		{
			ID: "D3", Grupo: "D", Risco: RiscoMedio, Padrao: true,
			Titulo:  "Negar execucao dos atualizadores",
			OQueFaz: "Aplica uma negacao de execucao (ACL) sobre o AutomaticUpdate.exe e o haspdinst.exe de todos os produtos PC SOFT encontrados.",
			PorQue:  "Aplicar um arquivo V2C e o unico caminho real de desativacao das suas chaves, e depende de rodar um destes programas. Bloquear a execucao fecha esse caminho de forma direta.",
			Reverte: "Remover a negacao com icacls /remove:d. Faca isso antes de reinstalar o driver do dongle.",
			Comandos: func(*Ctx) []string {
				var cs []string
				for _, p := range binariosAtualizacao() {
					cs = append(cs, sysexec.Formatar("icacls", p, "/deny", sidTodos+":(X)"))
				}
				if len(cs) == 0 {
					cs = append(cs, "(nenhum atualizador encontrado nesta maquina)")
				}
				return cs
			},
			Aplicar: func(*Ctx) error {
				for _, p := range binariosAtualizacao() {
					if _, err := sysexec.RodarErr("icacls", p, "/deny", sidTodos+":(X)"); err != nil {
						return err
					}
				}
				return nil
			},
			Reverter: func(*Ctx) error {
				for _, p := range binariosAtualizacao() {
					if _, err := sysexec.RodarErr("icacls", p, "/remove:d", sidTodos); err != nil {
						return err
					}
				}
				return nil
			},
			Ler: func(*Ctx) Estado {
				bins := binariosAtualizacao()
				if len(bins) == 0 {
					return Indisponivel
				}
				n := 0
				for _, p := range bins {
					r := sysexec.Rodar("icacls", p)
					if strings.Contains(r.Saida, "(DENY)") || strings.Contains(r.Saida, "(N)") {
						n++
					}
				}
				switch {
				case n == 0:
					return NaoAplicado
				case n == len(bins):
					return Aplicado
				default:
					return Parcial
				}
			},
		},
		{
			ID: "D4", Grupo: "D", Risco: RiscoBaixo, Padrao: false,
			Titulo:  "Monitorar as licencas todo dia",
			OQueFaz: "Cria uma tarefa agendada diaria que compara o estado das chaves com o baseline e registra alerta se algo mudar.",
			PorQue:  "Se algum dia uma licenca deixar de ser perpetua ou uma chave for desabilitada, voce fica sabendo no mesmo dia, e nao meses depois. Nesta maquina o servico Agendador de Tarefas do Windows esta desabilitado, entao esta acao so fica disponivel se voce reabilita-lo. Enquanto isso, use o botao \"Conferir agora\" no painel, ou o comando protegewx.exe --check.",
			Reverte: "Apagar a tarefa agendada \"" + tarefaNome + "\".",
			Comandos: func(c *Ctx) []string {
				return []string{scriptCriarTarefa(c.Exe)}
			},
			Aplicar: func(c *Ctx) error {
				_, err := sysexec.RodarErr("powershell", "-NoProfile", "-NonInteractive",
					"-ExecutionPolicy", "Bypass", "-Command", scriptCriarTarefa(c.Exe))
				return err
			},
			Reverter: func(*Ctx) error {
				_, err := sysexec.RodarErr("schtasks", "/Delete", "/TN", tarefaNome, "/F")
				return err
			},
			Ler: func(*Ctx) Estado {
				if sysexec.Rodar("schtasks", "/Query", "/TN", tarefaNome).Ok() {
					return Aplicado
				}
				if !agendadorAtivo() {
					// sem o servico Schedule nao ha como criar a tarefa; melhor
					// dizer isso do que oferecer um botao que falharia
					return Indisponivel
				}
				return NaoAplicado
			},
		},
		{
			ID: "D5", Grupo: "D", Risco: RiscoBaixo, Padrao: true,
			Titulo:  "Monitorar toda vez que o computador ligar",
			OQueFaz: "Registra a conferencia das licencas para rodar a cada logon, de forma invisivel. Se alguma licenca tiver mudado, abre uma janela avisando.",
			PorQue:  "Faz o mesmo papel do monitor diario, mas sem depender do servico Agendador de Tarefas do Windows - que nesta maquina esta desabilitado. E a forma menos invasiva de ter vigilancia automatica.",
			Reverte: "Remover o valor ProtegeWX da chave Run do registro.",
			Comandos: func(c *Ctx) []string {
				return []string{sysexec.Formatar("reg", "add", chaveRun,
					"/v", "ProtegeWX", "/t", "REG_SZ", "/d", comandoMonitorLogon(c.Exe), "/f")}
			},
			Aplicar: func(c *Ctx) error {
				_, err := sysexec.RodarErr("reg", "add", chaveRun,
					"/v", "ProtegeWX", "/t", "REG_SZ", "/d", comandoMonitorLogon(c.Exe), "/f")
				return err
			},
			Reverter: func(*Ctx) error {
				r := sysexec.Rodar("reg", "delete", chaveRun, "/v", "ProtegeWX", "/f")
				if r.Ok() || strings.Contains(strings.ToLower(r.Saida), "nao foi possivel localizar") ||
					strings.Contains(strings.ToLower(r.Saida), "unable to find") {
					return nil
				}
				return fmt.Errorf("%s: %s", r.Linha, r.Saida)
			},
			Ler: func(*Ctx) Estado {
				if sysexec.Rodar("reg", "query", chaveRun, "/v", "ProtegeWX").Ok() {
					return Aplicado
				}
				return NaoAplicado
			},
		},
	}
}

// agendadorAtivo informa se o servico Agendador de Tarefas do Windows pode ser
// usado. Nesta maquina ele esta desabilitado, e sem ele nenhuma tarefa agendada
// pode ser criada - inclusive por schtasks, que reporta um erro enganoso.
func agendadorAtivo() bool {
	r := sysexec.PowerShell(
		`$s = Get-Service Schedule -ErrorAction SilentlyContinue; ` +
			`if ($s -and $s.StartType -ne 'Disabled') { 'sim' } else { 'nao' }`)
	return strings.TrimSpace(r.Saida) == "sim"
}

// scriptCriarTarefa monta a criacao da tarefa diaria via cmdlets do agendador.
//
// Register-ScheduledTask recebe executavel e argumentos em campos separados, o
// que evita o aninhamento de aspas que o schtasks /TR nao consegue interpretar.
func scriptCriarTarefa(exe string) string {
	e := strings.ReplaceAll(exe, "'", "''")
	return "$a = New-ScheduledTaskAction -Execute '" + e + "' -Argument '--check'; " +
		"$g = New-ScheduledTaskTrigger -Daily -At 9am; " +
		"$p = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest; " +
		"$s = New-ScheduledTaskSettingsSet -StartWhenAvailable -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries; " +
		"Register-ScheduledTask -TaskName '" + tarefaNome + "' " +
		"-Action $a -Trigger $g -Principal $p -Settings $s " +
		"-Description 'ProtegeWX: compara diariamente o conteudo das licencas com o baseline' -Force | Out-Null"
}

// chaveRun e o caminho da chave Run da maquina, usada pelo monitor de logon.
const chaveRun = `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Run`

// comandoMonitorLogon monta a linha registrada na chave Run.
//
// O executavel e de console: chamado direto, piscaria uma janela preta a cada
// logon. O PowerShell com -WindowStyle Hidden roda a conferencia sem nada
// aparecer, e o proprio ProtegeWX abre uma janela de aviso se, e somente se,
// encontrar alteracao grave nas licencas.
func comandoMonitorLogon(exe string) string {
	return `powershell -NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -Command ` +
		`"& '` + exe + `' --check --silencioso"`
}

func robocopyOk(origem, destino string) bool {
	_, err := os.Stat(destino + `\hasplms.exe`)
	return err == nil
}

// ---------------------------------------------------------------- catalogo

// Todas devolve o catalogo completo, na ordem de aplicacao recomendada.
func Todas() []Acao {
	var v []Acao
	d := acoesGrupoD()
	v = append(v, d[:2]...) // D1 e D2 primeiro: backup antes de mexer em qualquer coisa
	v = append(v, acoesGrupoA()...)
	v = append(v, acoesGrupoB()...)
	v = append(v, acoesGrupoC()...)
	v = append(v, d[2:]...)
	return v
}

// Por devolve a acao de um dado ID.
func Por(id string) (Acao, bool) {
	for _, a := range Todas() {
		if a.ID == id {
			return a, true
		}
	}
	return Acao{}, false
}
