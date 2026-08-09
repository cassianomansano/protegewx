// ProtegeWX - isolamento de rede dos dongles Sentinel HL e dos programas PC SOFT.
// ProtegeWX.
//
// Sobe um painel local em 127.0.0.1 que mostra, antes de executar, exatamente o
// que sera feito na maquina, com o risco e a forma de desfazer. Nada e aplicado
// sem confirmacao explicita.
//
// Modos de linha de comando:
//
//	protegewx.exe                painel no navegador
//	protegewx.exe --check        compara as licencas com o baseline (usado pela tarefa diaria)
//	protegewx.exe --status       imprime o estado de todas as acoes
//	protegewx.exe --revert-all   desfaz todas as alteracoes
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	protegewx "protegewx"
	"protegewx/internal/actions"
	"protegewx/internal/fw"
	"protegewx/internal/marca"
	"protegewx/internal/sentinel"
	"protegewx/internal/sysexec"
)

var (
	ctx        *actions.Ctx
	arqLog     *os.File
	baseDir    string
	verbose    bool
	silencioso bool
)

func main() {
	var (
		check     = flag.Bool("check", false, "compara as licencas com o baseline e registra alertas")
		status    = flag.Bool("status", false, "imprime o estado de todas as acoes")
		revertAll = flag.Bool("revert-all", false, "desfaz todas as alteracoes feitas pelo ProtegeWX")
		aplicar   = flag.String("aplicar", "", "aplica as acoes indicadas, separadas por virgula (ex: A1,A2,B3)")
		reverter  = flag.String("reverter", "", "reverte as acoes indicadas, separadas por virgula")
		porta     = flag.Int("porta", 0, "porta do painel (0 = automatica)")
	)
	flag.BoolVar(&silencioso, "silencioso", false,
		"nao escreve no console; em caso de alerta critico, avisa numa janela. Usado pelo monitor automatico")
	flag.BoolVar(&verbose, "v", false, "detalha os comandos executados")
	flag.Parse()

	exe, err := os.Executable()
	if err != nil {
		fatal("nao foi possivel localizar o proprio executavel: %v", err)
	}
	baseDir = filepath.Dir(exe)
	// permite rodar via "go run" a partir da raiz do projeto
	if _, err := os.Stat(filepath.Join(baseDir, "web")); err != nil {
		if wd, err := os.Getwd(); err == nil {
			baseDir = wd
		}
	}

	abrirLog()
	defer arqLog.Close()

	ctx = actions.NovoCtx(baseDir, exe)

	switch {
	case *check:
		os.Exit(modoCheck())
	case *status:
		modoStatus()
		return
	case *revertAll:
		os.Exit(modoRevertAll())
	case *aplicar != "":
		os.Exit(modoLote(*aplicar, false))
	case *reverter != "":
		os.Exit(modoLote(*reverter, true))
	}

	if !ehAdmin() {
		fmt.Println("ProtegeWX precisa de privilegios de administrador. Solicitando elevacao...")
		if err := relancarElevado(exe); err != nil {
			fatal("nao foi possivel elevar: %v\nAbra o prompt como administrador e rode novamente.", err)
		}
		return
	}
	servirPainel(*porta)
}

// ---------------------------------------------------------------- log

func abrirLog() {
	dir := filepath.Join(baseDir, "logs")
	_ = os.MkdirAll(dir, 0o755)
	f, err := os.OpenFile(filepath.Join(dir, "protegewx.log"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	arqLog = f
	sysexec.Logger = func(r sysexec.Resultado) {
		registrar("CMD  %s  -> saida %d (%s)", r.Linha, r.ExitCode, r.Duracao.Round(time.Millisecond))
		if verbose && r.Saida != "" {
			registrar("     %s", strings.ReplaceAll(r.Saida, "\n", "\n     "))
		}
	}
}

func registrar(formato string, args ...any) {
	linha := fmt.Sprintf("%s  %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(formato, args...))
	if arqLog != nil {
		_, _ = arqLog.WriteString(linha)
	}
	if verbose {
		fmt.Print(linha)
	}
}

func fatal(formato string, args ...any) {
	fmt.Fprintf(os.Stderr, "erro: "+formato+"\n", args...)
	os.Exit(1)
}

// ---------------------------------------------------------------- modos de console

func modoCheck() int {
	alertas, err := actions.Conferir(ctx)
	if err != nil {
		registrar("CHECK falhou: %v", err)
		if !silencioso {
			fmt.Fprintf(os.Stderr, "nao foi possivel conferir: %v\n", err)
		}
		return 2
	}
	if len(alertas) == 0 {
		registrar("CHECK ok - nenhuma divergencia em relacao ao baseline")
		if !silencioso {
			fmt.Println("Sem divergencias: as licencas estao como no baseline.")
		}
		return 0
	}

	criticos := 0
	var resumo strings.Builder
	if !silencioso {
		fmt.Printf("%d divergencia(s) encontrada(s):\n\n", len(alertas))
	}
	for _, a := range alertas {
		if a.Gravidade == "critico" {
			criticos++
			fmt.Fprintf(&resumo, "â€¢ %s\n  %s\n\n", a.Assunto, a.Detalhe)
		}
		registrar("CHECK [%s] %s: %s", a.Gravidade, a.Assunto, a.Detalhe)
		if !silencioso {
			fmt.Printf("  [%s] %s\n      %s\n\n", strings.ToUpper(a.Gravidade), a.Assunto, a.Detalhe)
		}
	}
	gravarAlertas(alertas)

	// Rodando pelo monitor automatico nao ha console para ler. Um alerta critico
	// significa que algo mudou no conteudo das licencas, e isso precisa aparecer
	// na frente da pessoa - nao ficar so registrado num arquivo de log.
	if silencioso && criticos > 0 {
		avisarNaTela("ProtegeWX - alteracao nas suas licencas",
			"O ProtegeWX detectou "+fmt.Sprint(criticos)+" alteracao(oes) grave(s) nas licencas dos seus dongles:\n\n"+
				resumo.String()+
				"Abra o ProtegeWX para ver os detalhes.\n"+
				"O registro completo esta em logs\\protegewx.log")
	}

	if criticos > 0 {
		return 1
	}
	return 0
}

// avisarNaTela mostra uma janela do Windows com o alerta.
func avisarNaTela(titulo, texto string) {
	const mbIconWarning = 0x00000030
	const mbSystemModal = 0x00001000
	t, _ := syscall.UTF16PtrFromString(texto)
	c, _ := syscall.UTF16PtrFromString(titulo)
	procMessageBox.Call(0,
		uintptr(unsafe.Pointer(t)),
		uintptr(unsafe.Pointer(c)),
		uintptr(mbIconWarning|mbSystemModal))
}

func gravarAlertas(alertas any) {
	b, err := json.MarshalIndent(alertas, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(baseDir, "logs", "ultimo-alerta.json"), b, 0o644)
}

func modoStatus() {
	for _, g := range actions.Grupos() {
		fmt.Printf("\n== %s - %s\n", g.ID, g.Nome)
		for _, a := range actions.Todas() {
			if a.Grupo != g.ID {
				continue
			}
			fmt.Printf("  %-4s %-12s %s\n", a.ID, a.Ler(ctx), a.Titulo)
		}
	}
	if exposta, addrs, err := sentinel.EscutaExposta(); err == nil {
		fmt.Println()
		if exposta {
			fmt.Printf("ATENCAO: a porta 1947 ainda escuta em %s\n", strings.Join(addrs, ", "))
		} else {
			fmt.Println("A porta 1947 escuta somente no loopback.")
		}
	}
	fmt.Println()
}

// modoLote aplica ou reverte uma lista de acoes pela linha de comando.
// Serve para automacao e para aplicar grupo a grupo de forma controlada.
func modoLote(lista string, reverter bool) int {
	if !ehAdmin() {
		fmt.Fprintln(os.Stderr, "erro: esta operacao precisa de um prompt como administrador.")
		return 2
	}
	verbo := "aplicando"
	if reverter {
		verbo = "revertendo"
	}

	var ids []string
	for _, p := range strings.Split(lista, ",") {
		if p = strings.TrimSpace(strings.ToUpper(p)); p != "" {
			ids = append(ids, p)
		}
	}
	if reverter {
		for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
			ids[i], ids[j] = ids[j], ids[i]
		}
	}

	falhas := 0
	for _, id := range ids {
		a, ok := actions.Por(id)
		if !ok {
			fmt.Printf("%-4s acao desconhecida\n", id)
			falhas++
			continue
		}
		fmt.Printf("%-4s %s %s ... ", a.ID, verbo, a.Titulo)
		var err error
		if reverter {
			err = a.Reverter(ctx)
		} else {
			err = a.Aplicar(ctx)
		}
		if err != nil {
			falhas++
			fmt.Printf("FALHOU\n     %v\n", err)
			registrar("LOTE %s %s falhou: %v", verbo, id, err)
			continue
		}
		fmt.Printf("ok (estado: %s)\n", a.Ler(ctx))
		registrar("LOTE %s %s ok", verbo, id)
	}
	if falhas > 0 {
		fmt.Printf("\n%d de %d acao(oes) falharam.\n", falhas, len(ids))
		return 1
	}
	fmt.Printf("\n%d acao(oes) concluidas.\n", len(ids))
	return 0
}

func modoRevertAll() int {
	falhas := 0
	todas := actions.Todas()
	// reverte na ordem inversa da aplicacao
	for i := len(todas) - 1; i >= 0; i-- {
		a := todas[i]
		if a.Ler(ctx) == actions.NaoAplicado {
			continue
		}
		fmt.Printf("revertendo %s %s ... ", a.ID, a.Titulo)
		if err := a.Reverter(ctx); err != nil {
			falhas++
			fmt.Printf("FALHOU: %v\n", err)
			registrar("REVERT %s falhou: %v", a.ID, err)
			continue
		}
		fmt.Println("ok")
		registrar("REVERT %s ok", a.ID)
	}
	if falhas > 0 {
		fmt.Printf("\n%d acao(oes) nao puderam ser revertidas. Veja logs/protegewx.log\n", falhas)
		return 1
	}
	fmt.Println("\nTudo revertido.")
	return 0
}

// ---------------------------------------------------------------- painel

func servirPainel(porta int) {
	sub, err := protegewx.Painel()
	if err != nil {
		fatal("conteudo do painel ausente: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/estado", apiEstado)
	mux.HandleFunc("/api/aplicar", apiAplicar)
	mux.HandleFunc("/api/reverter", apiReverter)
	mux.HandleFunc("/api/senha", apiSenha)
	mux.HandleFunc("/api/check", apiCheck)

	lis, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", porta))
	if err != nil {
		fatal("nao foi possivel abrir o painel: %v", err)
	}
	url := fmt.Sprintf("http://%s/", lis.Addr().String())

	fmt.Println()
	fmt.Println("  " + marca.Nome + " - " + marca.Assinatura)
	fmt.Println("  Painel em " + url)
	fmt.Println("  Feche esta janela para encerrar.")
	fmt.Println()
	registrar("painel iniciado em %s", url)

	go abrirNavegador(url)
	if err := http.Serve(lis, mux); err != nil {
		fatal("painel encerrado: %v", err)
	}
}

func responder(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

type acaoJSON struct {
	actions.Acao
	Estado   actions.Estado `json:"estado"`
	Comandos []string       `json:"comandos"`
}

func apiEstado(w http.ResponseWriter, r *http.Request) {
	tipo := struct {
		Grupos      []actions.Grupo    `json:"grupos"`
		Acoes       []acaoJSON         `json:"acoes"`
		Diagnostico map[string]any     `json:"diagnostico"`
		Chaves      []sentinel.Chave   `json:"chaves"`
		Features    []sentinel.Feature `json:"features"`
	}{Grupos: actions.Grupos(), Diagnostico: map[string]any{}}

	for _, a := range actions.Todas() {
		tipo.Acoes = append(tipo.Acoes, acaoJSON{Acao: a, Estado: a.Ler(ctx), Comandos: a.Comandos(ctx)})
	}
	if cs, err := sentinel.Chaves(); err == nil {
		// na compilacao de distribuicao os numeros de serie saem mascarados,
		// para que um print do painel possa ser publicado sem expor as chaves
		for i := range cs {
			cs[i].HaspID = marca.Mascarar(cs[i].HaspID)
		}
		tipo.Chaves = cs
	}
	if fs, err := sentinel.Features(); err == nil {
		for i := range fs {
			fs[i].HaspID = marca.Mascarar(fs[i].HaspID)
		}
		tipo.Features = fs
	}
	tipo.Diagnostico["comunidade"] = marca.Comunidade
	if cfg, err := sentinel.LerConfig(); err == nil {
		tipo.Diagnostico["config"] = cfg
	}
	if exposta, addrs, err := sentinel.EscutaExposta(); err == nil {
		tipo.Diagnostico["portaExposta"] = exposta
		tipo.Diagnostico["enderecosExpostos"] = addrs
	}
	tipo.Diagnostico["regrasCriadas"] = fw.Listar()
	tipo.Diagnostico["temSenhaDefinida"] = ctx.SenhaACC() != ""
	responder(w, tipo)
}

type pedido struct {
	IDs []string `json:"ids"`
}

type resultadoItem struct {
	ID    string `json:"id"`
	Ok    bool   `json:"ok"`
	Erro  string `json:"erro,omitempty"`
	Nova  string `json:"novoEstado"`
}

func executar(w http.ResponseWriter, r *http.Request, reverter bool) {
	var p pedido
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "pedido invalido", http.StatusBadRequest)
		return
	}
	verbo := "APLICAR"
	if reverter {
		verbo = "REVERTER"
	}

	var saida []resultadoItem
	ids := p.IDs
	if reverter {
		// reverter na ordem inversa
		ids = make([]string, len(p.IDs))
		for i, id := range p.IDs {
			ids[len(p.IDs)-1-i] = id
		}
	}
	for _, id := range ids {
		a, ok := actions.Por(id)
		if !ok {
			saida = append(saida, resultadoItem{ID: id, Erro: "acao desconhecida"})
			continue
		}
		var err error
		if reverter {
			err = a.Reverter(ctx)
		} else {
			err = a.Aplicar(ctx)
		}
		item := resultadoItem{ID: id, Ok: err == nil}
		if err != nil {
			item.Erro = err.Error()
			registrar("%s %s FALHOU: %v", verbo, id, err)
		} else {
			registrar("%s %s ok", verbo, id)
		}
		item.Nova = string(a.Ler(ctx))
		saida = append(saida, item)
	}
	responder(w, saida)
}

func apiAplicar(w http.ResponseWriter, r *http.Request)  { executar(w, r, false) }
func apiReverter(w http.ResponseWriter, r *http.Request) { executar(w, r, true) }

func apiSenha(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Senha string `json:"senha"`
	}
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "pedido invalido", http.StatusBadRequest)
		return
	}
	if err := ctx.DefinirSenhaACC(p.Senha); err != nil {
		responder(w, map[string]any{"ok": false, "erro": err.Error()})
		return
	}
	registrar("senha do ACC definida (guardada em estado.json)")
	responder(w, map[string]any{"ok": true})
}

func apiCheck(w http.ResponseWriter, r *http.Request) {
	alertas, err := actions.Conferir(ctx)
	if err != nil {
		responder(w, map[string]any{"ok": false, "erro": err.Error()})
		return
	}
	responder(w, map[string]any{"ok": true, "alertas": alertas})
}

// ---------------------------------------------------------------- windows

var (
	shell32          = syscall.NewLazyDLL("shell32.dll")
	procIsUserAdmin  = shell32.NewProc("IsUserAnAdmin")
	procShellExecute = shell32.NewProc("ShellExecuteW")

	user32         = syscall.NewLazyDLL("user32.dll")
	procMessageBox = user32.NewProc("MessageBoxW")
)

func ehAdmin() bool {
	r, _, _ := procIsUserAdmin.Call()
	return r != 0
}

func relancarElevado(exe string) error {
	verbo, _ := syscall.UTF16PtrFromString("runas")
	arquivo, _ := syscall.UTF16PtrFromString(exe)
	dir, _ := syscall.UTF16PtrFromString(filepath.Dir(exe))
	r, _, err := procShellExecute.Call(
		0,
		uintptr(unsafe.Pointer(verbo)),
		uintptr(unsafe.Pointer(arquivo)),
		0,
		uintptr(unsafe.Pointer(dir)),
		1, // SW_SHOWNORMAL
	)
	if r <= 32 {
		return fmt.Errorf("ShellExecute devolveu %d: %v", r, err)
	}
	return nil
}

func abrirNavegador(url string) {
	time.Sleep(400 * time.Millisecond)
	if !sysexec.Rodar("rundll32", "url.dll,FileProtocolHandler", url).Ok() {
		fmt.Printf("  (nao foi possivel abrir o navegador; acesse %s manualmente)\n", url)
	}
}

