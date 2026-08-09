<#
    Gera o código "Pix Copia e Cola" (BR Code) a partir da sua chave.

    Uso:
        .\gerar-pix.ps1 -Chave "sua-chave-aleatoria" -Nome "SEU NOME" -Cidade "SUA CIDADE"

    O payload é montado inteiramente nesta máquina, seguindo o padrão EMV®
    QRCPS do Banco Central. Nada é enviado para lugar nenhum.

    Para a IMAGEM do QR Code, o caminho mais confiável é o app do seu banco:
    peça um "QR Code para receber" e salve a imagem. O código sai assinado pelo
    banco e você confere o valor antes de publicar.
#>

[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string]$Chave,
    [Parameter(Mandatory)] [string]$Nome,
    [Parameter(Mandatory)] [string]$Cidade,
    [string]$Descricao = '***',
    [string]$Saida
)

$ErrorActionPreference = 'Stop'

# ---------------------------------------------------------------- utilidades

# Campo EMV: identificador (2) + tamanho (2) + valor
function Campo([string]$id, [string]$valor) {
    '{0}{1:D2}{2}' -f $id, $valor.Length, $valor
}

# O padrão aceita apenas ASCII sem acento nos campos de nome e cidade.
# A decomposição Unicode separa a letra do acento, e os acentos são descartados.
function Normalizar([string]$t, [int]$max) {
    $decomposto = $t.Normalize([Text.NormalizationForm]::FormD)
    $semAcento = ($decomposto.ToCharArray() | Where-Object {
        [Globalization.CharUnicodeInfo]::GetUnicodeCategory($_) -ne 'NonSpacingMark'
    }) -join ''
    $limpo = ($semAcento -replace '[^A-Za-z0-9 ]', '' -replace '\s+', ' ').ToUpper().Trim()
    if ($limpo.Length -gt $max) { $limpo = $limpo.Substring(0, $max).Trim() }
    return $limpo
}

# CRC16/CCITT-FALSE - polinômio 0x1021, valor inicial 0xFFFF.
# É o algoritmo definido pelo Banco Central para o campo 63 do BR Code.
function CRC16([string]$texto) {
    [uint16]$crc = 0xFFFF
    foreach ($b in [Text.Encoding]::ASCII.GetBytes($texto)) {
        $crc = $crc -bxor ([uint16]$b -shl 8)
        for ($i = 0; $i -lt 8; $i++) {
            if ($crc -band 0x8000) { $crc = (($crc -shl 1) -bxor 0x1021) -band 0xFFFF }
            else                   { $crc = ($crc -shl 1) -band 0xFFFF }
        }
    }
    return '{0:X4}' -f $crc
}

# ---------------------------------------------------------------- montagem

$nomeLimpo   = Normalizar $Nome 25
$cidadeLimpa = Normalizar $Cidade 15

if (-not $nomeLimpo)   { throw "o nome ficou vazio depois da limpeza" }
if (-not $cidadeLimpa) { throw "a cidade ficou vazia depois da limpeza" }

$conta = (Campo '00' 'br.gov.bcb.pix') + (Campo '01' $Chave)

$payload  = Campo '00' '01'                  # formato do payload
$payload += Campo '26' $conta                # dados da conta PIX
$payload += Campo '52' '0000'                # categoria do recebedor
$payload += Campo '53' '986'                 # moeda: BRL
$payload += Campo '58' 'BR'                  # país
$payload += Campo '59' $nomeLimpo            # nome do recebedor
$payload += Campo '60' $cidadeLimpa          # cidade do recebedor
$payload += Campo '62' (Campo '05' $Descricao)
$payload += '6304'                           # cabeçalho do CRC, entra no cálculo

$payload += CRC16 $payload

# ---------------------------------------------------------------- saída

Write-Host ""
Write-Host "  Pix Copia e Cola gerado" -ForegroundColor Green
Write-Host "  recebedor : $nomeLimpo"
Write-Host "  cidade    : $cidadeLimpa"
Write-Host ""
Write-Host $payload -ForegroundColor Cyan
Write-Host ""
Write-Host "  Confira no app do seu banco antes de publicar:" -ForegroundColor Yellow
Write-Host "  cole o codigo em 'Pix > Pagar > Copia e Cola' e veja se aparece" -ForegroundColor Yellow
Write-Host "  o SEU nome como recebedor. So publique depois de confirmar." -ForegroundColor Yellow
Write-Host ""

if ($Saida) {
    Set-Content -Path $Saida -Value $payload -Encoding ascii -NoNewline
    Write-Host "  gravado em $Saida"
    Write-Host ""
}

return $payload
