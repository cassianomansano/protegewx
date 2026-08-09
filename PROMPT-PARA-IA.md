# Modelo de prompt para pedir ajuda a uma IA

Se você baixou os fontes e prefere que uma IA (Claude Code, ChatGPT, Copilot,
Gemini, Cursor…) coloque o projeto para rodar, copie um dos blocos abaixo.

Escolha conforme a ferramenta:

- **Prompt 1** — para IA com acesso ao seu terminal (Claude Code, Cursor, Copilot
  agent, Windsurf…). Ela executa os comandos.
- **Prompt 2** — para IA de chat sem acesso à máquina (ChatGPT, Gemini no
  navegador). Ela te guia e você executa.
- **Prompt 3** — quando algo deu errado e você quer ajuda para diagnosticar.

> Antes de colar: os prompts pedem que a IA **não aplique nada** sem te explicar.
> Isso é de propósito. Se alguma IA quiser sair aplicando tudo de uma vez, é sinal
> de que ela não leu — mande rodar `--status` primeiro.

---

## Prompt 1 — IA com acesso ao terminal

```
Preciso compilar e rodar um projeto Go em Windows chamado ProtegeWX, que já
baixei. Ele isola da rede os dongles Sentinel HL e os programas PC SOFT
(WINDEV/WEBDEV/WINDEV Mobile).

Contexto importante para você entender antes de agir:
- É um projeto Go que usa SOMENTE a biblioteca padrão. Não há dependências
  externas e não é preciso compilador C (compila com CGO_ENABLED=0).
- O painel (HTML/CSS/JS/imagens) é embutido no executável via go:embed, então o
  build gera um .exe único e autossuficiente.
- Ele mexe em firewall do Windows, no arquivo hosts e na configuração do serviço
  hasplms. Por isso precisa rodar como administrador.

O que eu quero que você faça, nesta ordem:

1. Verifique se o Go está instalado com "go version". Se não estiver, instale com
   "winget install --id GoLang.Go -e --source winget" e me avise que preciso
   reabrir o terminal para o PATH atualizar.

2. Na raiz do projeto, rode "go vet ./..." e depois:
   go build -o protegewx.exe ./cmd/protegewx

3. Rode "protegewx.exe --status". Esse comando é somente leitura e não altera
   nada. Me mostre a saída e me explique, em português simples, o que está
   exposto hoje na minha máquina.

4. PARE AÍ e me pergunte antes de aplicar qualquer coisa.

Regras que você deve seguir:
- NÃO aplique nenhuma ação sem me explicar antes o que ela faz e me perguntar.
- NÃO rode "--revert-all" nem "--aplicar" por conta própria.
- Se for aplicar algo depois que eu autorizar, comece por D1 e D2 (que só fazem
  registro e backup, sem alterar o sistema), e me avise que o grupo A reinicia o
  serviço hasplms e exige que eu feche o WINDEV antes, senão eu posso perder
  alterações não salvas.
- Se algum comando falhar, me mostre a mensagem de erro real em vez de tentar
  contornar sozinho.
```

---

## Prompt 2 — IA de chat, sem acesso à máquina

```
Vou compilar um projeto Go em Windows e preciso que você me guie passo a passo,
esperando eu confirmar cada etapa antes de passar para a próxima.

Sobre o projeto:
- Nome: ProtegeWX. Linguagem: Go. Sistema: Windows 10/11.
- Usa somente a biblioteca padrão do Go: sem dependências externas, sem
  necessidade de compilador C.
- O comando de build é: go build -o protegewx.exe ./cmd/protegewx
- O executável precisa de privilégios de administrador para funcionar.

Me ajude com:
1. Instalar o Go no Windows e confirmar que ficou no PATH.
2. Abrir o terminal na pasta certa do projeto.
3. Compilar.
4. Rodar "protegewx.exe --status", que é somente leitura, e me ajudar a
   interpretar a saída.

Considere que eu sei programar, mas não tenho prática com Go nem com linha de
comando do Windows. Seja direto, um passo por vez, e me diga o que devo ver na
tela quando der certo — para eu saber se funcionou antes de seguir.

Não me mande aplicar nenhuma configuração ainda. Só quero compilar e ver o
diagnóstico primeiro.
```

---

## Prompt 3 — quando deu erro

```
Estou compilando/rodando o projeto Go ProtegeWX no Windows e deu erro.

Ambiente:
- Saída de "go version": [COLE AQUI]
- Comando que rodei: [COLE AQUI]
- Erro completo: [COLE AQUI]

Informações sobre o projeto que podem ajudar no diagnóstico:
- Usa apenas a biblioteca padrão do Go, sem dependências externas.
- Compila com CGO_ENABLED=0, sem precisar de gcc nem MSVC.
- O painel é embutido com go:embed a partir das pastas web/ e webcom/. Se essas
  pastas estiverem faltando ou vazias, o build falha na diretiva de embed.
- Existem duas variantes por build tag: a padrão e a "comunidade"
  (go build -tags comunidade). Arquivos com //go:build comunidade só entram na
  segunda; arquivos com //go:build !comunidade só entram na primeira.
- Ele executa netsh, icacls, sc e powershell. Sem privilégio de administrador,
  esses comandos falham com acesso negado.

Me diga qual é a causa provável e como corrigir. Se precisar ver algum arquivo
do projeto para ter certeza, me peça o arquivo em vez de adivinhar.
```

---

## Erros comuns, sem precisar de IA

| Erro | Causa | Solução |
|---|---|---|
| `'go' não é reconhecido` | Go não instalado, ou terminal aberto antes da instalação | Feche e reabra o terminal |
| `pattern all:web: no matching files` | pasta `web/` ausente ou vazia | Baixe o repositório completo, não só a pasta `cmd` |
| `Acesso negado` ao aplicar | terminal sem elevação | Abra o PowerShell como administrador |
| `ACC inacessivel em http://127.0.0.1:1947` | serviço do dongle parado | `sc start hasplms` |
| Nenhum dongle listado | chave desconectada, ou driver Sentinel ausente | Conecte o dongle e abra `http://127.0.0.1:1947` |
| `O serviço 'Agendador de tarefas' não está sendo executado` | serviço `Schedule` desabilitado no Windows | Use `--check` manual, ou reative o serviço `Schedule` |

---

## O comando mais seguro para começar

```powershell
.\protegewx.exe --status
```

Somente leitura. Não altera absolutamente nada. Use-o antes e depois de qualquer
coisa que você aplicar.
