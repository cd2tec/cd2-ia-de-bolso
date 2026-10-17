# CD2 Harness - launcher para Windows (PowerShell 5.1+)
$ErrorActionPreference = 'Stop'
$Root   = Split-Path -Parent $PSScriptRoot
$Ctx    = if ($env:CD2_CTX) { $env:CD2_CTX } else { '8192' }
$PortFrom = if ($env:CD2_PORT) { [int]$env:CD2_PORT } else { 8080 }
$LogDir = Join-Path $env:TEMP 'cd2-harness'
New-Item -ItemType Directory -Force -Path $LogDir | Out-Null
$Log    = Join-Path $LogDir 'server.log'
$LogErr = Join-Path $LogDir 'server.err.log'
$script:Proc = $null
$script:CtlProc = $null

function Fail($msg) {
    Write-Host ""
    Write-Host "[ERRO] $msg" -ForegroundColor Red
    Read-Host "Pressione ENTER para sair"
    exit 1
}
function PortFree([int]$p) {
    try {
        $l = New-Object System.Net.Sockets.TcpListener([System.Net.IPAddress]::Loopback, $p)
        $l.Start(); $l.Stop(); return $true
    } catch { return $false }
}
function Stop-LeftoverLlamafile {
    if (-not $Exe -or -not (Test-Path -LiteralPath $Exe)) { return }
    $full = [System.IO.Path]::GetFullPath($Exe)
    $procs = @(Get-CimInstance Win32_Process -Filter "Name = 'llamafile.exe'" -ErrorAction SilentlyContinue)
    foreach ($leftover in $procs) {
        $cmdLine = $leftover.CommandLine
        $same = $false
        if ($leftover.ExecutablePath) {
            $same = [string]::Equals([System.IO.Path]::GetFullPath($leftover.ExecutablePath), $full, 'OrdinalIgnoreCase')
        }
        if (-not $same -and $cmdLine) {
            $same = $cmdLine.Replace('"', '').StartsWith($full, [System.StringComparison]::OrdinalIgnoreCase)
        }
        if ($same) { & taskkill.exe /PID $leftover.ProcessId /T /F 2>&1 | Out-Null }
    }
}
function Reset-ServerLogs {
    foreach ($logPath in @($Log, $LogErr)) {
        $opened = $false
        for ($attempt = 0; $attempt -lt 25 -and -not $opened; $attempt++) {
            try {
                Set-Content -Path $logPath -Value '' -ErrorAction Stop
                $opened = $true
            } catch {
                Start-Sleep -Milliseconds 200
            }
        }
        if (-not $opened) { Fail 'A IA anterior ainda esta fechando. Espere alguns segundos e abra de novo.' }
    }
}
function Stop-Server {
    if ($script:Proc -and -not $script:Proc.HasExited) {
        & taskkill.exe /PID $script:Proc.Id /T /F 2>&1 | Out-Null
    }
    Stop-LeftoverLlamafile
}
function Install-CloseHandler {
    if ($script:CloseHandler) { return }
    try {
    if (-not ('Cd2.ConsoleClose' -as [type])) {
        Add-Type -Namespace Cd2 -Name ConsoleClose -MemberDefinition @'
public delegate bool Handler(int ctrlType);
[System.Runtime.InteropServices.DllImport("kernel32.dll")]
public static extern bool SetConsoleCtrlHandler(Handler handler, bool add);
'@
    }
    $script:CloseHandler = [Cd2.ConsoleClose+Handler] {
        param($ctrlType)
        try { Stop-Server } catch { }
        return $false
    }
    [void][Cd2.ConsoleClose]::SetConsoleCtrlHandler($script:CloseHandler, $true)
    } catch { }
}
function Start-Server([string[]]$extra) {
    Stop-Server
    Reset-ServerLogs
    $srvArgs = @('-m', "`"$Model`"", '--server', '--host', '127.0.0.1', '--port', "$Port",
              '--jinja', '-c', $Ctx, '--path', "`"$(Join-Path $Root 'web')`"") + $extra
    $script:Proc = Start-Process -FilePath $Exe -ArgumentList $srvArgs -WindowStyle Hidden -PassThru `
        -RedirectStandardOutput $Log -RedirectStandardError $LogErr
}

# chave da sessao: 32 bytes aleatorios em hexadecimal (vai so para o cd2-ctl e para a janela)
function New-SessionKey {
    $bytes = New-Object byte[] 32
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($bytes) } finally { $rng.Dispose() }
    return -join ($bytes | ForEach-Object { $_.ToString('x2') })
}
# caminho entre aspas para a linha de comando; "E:\" terminaria em \" e quebraria o argumento
function Quote-Path([string]$p) { if ($p.EndsWith('\')) { $p = $p + '.' }; return '"' + $p + '"' }
# taskkill /T derruba o cd2-ctl e o toolbox que ele subiu
function Stop-Ctl {
    if ($script:CtlProc -and -not $script:CtlProc.HasExited) {
        & taskkill.exe /PID $script:CtlProc.Id /T /F 2>&1 | Out-Null
    }
}
function Start-Ctl {
    # roda direto do pen drive, como o llamafile.exe (no Windows nao falta bit de execucao)
    $src = Join-Path $Root 'bin\ctl\windows-amd64\cd2-ctl.exe'
    if (-not (Test-Path $src)) {
        Write-Host 'bin\ctl\windows-amd64 nao encontrado: banco de dados indisponivel neste pen drive' -ForegroundColor DarkYellow
        return $false
    }
    $ctlLog = Join-Path $LogDir 'ctl.log'; $ctlErr = Join-Path $LogDir 'ctl.err.log'
    $ctlArgs = @('--root', (Quote-Path $Root), '--cache', (Quote-Path $Cache), '--harness-port', "$Port", '--parent-pid', "$PID")
    # a chave vai so no ambiente do cd2-ctl: sai daqui assim que ele sobe, antes de abrir o navegador
    $env:CD2_CTL_KEY = $CtlKey
    try {
        $script:CtlProc = Start-Process -FilePath $src -ArgumentList $ctlArgs -WindowStyle Hidden -PassThru `
            -RedirectStandardOutput $ctlLog -RedirectStandardError $ctlErr
    } catch {
        Write-Host "[AVISO] Nao consegui iniciar o controle do banco: $($_.Exception.Message). Seguindo sem banco de dados." -ForegroundColor DarkYellow
        return $false
    } finally {
        Remove-Item Env:CD2_CTL_KEY -ErrorAction SilentlyContinue
    }
    for ($i = 0; $i -lt 10; $i++) {
        if ($script:CtlProc.HasExited) { break }
        # 401 = esta no ar (o teste vai sem a chave)
        try { $null = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$CtlPort/status" -TimeoutSec 2; return $true }
        catch { if ($_.Exception.Response -and [int]$_.Exception.Response.StatusCode -eq 401) { return $true } }
        Start-Sleep -Seconds 1
    }
    Stop-Ctl
    Write-Host "[AVISO] O controle do banco (cd2-ctl) nao subiu. Ultimas linhas de ${ctlErr}:" -ForegroundColor DarkYellow
    if (Test-Path $ctlErr) { Get-Content $ctlErr -Tail 5 | ForEach-Object { Write-Host "   $_" } }
    Write-Host 'Seguindo sem banco de dados (Demonstracao e Archeo continuam).' -ForegroundColor DarkYellow
    return $false
}

Clear-Host
Write-Host ""
Write-Host "  +-----------------------------+" -ForegroundColor DarkYellow
Write-Host "  |   CD2 Tech  IA de Bolso     |" -ForegroundColor Yellow
Write-Host "  +-----------------------------+" -ForegroundColor DarkYellow
Write-Host "  IA local, sem nuvem." -ForegroundColor Gray
Write-Host ""

$Exe = Join-Path $Root 'bin\llamafile.exe'
if (-not (Test-Path $Exe)) { Fail "bin\llamafile.exe nao encontrado. Rode a preparacao do pen drive (core\prepare-windows.bat)." }
Install-CloseHandler
Stop-LeftoverLlamafile
$ModelFile = Get-ChildItem -Path (Join-Path $Root 'models') -Filter *.gguf -ErrorAction SilentlyContinue | Select-Object -First 1
if (-not $ModelFile) { Fail "Nenhum modelo .gguf em models\. Rode a preparacao do pen drive." }
$Model = $ModelFile.FullName

# trio de portas livre: P (harness), P+1000 (toolbox) e P+3000 (cd2-ctl), avancando juntas
$Port = $PortFrom
while ((-not (PortFree $Port) -or -not (PortFree ($Port + 1000)) -or -not (PortFree ($Port + 3000))) -and $Port -lt ($PortFrom + 20)) { $Port++ }
$Url = "http://127.0.0.1:$Port"
$CtlPort = $Port + 3000
$Cache = Join-Path $env:LOCALAPPDATA 'cd2-harness'
New-Item -ItemType Directory -Force -Path $Cache | Out-Null
$CtlKey = New-SessionKey

try {
    # robo de carregamento: le o modelo em pedacos (progresso real) e anima enquanto o servidor sobe
    . (Join-Path $PSScriptRoot 'robo-windows.ps1')
    Robo-Init (Join-Path $PSScriptRoot 'robo.txt')
    # Para quem usa, o modelo e so "o modelo de IA": o nome do arquivo nao aparece no terminal.
    Write-Host "Carregando o modelo de IA..." -ForegroundColor Gray
    if (-not (Robo-Ler $Model)) { Write-Host "[AVISO] Nao consegui ler o modelo antes de subir o servidor; seguindo assim mesmo." -ForegroundColor DarkYellow }
    Start-Server @('-ngl', '999')
    if (-not (Robo-Esperar $Url $script:Proc 240)) {
        Robo-Aviso "Aceleracao por GPU indisponivel, tentando so com CPU..."
        Start-Server @('--gpu', 'disable', '-ngl', '0')
        if (-not (Robo-Esperar $Url $script:Proc 240)) {
            Robo-Fim 'erro' "O modelo nao subiu."
            Fail "O servidor nao iniciou. Logs em $LogDir"
        }
    }
    Robo-Fim 'ok' "Pronto: $Url"
    Write-Host ""

    if (Start-Ctl) { Write-Host "Controle do banco em 127.0.0.1:$CtlPort. Configure o banco pela tela Configuracoes." -ForegroundColor Gray }
    # ?v=<versao> muda o endereco a cada versao: o navegador nao reaproveita o front antigo do cache.
    $Ver = ''; try { $Ver = (Get-Content -Raw (Join-Path $Root 'VERSION')).Trim() } catch { }
    $AppUrl = "$Url/?v=$Ver#k=$CtlKey"

    # abre como janela de aplicativo (Edge vem com o Windows; Chrome como alternativa)
    $cands = @(
        "${env:ProgramFiles(x86)}\Microsoft\Edge\Application\msedge.exe",
        "$env:ProgramFiles\Microsoft\Edge\Application\msedge.exe",
        "$env:ProgramFiles\Google\Chrome\Application\chrome.exe",
        "${env:ProgramFiles(x86)}\Google\Chrome\Application\chrome.exe",
        "$env:LOCALAPPDATA\Google\Chrome\Application\chrome.exe"
    )
    $browser = $cands | Where-Object { Test-Path $_ } | Select-Object -First 1
    # maximizada: o campo de pergunta cabe em tela pequena
    if ($browser) { Start-Process -FilePath $browser -ArgumentList @("--app=$AppUrl", '--start-maximized') }
    else { Start-Process $AppUrl }

    Write-Host ""
    Write-Host "A IA de Bolso esta rodando. Para encerrar, pressione ENTER nesta janela." -ForegroundColor Gray
    while (-not $script:Proc.HasExited) {
        if ([Console]::KeyAvailable) { $k = [Console]::ReadKey($true); if ($k.Key -eq 'Enter') { break } }
        Start-Sleep -Milliseconds 400
    }
}
finally {
    if (Get-Command Robo-Restaura -ErrorAction SilentlyContinue) { Robo-Restaura }
    try { Stop-Ctl } catch { }
    try { Stop-Server } catch { }
}
