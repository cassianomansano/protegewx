// Package sentinel conversa com o Sentinel Admin Control Center em 127.0.0.1:1947
// e com o arquivo de configuracao hasplm.ini do License Manager.
//
// Duas responsabilidades:
//   - ler o estado real das chaves e da configuracao (fonte de verdade do painel)
//   - aplicar a configuracao de isolamento e VERIFICAR se ela realmente pegou
//
// A verificacao e o ponto importante. Os nomes das chaves do hasplm.ini variam
// entre versoes do runtime, entao nao confiamos em ter escrito o nome certo:
// aplicamos, reiniciamos o servico e medimos o efeito observavel. Se o efeito
// nao aconteceu, revertemos sozinhos e dizemos que falhou.
package sentinel

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"protegewx/internal/marca"
	"protegewx/internal/sysexec"
)

const (
	BaseACC = "http://127.0.0.1:1947"
	Servico = "hasplms"
)

// PastaLM localiza a instalacao do Sentinel License Manager.
//
// O caminho varia conforme a arquitetura do Windows e a versao do runtime, por
// isso e procurado em vez de fixado: assim o programa funciona em qualquer
// maquina sem precisar de ajuste.
func PastaLM() string {
	candidatos := []string{
		os.Getenv("CommonProgramFiles(x86)") + `\Aladdin Shared\HASP`,
		os.Getenv("CommonProgramFiles") + `\Aladdin Shared\HASP`,
		`C:\Program Files (x86)\Common Files\Aladdin Shared\HASP`,
		`C:\Program Files\Common Files\Aladdin Shared\HASP`,
	}
	for _, c := range candidatos {
		if strings.HasPrefix(c, `\`) {
			continue // variavel de ambiente vazia
		}
		if _, err := os.Stat(filepath.Join(c, "hasplms.exe")); err == nil {
			return c
		}
	}
	// nao encontrado: devolve o caminho mais comum, para as mensagens de erro
	// apontarem para um lugar reconhecivel
	return `C:\Program Files (x86)\Common Files\Aladdin Shared\HASP`
}

// CaminhoINI e o hasplm.ini que o License Manager le na inicializacao.
func CaminhoINI() string { return filepath.Join(PastaLM(), "hasplm.ini") }

var cliente = &http.Client{Timeout: 12 * time.Second}

// ---------------------------------------------------------------- leitura

// Chave e um dongle visto pelo License Manager.
type Chave struct {
	HaspID      string `json:"haspid"`
	Tipo        string `json:"typ"`
	Vendor      string `json:"vid"`
	Firmware    string `json:"fw"`
	HWVersion   string `json:"hw_version"`
	Locked      string `json:"locked"`
	KeyDisabled string `json:"key_disabled"`
	CloudBased  string `json:"cloud_based"`
	RehostType  string `json:"rehost_type"`
	Cloned      string `json:"cloned"`
	Sessoes     string `json:"sesc"`
	Local       string `json:"loc"`
	C2V         string `json:"c2v"`
}

// Feature e uma licenca gravada dentro de uma chave.
type Feature struct {
	HaspID       string `json:"haspid"`
	FeatureID    string `json:"fid"`
	Produto      string `json:"prname"`
	ProdutoID    string `json:"prid"`
	Licenca      string `json:"lic"`
	Expirada     string `json:"ex"`
	Desabilitada string `json:"dis"`
	Inutilizavel string `json:"unusable"`
	Locked       string `json:"locked"`
	RelogioMex   string `json:"time_tampered"`
	Sessoes      string `json:"sesc"`
}

func obter(caminho string) (string, error) {
	resp, err := cliente.Get(BaseACC + caminho)
	if err != nil {
		return "", fmt.Errorf("ACC inacessivel em %s: %w", BaseACC, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ACC devolveu HTTP %d para %s", resp.StatusCode, caminho)
	}
	return string(b), nil
}

// o ACC devolve objetos JSON concatenados, precedidos de um marcador em comentario
var reObjeto = regexp.MustCompile(`\{[^{}]*\}`)

func extrairObjetos(corpo string, alvo any) error {
	// descarta comentarios /* ... */ (marcador e admin_status no fim)
	limpo := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(corpo, "")
	itens := reObjeto.FindAllString(limpo, -1)
	if len(itens) == 0 {
		return fmt.Errorf("nenhum registro retornado pelo ACC")
	}
	return json.Unmarshal([]byte("["+strings.Join(itens, ",")+"]"), alvo)
}

// Chaves devolve os dongles reais, descartando a chave demo padrao (DEMOMA).
func Chaves() ([]Chave, error) {
	corpo, err := obter("/_int_/tab_dev.html")
	if err != nil {
		return nil, err
	}
	var todas []Chave
	if err := extrairObjetos(corpo, &todas); err != nil {
		return nil, err
	}
	var reais []Chave
	for _, c := range todas {
		if c.HaspID == "" || c.Tipo == "" || c.Tipo == "placeholder" {
			continue
		}
		reais = append(reais, c)
	}
	return reais, nil
}

// Features devolve as licencas gravadas nas chaves.
func Features() ([]Feature, error) {
	corpo, err := obter("/_int_/tab_feat.html")
	if err != nil {
		return nil, err
	}
	var todas []Feature
	if err := extrairObjetos(corpo, &todas); err != nil {
		return nil, err
	}
	var reais []Feature
	for _, f := range todas {
		if f.HaspID != "" {
			reais = append(reais, f)
		}
	}
	return reais, nil
}

// ---------------------------------------------------------------- estado da config

// Config e o estado de rede do License Manager, lido do proprio ACC.
type Config struct {
	AceitaRemoto    bool `json:"aceitaRemoto"`    // aceita clientes de outras maquinas
	ProcuraRemoto   bool `json:"procuraRemoto"`   // sai buscando License Managers na rede
	Broadcast       bool `json:"broadcast"`       // dispara broadcast UDP 1947
	SomenteLoopback bool `json:"somenteLoopback"` // escuta apenas em 127.0.0.1
	ACCComSenha     bool `json:"accComSenha"`     // painel exige senha
	INIExiste       bool `json:"iniExiste"`
}

var (
	// no HTML do ACC, um checkbox ligado traz o atributo checked logo apos o id
	reChecked = func(id string) *regexp.Regexp {
		return regexp.MustCompile(`(?s)id="` + id + `"[^>]{0,200}?checked`)
	}
	rePassACC = regexp.MustCompile(`passACC\s*=\s*(\d)`)
)

// LerConfig monta o estado atual perguntando ao proprio ACC.
func LerConfig() (Config, error) {
	var c Config

	de, err := obter("/_int_/conf_from.html")
	if err != nil {
		return c, err
	}
	para, err := obter("/_int_/conf_to.html")
	if err != nil {
		return c, err
	}

	c.SomenteLoopback = reChecked("bindlocalonly").MatchString(de)
	c.AceitaRemoto = reChecked("accessfromremote_remote").MatchString(de) ||
		reChecked("accessfromremote_split").MatchString(de)
	c.ProcuraRemoto = reChecked("accesstoremote").MatchString(para)
	c.Broadcast = reChecked("broadcastsearch").MatchString(para)

	if raiz, err := obter("/_int_/devices.html"); err == nil {
		if m := rePassACC.FindStringSubmatch(raiz); len(m) == 2 {
			c.ACCComSenha = m[1] != "0"
		}
	}

	_, errIni := os.Stat(CaminhoINI())
	c.INIExiste = errIni == nil

	return c, nil
}

// EscutaExposta informa se a porta 1947 esta escutando em alguma interface que
// nao seja o loopback. E a verificacao objetiva de que o isolamento funcionou:
// nao depende de interpretar configuracao, olha o socket de verdade.
func EscutaExposta() (bool, []string, error) {
	res := sysexec.PowerShell(
		`Get-NetTCPConnection -LocalPort 1947 -State Listen -ErrorAction SilentlyContinue | ` +
			`Select-Object -ExpandProperty LocalAddress`)
	if !res.Ok() {
		return false, nil, fmt.Errorf("nao foi possivel inspecionar a porta 1947: %s", res.Saida)
	}
	var expostos []string
	for _, linha := range strings.Split(res.Saida, "\n") {
		addr := strings.TrimSpace(linha)
		if addr == "" {
			continue
		}
		if ip := net.ParseIP(strings.Trim(addr, "[]")); ip != nil && ip.IsLoopback() {
			continue
		}
		expostos = append(expostos, addr)
	}
	return len(expostos) > 0, expostos, nil
}

// escutaExpostaEstavel observa a porta 1947 durante um intervalo e considera a
// escuta exposta se ela aparecer em qualquer momento da janela. Assim uma
// medicao feita cedo demais nao produz um falso "ja esta isolado".
func escutaExpostaEstavel(janela time.Duration) (bool, []string, error) {
	fim := time.Now().Add(janela)
	vistos := map[string]bool{}
	var ultimoErro error

	for {
		exposta, addrs, err := EscutaExposta()
		if err != nil {
			ultimoErro = err
		} else if exposta {
			for _, a := range addrs {
				vistos[a] = true
			}
		}
		if time.Now().After(fim) {
			break
		}
		time.Sleep(700 * time.Millisecond)
	}

	if len(vistos) == 0 {
		return false, nil, ultimoErro
	}
	lista := make([]string, 0, len(vistos))
	for a := range vistos {
		lista = append(lista, a)
	}
	sort.Strings(lista)
	return true, lista, nil
}

// ---------------------------------------------------------------- escrita

// OpcoesIsolamento descreve o que queremos que o License Manager passe a fazer.
type OpcoesIsolamento struct {
	SomenteLoopback bool
	RecusarRemoto   bool
	NaoProcurar     bool
	SemBroadcast    bool
	Logs            bool
	SenhaACC        string // vazio = nao mexe na senha
}

// conteudoINI monta o hasplm.ini.
//
// Os nomes usados aqui sao os mesmos identificadores que o ACC expoe no seu
// formulario de configuracao, que e a pista mais confiavel de como esta versao
// do runtime nomeia cada opcao. Ainda assim o resultado e sempre conferido por
// Verificar() - nada aqui e assumido como certo.
func conteudoINI(o OpcoesIsolamento) string {
	b := &strings.Builder{}
	b.WriteString("; ============================================================\n")
	b.WriteString(";  hasplm.ini  -  gerado pelo " + marca.Nome + "\n")
	b.WriteString(";  " + time.Now().Format("02/01/2006 15:04:05") + "\n")
	b.WriteString(";\n")
	b.WriteString(";  Mantem o Sentinel License Manager restrito a esta maquina.\n")
	b.WriteString(";  Nao altera nenhuma licenca: apenas configuracao de rede.\n")
	b.WriteString(";  Para reverter, apague este arquivo e reinicie o servico hasplms.\n")
	b.WriteString("; ============================================================\n\n")

	b.WriteString("[SERVER]\n")
	if o.SomenteLoopback {
		b.WriteString("bindlocalonly = 1\n")
		b.WriteString("bind_local_only = 1\n") // grafia alternativa entre versoes
	}
	if o.RecusarRemoto {
		b.WriteString("accremote = 0\n")
		b.WriteString("accessfromremote = 0\n")
	}
	if o.Logs {
		b.WriteString("requestlog = 1\n")
		b.WriteString("errorlog = 1\n")
	}
	if o.SenhaACC != "" {
		b.WriteString("password = " + o.SenhaACC + "\n")
		b.WriteString("adminpassword = " + o.SenhaACC + "\n")
	}

	b.WriteString("\n[REMOTE]\n")
	if o.NaoProcurar {
		b.WriteString("accessremote = 0\n")
		b.WriteString("accesstoremote = 0\n")
	}
	if o.SemBroadcast {
		b.WriteString("broadcastsearch = 0\n")
	}
	b.WriteString("serveraddr =\n")
	b.WriteString("serverlist =\n")

	b.WriteString("\n[ACCESS]\n")
	if o.RecusarRemoto {
		// so o proprio host pode consumir licencas
		b.WriteString("allow = localhost\n")
		b.WriteString("deny = ALL\n")
	}
	return b.String()
}

// Aplicar grava o hasplm.ini e reinicia o License Manager.
func Aplicar(o OpcoesIsolamento) error {
	if err := os.WriteFile(CaminhoINI(), []byte(conteudoINI(o)), 0o644); err != nil {
		return fmt.Errorf("nao foi possivel gravar %s: %w", CaminhoINI(), err)
	}
	return ReiniciarServico()
}

// Remover apaga o hasplm.ini, devolvendo o License Manager ao padrao de fabrica.
func Remover() error {
	if err := os.Remove(CaminhoINI()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return ReiniciarServico()
}

// ReiniciarServico reinicia o hasplms e espera o ACC voltar a responder.
func ReiniciarServico() error {
	if _, err := sysexec.RodarErr("sc", "stop", Servico); err != nil {
		// parar um servico ja parado nao e falha
		if !strings.Contains(strings.ToLower(err.Error()), "1062") {
			return err
		}
	}
	aguardarServico("STOPPED", 20*time.Second)

	if _, err := sysexec.RodarErr("sc", "start", Servico); err != nil {
		return err
	}
	if !aguardarACC(30 * time.Second) {
		return fmt.Errorf("o servico %s foi iniciado mas o ACC nao respondeu em 30s", Servico)
	}
	return nil
}

func aguardarServico(estado string, limite time.Duration) {
	fim := time.Now().Add(limite)
	for time.Now().Before(fim) {
		r := sysexec.Rodar("sc", "query", Servico)
		if strings.Contains(r.Saida, estado) {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func aguardarACC(limite time.Duration) bool {
	fim := time.Now().Add(limite)
	for time.Now().Before(fim) {
		if _, err := obter("/_int_/devices.html"); err == nil {
			return true
		}
		time.Sleep(700 * time.Millisecond)
	}
	return false
}

// ---------------------------------------------------------------- verificacao

// Divergencia e uma opcao pedida que o License Manager nao passou a respeitar.
type Divergencia struct {
	Opcao     string `json:"opcao"`
	Esperado  string `json:"esperado"`
	Observado string `json:"observado"`
}

// Verificar confere, contra o estado real da maquina, se o isolamento pedido
// de fato entrou em vigor. Devolve a lista do que nao pegou.
func Verificar(o OpcoesIsolamento) ([]Divergencia, error) {
	cfg, err := LerConfig()
	if err != nil {
		return nil, err
	}
	var d []Divergencia

	if o.SomenteLoopback {
		// O servico acabou de reiniciar e faz o bind das interfaces aos poucos.
		// Medir imediatamente pode flagrar so o loopback e concluir, errado, que
		// o isolamento funcionou. Damos tempo ao servico antes de julgar.
		exposta, addrs, err := escutaExpostaEstavel(4 * time.Second)
		if err != nil {
			return nil, err
		}
		if exposta {
			d = append(d, Divergencia{
				Opcao:     "escutar somente em 127.0.0.1",
				Esperado:  "porta 1947 apenas no loopback",
				Observado: "ainda escutando em " + strings.Join(addrs, ", "),
			})
		}
	}
	if o.RecusarRemoto && cfg.AceitaRemoto {
		d = append(d, Divergencia{"recusar clientes remotos", "desligado", "ACC ainda aceita acesso remoto"})
	}
	if o.NaoProcurar && cfg.ProcuraRemoto {
		d = append(d, Divergencia{"nao procurar License Managers remotos", "desligado", "ACC ainda com acesso a licencas remotas"})
	}
	if o.SemBroadcast && cfg.Broadcast {
		d = append(d, Divergencia{"desligar broadcast", "desligado", "ACC ainda com broadcast search ligado"})
	}
	if o.SenhaACC != "" && !cfg.ACCComSenha {
		d = append(d, Divergencia{"senha no ACC", "exigindo senha", "painel ainda abre sem senha"})
	}
	return d, nil
}
