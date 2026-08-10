# Material de divulgação

Pegue e compartilhe à vontade. Quanto mais gente souber que o painel do dongle
abre sem senha, melhor.

| Arquivo | Para quê |
|---|---|
| [`card-protegewx.png`](card-protegewx.png) | grupos de WhatsApp e Telegram, redes sociais |
| [`card-protegewx.pdf`](card-protegewx.pdf) | imprimir, ou mandar sem perder qualidade |
| [`card-protegewx.html`](card-protegewx.html) | a fonte, caso queira adaptar ou traduzir |

## Mandando no WhatsApp

Envie o PNG **como documento**, não como foto.

Foto passa pela compressão do WhatsApp e o texto menor fica ilegível. Como
documento, chega igual ao original. O PDF também não sofre compressão.

## Gerando de novo depois de editar

O card é uma página HTML renderizada pelo Chrome em modo headless:

```powershell
$chrome = "$env:ProgramFiles\Google\Chrome\Application\chrome.exe"
$html   = "file:///$((Resolve-Path card-protegewx.html).Path.Replace('\','/'))"

& $chrome --headless --disable-gpu --hide-scrollbars `
  --force-device-scale-factor=2 --window-size=1080,2110 `
  --screenshot=card-protegewx.png $html

& $chrome --headless --disable-gpu --virtual-time-budget=5000 `
  --print-to-pdf-no-header --print-to-pdf=card-protegewx.pdf $html
```

> O Chrome em modo headless não consegue gravar dentro de `C:\Windows\System32`.
> Se for gerar a partir de uma pasta assim, grave em outro lugar e copie depois.

## Se for adaptar

Fique à vontade para traduzir ou ajustar. Dois pedidos:

- **Mantenha o bloco vermelho do `127.0.0.1:1947`.** É o que faz a pessoa testar
  na hora e entender o problema em cinco segundos, antes de qualquer explicação.
- **Mantenha o bloco "O que ele NÃO é".** Sem ele, muita gente lê o título e
  conclui que é ferramenta de pirataria — que é exatamente o oposto do projeto.
