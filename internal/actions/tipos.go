// Package actions e o catalogo declarativo de tudo que o ProtegeWX sabe fazer.
//
// Cada acao carrega, alem do codigo que a executa, a explicacao em portugues do
// que ela faz, por que existe, qual o risco e como desfaze-la. O painel monta a
// interface a partir destes campos - a documentacao e o proprio programa, nao um
// texto separado que envelhece.
package actions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Risco indica o impacto pratico de aplicar uma acao.
type Risco string

const (
	RiscoBaixo Risco = "baixo"
	RiscoMedio Risco = "medio"
	RiscoAlto  Risco = "alto"
)

// Estado e a situacao atual de uma acao na maquina.
type Estado string

const (
	Aplicado     Estado = "aplicado"
	NaoAplicado  Estado = "nao-aplicado"
	Parcial      Estado = "parcial"
	Indisponivel Estado = "indisponivel"
)

// Acao e uma alteracao reversivel no sistema.
type Acao struct {
	ID      string `json:"id"`
	Grupo   string `json:"grupo"`
	Titulo  string `json:"titulo"`
	OQueFaz string `json:"oQueFaz"`
	PorQue  string `json:"porQue"`
	Reverte string `json:"reverte"`
	Risco   Risco  `json:"risco"`
	Padrao  bool   `json:"padrao"`

	// Comandos e o que sera executado, em texto, exibido ANTES de rodar.
	Comandos func(*Ctx) []string `json:"-"`
	Aplicar  func(*Ctx) error    `json:"-"`
	Reverter func(*Ctx) error    `json:"-"`
	Ler      func(*Ctx) Estado   `json:"-"`
}

// Grupo reune acoes relacionadas.
type Grupo struct {
	ID        string `json:"id"`
	Nome      string `json:"nome"`
	Resumo    string `json:"resumo"`
	Ressalva  string `json:"ressalva,omitempty"`
	Principal bool   `json:"principal"`
}

// Ctx carrega o que as acoes precisam para rodar.
type Ctx struct {
	Base  string // pasta de instalacao do ProtegeWX
	Exe   string // caminho do proprio executavel
	mu    sync.Mutex
	dados dadosPersistidos
}

type dadosPersistidos struct {
	SenhaACC string `json:"senhaACC,omitempty"`

	// LM guarda as opcoes de isolamento JA PEDIDAS pelo usuario.
	//
	// Elas precisam ser persistidas, e nao deduzidas do ACC a cada operacao:
	// se uma opcao nao for refletida de volta pelo painel do Sentinel, deduzi-la
	// faria a acao seguinte reescrever o arquivo sem ela, desfazendo em silencio
	// o que a anterior tinha pedido.
	LM OpcoesLM `json:"licenseManager"`

	// BindLocalIndisponivel registra que este runtime ignorou o pedido de
	// escutar apenas em loopback, para nao insistir numa opcao inexistente.
	BindLocalIndisponivel bool `json:"bindLocalIndisponivel,omitempty"`
}

// OpcoesLM sao as opcoes de isolamento do License Manager pedidas pelo usuario.
type OpcoesLM struct {
	SomenteLoopback bool `json:"somenteLoopback"`
	RecusarRemoto   bool `json:"recusarRemoto"`
	NaoProcurar     bool `json:"naoProcurar"`
	SemBroadcast    bool `json:"semBroadcast"`
	Logs            bool `json:"logs"`
}

func NovoCtx(base, exe string) *Ctx {
	c := &Ctx{Base: base, Exe: exe}
	c.carregar()
	return c
}

func (c *Ctx) arquivoEstado() string { return filepath.Join(c.Base, "estado.json") }

func (c *Ctx) carregar() {
	b, err := os.ReadFile(c.arquivoEstado())
	if err != nil {
		return
	}
	_ = json.Unmarshal(b, &c.dados)
}

func (c *Ctx) salvar() error {
	b, err := json.MarshalIndent(c.dados, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.arquivoEstado(), b, 0o600)
}

// SenhaACC devolve a senha configurada para o Admin Control Center.
func (c *Ctx) SenhaACC() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dados.SenhaACC
}

// DefinirSenhaACC guarda a senha do ACC. Fica em estado.json com permissao
// restrita - e preciso guarda-la para poder reaplicar a configuracao e para o
// usuario nao se trancar fora do proprio painel.
func (c *Ctx) DefinirSenhaACC(s string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dados.SenhaACC = s
	return c.salvar()
}

// OpcoesLM devolve as opcoes de isolamento pedidas ate agora.
func (c *Ctx) OpcoesLM() OpcoesLM {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dados.LM
}

// AjustarLM altera as opcoes de isolamento e as persiste.
func (c *Ctx) AjustarLM(f func(*OpcoesLM)) (OpcoesLM, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	f(&c.dados.LM)
	return c.dados.LM, c.salvar()
}

// BindLocalIndisponivel informa se ja constatamos que este runtime ignora a
// opcao de escutar somente em loopback.
func (c *Ctx) BindLocalIndisponivel() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dados.BindLocalIndisponivel
}

// MarcarBindLocalIndisponivel registra a constatacao acima.
func (c *Ctx) MarcarBindLocalIndisponivel() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dados.BindLocalIndisponivel = true
	c.dados.LM.SomenteLoopback = false
	return c.salvar()
}
