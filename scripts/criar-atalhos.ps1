<#
    ProtegeWX - cria os atalhos de uso diario

      "ProtegeWX - Painel"   abre o painel no navegador (pede elevacao sozinho)
      "ProtegeWX - Conferir" roda a conferencia das licencas e mostra o resultado

    Os dois vao para a Area de Trabalho. Nada aqui altera configuracao do
    sistema: sao apenas atalhos.
#>

[CmdletBinding()]
param(
    [string]$Base = (Split-Path -Parent $PSScriptRoot)
)

$ErrorActionPreference = 'Stop'

$exe      = Join-Path $Base 'protegewx.exe'
$desktop  = [Environment]::GetFolderPath('Desktop')
$shell    = New-Object -ComObject WScript.Shell

if (-not (Test-Path $exe)) { throw "nao encontrei $exe - compile antes com scripts\build.ps1" }

function Novo-Atalho($nome, $argumentos, $descricao, $iconIndex) {
    $caminho = Join-Path $desktop "$nome.lnk"
    $a = $shell.CreateShortcut($caminho)
    $a.TargetPath       = $exe
    $a.Arguments        = $argumentos
    $a.WorkingDirectory = $Base
    $a.Description      = $descricao
    $a.IconLocation     = "$env:SystemRoot\System32\shell32.dll,$iconIndex"
    $a.Save()

    # marca o atalho para sempre abrir elevado: sem isso o painel nao consegue
    # ler o firewall nem gravar em Program Files
    $bytes = [IO.File]::ReadAllBytes($caminho)
    $bytes[0x15] = $bytes[0x15] -bor 0x20
    [IO.File]::WriteAllBytes($caminho, $bytes)

    Write-Host "[OK] $caminho"
    return $caminho
}

Write-Host ""
Write-Host "Criando atalhos na Area de Trabalho" -ForegroundColor Cyan
Write-Host ""

Novo-Atalho 'ProtegeWX - Painel' '' `
    'Abre o painel do ProtegeWX e mostra em que pe esta a protecao' 48 | Out-Null

Novo-Atalho 'ProtegeWX - Conferir licencas' '--check' `
    'Compara as licencas dos dongles com o registro de referencia' 23 | Out-Null

Write-Host ""
Write-Host "Pronto. Os dois atalhos ja abrem como administrador." -ForegroundColor Green
Write-Host ""
