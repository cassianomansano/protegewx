<#
    ProtegeWX - build das duas variantes

      protegewx.exe             versao padrao, com os dados completos
      protegewx-comunidade.exe  versao de distribuicao, sem logo propria e com os
                                numeros de serie mascarados

    A separacao e por build tag: o binario da comunidade NAO contem a logo da
    ProtegeWX, porque embute webcom/ em vez de web/. Nao e so questao de esconder
    na tela - o arquivo distribuido nao carrega a marca de ninguem.

    O CSS e o JS tem fonte unica em web/. Este script copia os dois para webcom/
    e concatena webcom/extra.css ao final do CSS, de modo que a paleta nao
    precise ser mantida em dois lugares.
#>

[CmdletBinding()]
param(
    # Por padrao, a raiz do projeto e a pasta que contem este script.
    # Assim o build funciona em qualquer maquina, sem caminho fixo.
    [string]$Base = (Split-Path -Parent $PSScriptRoot),
    [switch]$Pacote   # gera tambem o zip de distribuicao
)

$ErrorActionPreference = 'Stop'
$ProgressPreference    = 'SilentlyContinue'

$env:Path = [Environment]::GetEnvironmentVariable('Path','Machine') + ';' +
            [Environment]::GetEnvironmentVariable('Path','User')

Set-Location $Base

Write-Host ""
Write-Host "ProtegeWX - build" -ForegroundColor Cyan
Write-Host ""

# ---------------------------------------------------------------- assets compartilhados
Write-Host "[  ] sincronizando assets da edicao da comunidade" -NoNewline
Copy-Item "$Base\web\app.js" "$Base\webcom\app.js" -Force

$css = Get-Content "$Base\web\style.css" -Raw

# O cabecalho de web/style.css cita a origem da paleta e a marca. Ele nao pode
# viajar junto para a edicao da comunidade, entao e trocado por um neutro.
$cabecalhoNeutro = @'
/* ProtegeWX â€” ediÃ§Ã£o da comunidade
   Paleta em tons de teal profundo, pensada para leitura confortÃ¡vel em telas
   escuras e para dar contraste claro entre "aplicado", "nÃ£o aplicado" e risco. */
'@
$css = [regex]::Replace($css, '^\s*/\*.*?\*/', $cabecalhoNeutro, 'Singleline')

$extra = Get-Content "$Base\webcom\extra.css" -Raw
Set-Content -Path "$Base\webcom\style.css" -Encoding utf8 -Value ($css + "`n`n" + $extra)
Write-Host "`r[OK] assets da edicao da comunidade sincronizados"

# a logo da instalacao nunca pode acabar em webcom/
$vazou = Get-ChildItem "$Base\webcom" -Filter '*ProtegeWX*' -ErrorAction SilentlyContinue
if ($vazou) {
    Write-Host "[!!] ABORTADO: arquivo de marca encontrado em webcom/: $($vazou.Name)" -ForegroundColor Red
    exit 1
}

# ---------------------------------------------------------------- compilacao
Write-Host "[  ] go vet" -NoNewline
go vet ./...
go vet -tags comunidade ./...
Write-Host "`r[OK] go vet"

Write-Host "[  ] compilando protegewx.exe (padrao)" -NoNewline
go build -trimpath -ldflags "-s -w" -o protegewx.exe ./cmd/protegewx
Write-Host "`r[OK] protegewx.exe            $('{0:N1} MB' -f ((Get-Item protegewx.exe).Length/1MB))"

Write-Host "[  ] compilando protegewx-comunidade.exe" -NoNewline
go build -tags comunidade -trimpath -ldflags "-s -w" -o protegewx-comunidade.exe ./cmd/protegewx
Write-Host "`r[OK] protegewx-comunidade.exe $('{0:N1} MB' -f ((Get-Item protegewx-comunidade.exe).Length/1MB))"

# ---------------------------------------------------------------- conferencia
# o binario da comunidade nao pode conter a marca nem o numero de serie de ninguem
Write-Host "[  ] conferindo o binario de distribuicao" -NoNewline
$bytes = [IO.File]::ReadAllBytes("$Base\protegewx-comunidade.exe")
$texto = [Text.Encoding]::ASCII.GetString($bytes)

$proibidos = @('logo-ProtegeWX', 'ProtegeWX')
$baseFile  = "$Base\baseline\chaves-baseline.json"
if (Test-Path $baseFile) {
    $b = Get-Content $baseFile -Raw | ConvertFrom-Json
    $proibidos += @($b.chaves | ForEach-Object { $_.haspid })
}

$achados = @()
foreach ($p in $proibidos) {
    if ($p -and $texto.ToLower().Contains($p.ToLower())) { $achados += $p }
}
if ($achados) {
    Write-Host "`r[!!] ABORTADO: o binario de distribuicao contem: $($achados -join ', ')" -ForegroundColor Red
    Remove-Item "$Base\protegewx-comunidade.exe" -Force
    exit 1
}
Write-Host "`r[OK] binario de distribuicao limpo (sem marca e sem numeros de serie)"

# ---------------------------------------------------------------- pacote
if ($Pacote) {
    $dist = "$Base\dist"
    Remove-Item $dist -Recurse -Force -ErrorAction SilentlyContinue

    $docs = @('LEIAME-COMUNIDADE.md', 'README.md', 'PROMPT-PARA-IA.md',
              'AVISO-LEGAL.md', 'LICENSE')

    # ---------- pacote 1: executavel pronto para usar ----------
    $stage = "$dist\ProtegeWX-Comunidade"
    New-Item -ItemType Directory -Force -Path $stage | Out-Null
    Copy-Item "$Base\protegewx-comunidade.exe" "$stage\protegewx.exe" -Force
    foreach ($f in $docs) { if (Test-Path "$Base\$f") { Copy-Item "$Base\$f" $stage -Force } }

    # o instalador e os atalhos vao junto: e por eles que a pessoa comeca
    New-Item -ItemType Directory -Force -Path "$stage\scripts" | Out-Null
    Copy-Item "$Base\scripts\instalar.ps1"      "$stage\scripts\" -Force
    Copy-Item "$Base\scripts\criar-atalhos.ps1" "$stage\scripts\" -Force

    $zipBin = "$dist\ProtegeWX-Comunidade.zip"
    Compress-Archive -Path "$stage\*" -DestinationPath $zipBin -Force
    Write-Host "[OK] pacote binario: $zipBin  $('{0:N1} MB' -f ((Get-Item $zipBin).Length/1MB))"

    # ---------- pacote 2: codigo-fonte ----------
    # Leva so o que e codigo. Ficam de fora backups, logs, baseline, estado.json
    # e binarios - ou seja, tudo que descreve a maquina de quem compilou.
    $src = "$dist\ProtegeWX-Fontes"
    New-Item -ItemType Directory -Force -Path $src | Out-Null

    Copy-Item "$Base\go.mod" $src -Force
    Copy-Item "$Base\web_comunidade.go" $src -Force
    Copy-Item "$Base\web.go" $src -Force
    Copy-Item "$Base\cmd"      $src -Recurse -Force
    Copy-Item "$Base\internal" $src -Recurse -Force
    Copy-Item "$Base\webcom"   $src -Recurse -Force
    foreach ($f in $docs) { if (Test-Path "$Base\$f") { Copy-Item "$Base\$f" $src -Force } }

    New-Item -ItemType Directory -Force -Path "$src\scripts" | Out-Null
    Copy-Item "$Base\scripts\build.ps1" "$src\scripts\" -Force

    # web/ do pacote de fontes recebe o painel neutro, e nao o da ProtegeWX:
    # assim o build padrao (sem tags) tambem compila, e nenhuma variante carrega
    # marca de terceiro.
    Copy-Item "$Base\webcom" "$src\web" -Recurse -Force
    Remove-Item "$src\web\extra.css" -Force -ErrorAction SilentlyContinue

    # ---------- sanitizacao do pacote de fontes ----------
    # O codigo interno cita a ProtegeWX em comentarios e na assinatura da variante
    # padrao. No pacote publico isso vira texto neutro, e o arquivo de marca e
    # renomeado - o proximo a compilar nao herda a identidade de ninguem.
    Rename-Item "$src\internal\marca\marca_ProtegeWX.go" 'marca_padrao.go' -Force

    $trocas = [ordered]@{
        'ProtegeWX' = 'ProtegeWX'
        'ProtegeWX' = 'ProtegeWX'
        'versao padrao, com os dados completos' = 'versao padrao, com os dados completos'
        'identidade padrao'                     = 'identidade padrao'
        'que contem a identidade padrao'        = 'padrao'
        'a logo da instalacao'                      = 'a logo da instalacao'
        'logo propria'                        = 'logo propria'
        'compilando protegewx.exe (padrao)'     = 'compilando protegewx.exe (padrao)'
        'versao padrao'                         = 'versao padrao'
        'ProtegeWX'                                = 'ProtegeWX'
    }
    Get-ChildItem $src -Recurse -File -Include '*.go','*.md','*.ps1','*.html','*.css','*.js' | ForEach-Object {
        $t = Get-Content $_.FullName -Raw
        if ($t -match '(?i)ProtegeWX') {
            foreach ($k in $trocas.Keys) { $t = $t -replace [regex]::Escape($k), $trocas[$k] }
            $t = $t -replace '(?i)ProtegeWX', 'ProtegeWX'
            Set-Content -Path $_.FullName -Encoding utf8 -Value $t
        }
    }

    # conferencia final do pacote de fontes
    $sujos = @()
    Get-ChildItem $src -Recurse -File | ForEach-Object {
        if ($_.Name -match '(?i)ProtegeWX') { $sujos += $_.FullName; return }
        if ($_.Extension -in @('.go','.md','.ps1','.html','.css','.js','.json','.mod')) {
            $t = Get-Content $_.FullName -Raw -ErrorAction SilentlyContinue
            if ($t -and $t -match '(?i)ProtegeWX') { $sujos += $_.FullName }
        }
    }
    if ($sujos) {
        Write-Host "[!!] ABORTADO: pacote de fontes contem referencias a marca:" -ForegroundColor Red
        $sujos | ForEach-Object { Write-Host "     $_" -ForegroundColor Red }
        exit 1
    }

    $zipSrc = "$dist\ProtegeWX-Fontes.zip"
    Compress-Archive -Path "$src\*" -DestinationPath $zipSrc -Force
    Write-Host "[OK] pacote de fontes: $zipSrc  $('{0:N1} MB' -f ((Get-Item $zipSrc).Length/1MB))"
}

Write-Host ""
Write-Host "Build concluido." -ForegroundColor Green
Write-Host ""

