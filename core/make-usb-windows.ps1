# CD2 Harness - grava um pen drive pronto para uso (Windows)
# Uso: duplo clique em GRAVAR-PENDRIVE-Windows.bat (ele pede administrador sozinho)
#
# O que faz, nesta ordem:
#   1. monta uma copia local do harness e baixa motor + modelo (ficam em cache em
#      %LOCALAPPDATA%\cd2-harness; o segundo pen drive nao baixa nada de novo)
#   2. APAGA o pen drive escolhido e formata em exFAT (MBR), nome CD2
#   3. copia os arquivos para a raiz do pen drive (ele acabou de ser formatado,
#      entao nao precisa de subpasta)
#   4. confere os checksums direto no pen drive, ejeta e, ao plugar de novo,
#      abre o INICIAR do pen drive (a IA sobe lendo o modelo de la)
#
# Variaveis opcionais: CD2_USB_DISK (numero do disco) pula a pergunta de qual disco;
# CD2_USB_YES=1 pula a confirmacao "APAGAR" (para gravar em serie).
$ErrorActionPreference = 'Stop'
$Root  = Split-Path -Parent $PSScriptRoot
$Label = if ($env:CD2_USB_LABEL) { $env:CD2_USB_LABEL } else { 'CD2' }
$Cache = Join-Path $env:LOCALAPPDATA 'cd2-harness\usb'
$Stage = Join-Path $Cache 'CD2-HARNESS'   # copia local no cache (nome antigo, para nao baixar tudo de novo); no pen drive vai para a raiz
$MinBytes = 4000000000

function Pause-Exit($code) { Write-Host ''; Read-Host 'Pressione ENTER para fechar' | Out-Null; exit $code }
function Die($msg) { Write-Host "`n[ERRO] $msg" -ForegroundColor Red; Pause-Exit 1 }
function Find-Cd2Starter {
    $usb = @(Get-Disk -ErrorAction SilentlyContinue | Where-Object { $_.BusType -in @('USB', 'SD', 'MMC') })
    foreach ($d in $usb) {
        $parts = @(Get-Partition -DiskNumber $d.Number -ErrorAction SilentlyContinue | Where-Object { $_.DriveLetter })
        foreach ($p in $parts) {
            $volume = "$($p.DriveLetter):\"
            $motor = Join-Path $volume 'bin\llamafile.exe'
            if (-not (Test-Path -LiteralPath $motor)) { continue }
            # Com lancador, ele e o que abre; sem, o .bat (em core\ ou, no plano B, na raiz).
            foreach ($cand in @('IA de Bolso.exe', 'core\INICIAR-CD2-Windows.bat', 'INICIAR-CD2-Windows.bat')) {
                $achado = Join-Path $volume $cand
                if (Test-Path -LiteralPath $achado -PathType Leaf) { return $achado }
            }
        }
    }
    return $null
}

Write-Host ''
Write-Host '  +-----------------------------+'
Write-Host '  |   CD2 Tech  IA de Bolso     |'
Write-Host '  +-----------------------------+'
Write-Host '  Gravar pen drive'
Write-Host ''

$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) { Die 'Formatar exige administrador. Use o GRAVAR-PENDRIVE-Windows.bat, que pede a permissao sozinho.' }

# --- 0. escolher o pen drive --------------------------------------------------
$disks = @(Get-Disk | Where-Object { $_.BusType -in @('USB', 'SD', 'MMC') } | Sort-Object Number)
if ($disks.Count -eq 0) { Die 'Nenhum pen drive encontrado. Plugue o pen drive e rode de novo.' }
Write-Host 'Pen drives encontrados:'
foreach ($d in $disks) { Write-Host ('  [{0}]  {1,6:N1} GB  {2}' -f $d.Number, ($d.Size / 1GB), $d.FriendlyName) }
Write-Host ''
$default = if ($disks.Count -eq 1) { "$($disks[0].Number)" } else { '' }
if ($env:CD2_USB_DISK) { $sel = $env:CD2_USB_DISK }
else {
    $prompt = if ($default) { "Qual deles? Digite o numero [$default]" } else { 'Qual deles? Digite o numero' }
    $sel = Read-Host $prompt
    if (-not $sel) { $sel = $default }
}
if (-not $sel) { Write-Host 'Cancelado. Nada foi apagado.'; Pause-Exit 0 }
$disk = $disks | Where-Object { "$($_.Number)" -eq "$sel" } | Select-Object -First 1
if (-not $disk) { Die "'$sel' nao esta na lista de pen drives acima. Nada foi apagado." }
if ($disk.Size -lt $MinBytes) { Die 'Esse pen drive tem menos de 4 GB; a IA de Bolso precisa de uns 3 GB livres.' }

if ($env:CD2_USB_YES -ne '1') {
    Write-Host ''
    Write-Host "ATENCAO: TUDO que esta no disco $($disk.Number) ($($disk.FriendlyName)) sera apagado." -ForegroundColor Yellow
    $ok = Read-Host 'Digite APAGAR para continuar (qualquer outra coisa cancela)'
    if ($ok -ne 'APAGAR') { Write-Host 'Cancelado. Nada foi apagado.'; Pause-Exit 0 }
}

# --- 1. copia local + downloads (cache) ---------------------------------------
Write-Host ''
Write-Host "== 1/4 Montando a copia local em $Stage"
New-Item -ItemType Directory -Force -Path $Stage | Out-Null
Get-ChildItem -Force $Stage | Where-Object { $_.Name -notin @('bin', 'models', 'CHECKSUMS.sha256') } | Remove-Item -Recurse -Force
$items = @('INICIAR-CD2-Windows.bat', 'INICIAR-CD2-Mac.command', 'ia-de-bolso.sh', 'README.md', 'LICENSE', 'VERSION', 'web', 'core')
foreach ($i in $items) {
    $src = Join-Path $Root $i
    if (-not (Test-Path $src)) { Die "Nao achei '$i' em $Root. Rode de dentro da pasta do repositorio." }
    Copy-Item -Recurse -Force -LiteralPath $src -Destination $Stage
}
# Lancadores da raiz: entram so se existirem na copia.
foreach ($i in @('IA de Bolso.exe', 'IA de Bolso.app')) {
    $src = Join-Path $Root $i
    if (Test-Path -LiteralPath $src) { Copy-Item -Recurse -Force -LiteralPath $src -Destination $Stage }
}
Remove-Item -Force -ErrorAction SilentlyContinue (Join-Path $Stage 'core\make-usb-unix.sh'), (Join-Path $Stage 'core\make-usb-windows.ps1')

# Com o lancador na copia, o INICIAR do sistema (.command ou .bat) vai para core\; sem ele,
# fica na raiz (plano B). Mesma regra do core/lancadores.sh e do instalador.
New-Item -ItemType Directory -Force -Path (Join-Path $Stage 'core') | Out-Null
foreach ($par in @(@('IA de Bolso.app', 'INICIAR-CD2-Mac.command'), @('IA de Bolso.exe', 'INICIAR-CD2-Windows.bat'))) {
    $iniciar = Join-Path $Stage $par[1]
    if ((Test-Path -LiteralPath (Join-Path $Stage $par[0])) -and (Test-Path -LiteralPath $iniciar)) {
        Move-Item -LiteralPath $iniciar -Destination (Join-Path $Stage ('core\' + $par[1])) -Force
    }
}

Write-Host ''
Write-Host 'Baixando motor e modelo (uns 3,9 GB na primeira vez: motor, modelo, toolbox e cd2-ctl; depois fica em cache)...'
& powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $Stage 'core\prepare-windows.ps1')
if ($LASTEXITCODE -ne 0) { Die 'O preparo (download) falhou. Rode de novo: ele retoma de onde parou.' }

# --- 2. formatar ----------------------------------------------------------------
Write-Host ''
Write-Host "== 2/4 Formatando o disco $($disk.Number) em exFAT ($Label)"
Set-Disk -Number $disk.Number -IsOffline $false -ErrorAction SilentlyContinue
Set-Disk -Number $disk.Number -IsReadOnly $false -ErrorAction SilentlyContinue
try {
    Clear-Disk -Number $disk.Number -RemoveData -RemoveOEM -Confirm:$false
} catch {
    Die "Nao consegui apagar o disco: $($_.Exception.Message)"
}
# Clear-Disk apaga as particoes, mas o disco pode continuar inicializado em MBR.
# Initialize-Disk so aceita disco cru (RAW); num pen drive ja usado o Windows recusa.
$disk = Get-Disk -Number $disk.Number
if ($disk.PartitionStyle -eq 'RAW') {
    try { Initialize-Disk -Number $disk.Number -PartitionStyle MBR }
    catch { Die "Nao consegui iniciar o disco: $($_.Exception.Message)" }
} elseif ($disk.PartitionStyle -ne 'MBR') {
    Die "O disco ficou em $($disk.PartitionStyle). O pen drive precisa estar em MBR para exFAT."
}
$part = New-Partition -DiskNumber $disk.Number -UseMaximumSize -IsActive -AssignDriveLetter
Format-Volume -Partition $part -FileSystem exFAT -NewFileSystemLabel $Label -Confirm:$false | Out-Null
$letter = $null
for ($i = 0; $i -lt 20 -and -not $letter; $i++) {
    Start-Sleep -Seconds 1
    $letter = (Get-Partition -DiskNumber $disk.Number | Where-Object { $_.DriveLetter } | Select-Object -First 1).DriveLetter
}
if (-not $letter) { Die 'Formatei, mas a particao nao recebeu letra. Atribua uma letra no Gerenciamento de Disco e rode de novo.' }
$Dest = "${letter}:\"

# --- 3. copiar ------------------------------------------------------------------
Write-Host ''
Write-Host "== 3/4 Copiando para a raiz de $Dest (uns 3 GB; pode levar alguns minutos)"
& robocopy $Stage $Dest /E /R:2 /W:2 /NFL /NDL /NJH /NJS /NP | Out-Null
if ($LASTEXITCODE -ge 8) { Die "A copia falhou (robocopy codigo $LASTEXITCODE)." }

# --- 4. conferir e ejetar -------------------------------------------------------
Write-Host ''
Write-Host '== 4/4 Conferindo os checksums direto no pen drive'
$sums = Join-Path $Dest 'CHECKSUMS.sha256'
if (-not (Test-Path $sums)) { Die 'CHECKSUMS.sha256 nao foi copiado.' }
foreach ($line in Get-Content $sums) {
    if ($line -match '^([0-9a-fA-F]{64})\s+\*?(.+)$') {
        $want = $matches[1].ToLower(); $rel = $matches[2].Trim() -replace '/', '\'
        $got = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $Dest $rel)).Hash.ToLower()
        if ($got -ne $want) { Die "Checksum diferente em $rel. Nao use este pen drive; grave de novo." }
        Write-Host "  OK  $rel"
    }
}
$keep = Join-Path $Cache ('CHECKSUMS-{0}.sha256' -f (Get-Date -Format 'yyyyMMdd-HHmmss'))
Copy-Item -Force $sums $keep
Write-Host ''
Write-Host 'Conteudo gravado:'
Get-ChildItem (Join-Path $Dest 'bin'), (Join-Path $Dest 'models') | ForEach-Object { Write-Host ('  {0,10:N0} KB  {1}' -f ($_.Length / 1KB), $_.Name) }
Get-ChildItem -Recurse (Join-Path $Dest 'bin\toolbox') -File | ForEach-Object { Write-Host ('  {0,10:N0} KB  toolbox\{1}\{2}' -f ($_.Length / 1KB), $_.Directory.Name, $_.Name) }
Get-ChildItem -Recurse (Join-Path $Dest 'bin\ctl') -File | ForEach-Object { Write-Host ('  {0,10:N0} KB  ctl\{1}\{2}' -f ($_.Length / 1KB), $_.Directory.Name, $_.Name) }
try { (New-Object -ComObject Shell.Application).NameSpace(17).ParseName("${letter}:").InvokeVerb('Eject'); Write-Host 'Ejetado.' }
catch { Write-Host 'Nao consegui ejetar sozinha; use "Remover hardware com seguranca" antes de tirar.' }
Write-Host ''
Write-Host "Copia do checksum guardada em: $keep"
Write-Host 'Plugue o pen drive de novo. Quando o Windows reconhecer, a IA abre do pen drive e esta janela fecha.'
$starter = $null
while (-not $starter) {
    Start-Sleep -Seconds 2
    $starter = Find-Cd2Starter
}
Write-Host "Abrindo $starter"
Start-Process -FilePath $starter -WorkingDirectory (Split-Path -Parent $starter)
exit 0
