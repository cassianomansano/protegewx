# Contribuindo

Obrigado por querer ajudar. Este projeto existe porque alguém precisou disso e
resolveu não guardar só para si — continua assim por causa de quem devolve.

## Antes de tudo: nunca envie dados da sua máquina

O `.gitignore` já bloqueia, mas confira antes de abrir um PR:

- `baseline/` — contém os **números de série dos seus dongles** e suas licenças
- `backup/`, `logs/`, `estado.json` — descrevem a sua máquina (e `estado.json`
  guarda a senha do seu Admin Control Center)
- qualquer `.c2v`, `.v2c` ou `hasplm.ini`

Se for colar saída de terminal num issue, **mascare os números de série**. Uma
forma fácil: rode a edição da comunidade, que já mostra mascarado na tela.

## Compilando

Só a biblioteca padrão do Go, sem dependências:

```powershell
go build -o protegewx.exe ./cmd/protegewx
```

Para as duas variantes e os pacotes:

```powershell
.\scripts\build.ps1 -Pacote
```

Antes de abrir PR:

```powershell
go vet ./...
go vet -tags comunidade ./...
```

## O que é mais útil contribuir

**Domínios novos para bloquear.** É a contribuição mais valiosa e a mais fácil.
Se você descobrir um endereço da PC SOFT ou da Thales que os programas contatam,
abra uma issue. Só peço uma coisa: **confirme que o domínio existe de verdade**
antes de propor.

```powershell
[System.Net.Dns]::GetHostAddresses('dominio.exemplo')
```

Já removemos vários nomes plausíveis que simplesmente não existiam. Bloquear
domínio inexistente não protege nada e só faz a lista parecer maior do que é.

**Compatibilidade com outras versões.** Se você tem WINDEV 25, 28, 2024 ou uma
combinação diferente, rode `--status` e conte o que aconteceu. A detecção é
automática, mas só foi testada de verdade em algumas máquinas.

**Outros modelos de dongle.** O projeto foi construído em cima de chaves
`Sentinel HL Pro`. Se a sua for diferente e algo se comportar de outro jeito,
queremos saber.

**Tradução, correção de texto, clareza.** As explicações do painel são a parte
mais importante do programa — é o que faz alguém entender o que está aplicando.

## Como escrever uma ação nova

Todas as ações vivem em `internal/actions/registry.go`. Cada uma declara, além do
código, a explicação do que faz, por que existe, o risco e como desfazer. O painel
monta a interface a partir desses campos.

```go
{
    ID: "X1", Grupo: "B", Risco: RiscoBaixo, Padrao: true,
    Titulo:  "frase curta, no infinitivo",
    OQueFaz: "o que muda na máquina, em português claro",
    PorQue:  "qual ameaça concreta isso fecha",
    Reverte: "como desfazer",
    Comandos: func(c *Ctx) []string { /* o que será exibido ANTES de rodar */ },
    Aplicar:  func(c *Ctx) error { /* ... */ },
    Reverter: func(c *Ctx) error { /* ... */ },
    Ler:      func(c *Ctx) Estado { /* lê o estado REAL do sistema */ },
}
```

Três regras que o projeto leva a sério:

1. **Toda ação precisa de `Reverter` que funcione.** Sem exceção.
2. **`Ler` consulta o sistema, não um arquivo de estado nosso.** Se o usuário
   desfizer algo por fora, o painel tem que perceber.
3. **`Comandos` mostra o comando de verdade**, o mesmo que será executado. Não é
   um resumo bonito: é a promessa central do programa para quem usa.

## Estilo

- Comentários e textos de interface em **português**
- Comentário explica **por que**, não o que a linha faz
- Sem dependências externas — se precisar de uma, abra uma issue antes
- Mensagem de erro deve dizer o que fazer, não só que falhou

## Reportando um problema

Inclua: versão do Windows, versão do WINDEV/WEBDEV, o que você rodou, o que
esperava e o que aconteceu. A saída de `--status` ajuda muito — **com os seriais
mascarados**.

Se o programa fez algo inesperado na sua máquina, diga logo de cara: prioridade
máxima é não estragar a máquina de ninguém.
