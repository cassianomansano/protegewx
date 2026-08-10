<#
    ProtegeWX - instalacao em uma maquina nova

    Clique com o botao direito neste arquivo e escolha
    "Executar com o PowerShell", ou rode num PowerShell como administrador:

        .\instalar.ps1

    O que ele faz:
      1. confere se a maquina tem dongle e o servico do Sentinel
      2. copia o ProtegeWX para C:\ProtegeWX
      3. registra o estado atual das licencas (o retrato de referencia)
      4. cria os atalhos na Area de Trabalho
      5. mostra o diagnostico da maquina

    O que ele NAO faz: nenhuma protecao e aplicada automaticamente. Depois de
    instalar, voce abre o painel e escolhe o que quer ligar.
#>

[CmdletBinding()]
param(
    [string]$Destino = 'C:\ProtegeWX'
)

$ErrorActionPreference = 'Stop'

# Pausar espera o ENTER para a janela nao fechar sozinha quando o script e
# aberto com duplo clique. Rodando de forma automatizada nao ha console para
# ler, e nesse caso a espera e simplesmente ignorada.
function Pausar {
    if ($Host.UI.RawUI -and -not [Environment]::UserInteractive) { return }
    try { Read-Host "  Pressione ENTER para sair" | Out-Null } catch { }
}

function Titulo($t) { Write-Host ""; Write-Host $t -ForegroundColor Cyan }
function Ok($t)     { Write-Host "  [OK] $t" -ForegroundColor Green }
function Aviso($t)  { Write-Host "  [!]  $t" -ForegroundColor Yellow }
function Erro($t)   { Write-Host "  [X]  $t" -ForegroundColor Red }

Write-Host ""
Write-Host "  ===============================================" -ForegroundColor Cyan
Write-Host "   ProtegeWX - instalacao" -ForegroundColor Cyan
Write-Host "  ===============================================" -ForegroundColor Cyan

# ---------------------------------------------------------------- 1. administrador
Titulo "1. Verificando privilegios"
$identidade = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal  = New-Object Security.Principal.WindowsPrincipal($identidade)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Erro "Esta janela nao esta como administrador."
    Write-Host ""
    Write-Host "  Feche esta janela, clique no menu Iniciar, digite PowerShell," -ForegroundColor Yellow
    Write-Host "  clique com o botao DIREITO em 'Windows PowerShell' e escolha" -ForegroundColor Yellow
    Write-Host "  'Executar como administrador'. Depois rode este arquivo de novo." -ForegroundColor Yellow
    Write-Host ""
    Pausar
    exit 1
}
Ok "rodando como administrador"

# ---------------------------------------------------------------- 2. requisitos
Titulo "2. Verificando a maquina"

$servico = Get-Service hasplms -ErrorAction SilentlyContinue
if (-not $servico) {
    Erro "O servico do Sentinel (hasplms) nao existe nesta maquina."
    Write-Host "       Isso quer dizer que o driver do dongle nao esta instalado." -ForegroundColor Yellow
    Write-Host "       Instale primeiro o driver do dongle e rode este instalador de novo." -ForegroundColor Yellow
    Pausar
    exit 1
}
Ok "servico do Sentinel encontrado (status: $($servico.Status))"

if ($servico.Status -ne 'Running') {
    Aviso "o servico estava parado - iniciando"
    Start-Service hasplms
    Start-Sleep -Seconds 3
}

try {
    $r = Invoke-WebRequest -Uri 'http://127.0.0.1:1947/_int_/tab_dev.html' -UseBasicParsing -TimeoutSec 15
    $n = ([regex]::Matches($r.Content, '"typ":"(?!placeholder)[^"]+"')).Count
    if ($n -gt 0) { Ok "$n dongle(s) detectado(s)" }
    else { Aviso "o servico responde, mas nenhum dongle foi encontrado - confira se a chave esta conectada" }
} catch {
    Aviso "nao consegui consultar o gerenciador de licencas: $($_.Exception.Message)"
}

$produtos = @()
foreach ($raiz in @('C:\PC SOFT', "$env:ProgramFiles\PC SOFT", "${env:ProgramFiles(x86)}\PC SOFT")) {
    if (Test-Path $raiz) { $produtos += (Get-ChildItem $raiz -Directory -ErrorAction SilentlyContinue).Name }
}
if ($produtos) { Ok "produtos PC SOFT: $($produtos -join ', ')" }
else { Aviso "nenhum produto PC SOFT encontrado - o ProtegeWX ainda protege o dongle, mas nao havera atualizador a bloquear" }

# ---------------------------------------------------------------- 3. copia
Titulo "3. Instalando em $Destino"

$origem = Split-Path -Parent $PSScriptRoot
$exe    = Join-Path $origem 'protegewx.exe'
if (-not (Test-Path $exe)) {
    Erro "nao encontrei o protegewx.exe ao lado deste script"
    Pausar
    exit 1
}

New-Item -ItemType Directory -Force -Path $Destino | Out-Null
if ((Resolve-Path $origem).Path -ne (Resolve-Path $Destino).Path) {
    Copy-Item $exe (Join-Path $Destino 'protegewx.exe') -Force
    foreach ($d in @('README.md','LEIAME-COMUNIDADE.md','PROMPT-PARA-IA.md')) {
        if (Test-Path (Join-Path $origem $d)) { Copy-Item (Join-Path $origem $d) $Destino -Force }
    }
    New-Item -ItemType Directory -Force -Path (Join-Path $Destino 'scripts') | Out-Null
    Copy-Item $PSCommandPath (Join-Path $Destino 'scripts') -Force
    $atalhos = Join-Path $origem 'scripts\criar-atalhos.ps1'
    if (Test-Path $atalhos) { Copy-Item $atalhos (Join-Path $Destino 'scripts') -Force }
}
Ok "arquivos copiados"

$exeFinal = Join-Path $Destino 'protegewx.exe'

# ---------------------------------------------------------------- 4. baseline
Titulo "4. Registrando o estado atual das licencas"
Write-Host "     (isto so grava um retrato do que suas chaves contem hoje;" -ForegroundColor DarkGray
Write-Host "      nada e alterado no sistema)" -ForegroundColor DarkGray

& $exeFinal --aplicar D1,D2 | Out-Host
if ($LASTEXITCODE -eq 0) { Ok "retrato de referencia e copia do runtime guardados" }
else { Aviso "algum passo do registro falhou - veja a mensagem acima" }

# ---------------------------------------------------------------- 5. atalhos
Titulo "5. Criando atalhos na Area de Trabalho"
$criar = Join-Path $Destino 'scripts\criar-atalhos.ps1'
if (Test-Path $criar) {
    & $criar -Base $Destino | Out-Host
} else {
    Aviso "script de atalhos nao encontrado; abra o painel por $exeFinal"
}

# ---------------------------------------------------------------- 6. diagnostico
Titulo "6. Como esta a maquina agora"
& $exeFinal --status | Out-Host

Write-Host ""
Write-Host "  ===============================================" -ForegroundColor Green
Write-Host "   Instalacao concluida" -ForegroundColor Green
Write-Host "  ===============================================" -ForegroundColor Green
Write-Host ""
Write-Host "   Nenhuma protecao foi ligada ainda - isso e escolha sua." -ForegroundColor White
Write-Host ""
Write-Host "   Para ligar, abra o atalho 'ProtegeWX - Painel' na Area de" -ForegroundColor White
Write-Host "   Trabalho, marque o que quiser e clique em Aplicar." -ForegroundColor White
Write-Host ""
Write-Host "   IMPORTANTE: feche o WINDEV / WEBDEV antes de aplicar as acoes" -ForegroundColor Yellow
Write-Host "   do grupo A, porque elas reiniciam o servico do dongle." -ForegroundColor Yellow
Write-Host ""
Pausar
