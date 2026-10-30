# Versao Windows do tests/eval/rodar.sh (chamada pelo rodar.bat). Mesmas variaveis CD2_EVAL_*:
# sobe o banco de avaliacao, o toolbox e o llamafile (so CPU por padrao), roda a avaliacao e derruba tudo.
#   tests\eval\rodar.bat                          modelo padrao em models\, so CPU
#   set CD2_EVAL_GPU=1 & tests\eval\rodar.bat     usa a GPU (mais rapido, nao vale para o portao)
#   set CD2_EVAL_MODEL=C:\caminho\outro.gguf
#   set CD2_EVAL_BANCO=erp2                       segundo banco (ingles, sem prefixo); CD2_EVAL_DESCRICAO=0 pula a fila do modelo
#   set CD2_EVAL_PORT=18080                       muda a porta do llamafile; o toolbox usa essa porta + 1000
$ErrorActionPreference = 'Stop'
$Here = $PSScriptRoot
$Root = (Resolve-Path (Join-Path $Here '..\..')).Path

$Model = $env:CD2_EVAL_MODEL
if (-not $Model) { $Model = Get-ChildItem (Join-Path $Root 'models\*.gguf') -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty FullName }
if (-not $Model) { Write-Host 'Sem modelo em models\'; exit 1 }
$Ngl = if ($env:CD2_EVAL_GPU -eq '1') { @() } else { @('-ngl', '0') }
$Port = if ($env:CD2_EVAL_PORT) { [int]$env:CD2_EVAL_PORT } else { 18080 }
$TbPort = $Port + 1000
$env:CD2_EVAL_MODEL_URL = "http://127.0.0.1:$Port/v1/chat/completions"
$env:CD2_EVAL_TOOLBOX_URL = "http://127.0.0.1:$TbPort/mcp"
$Log = Join-Path $env:TEMP 'cd2-eval'; New-Item -ItemType Directory -Force $Log | Out-Null
$Compose = @('compose', '-p', 'cd2eval', '-f', (Join-Path $Here 'docker-compose.yml'))
$Banco = if ($env:CD2_EVAL_BANCO) { $env:CD2_EVAL_BANCO } else { 'erp' }
$EnvFile = Join-Path $Here "$Banco.env"
if (-not (Test-Path $EnvFile)) { Write-Host "Banco de avaliacao desconhecido: $Banco (use erp ou erp2)"; exit 1 }
$env:CD2_EVAL_PERGUNTAS = if ($Banco -eq 'erp') { Join-Path $Here 'perguntas.json' } else { Join-Path $Here "perguntas-$Banco.json" }

$Toolbox = Join-Path $Root 'bin\toolbox\windows-amd64\toolbox.exe'
$Llamafile = Join-Path $Root 'bin\llamafile.exe'
foreach ($b in $Toolbox, $Llamafile) { if (-not (Test-Path $b)) { Write-Host "$b nao encontrado. Rode core\prepare-windows.bat."; exit 1 } }

# Start-Process junta os argumentos num texto so: caminhos com espaco vao entre aspas.
function Q([string]$s) { if ($s -match '\s') { '"' + $s + '"' } else { $s } }
function Responde([string]$url, [bool]$exigeOk) {
    try { Invoke-WebRequest $url -UseBasicParsing -TimeoutSec 2 | Out-Null; return $true }
    catch [System.Net.WebException] { return (-not $exigeOk) -and ($null -ne $_.Exception.Response) }
    catch { return $false }
}

$tb = $null; $ll = $null; $code = 1
try {
    & docker @Compose up -d --wait
    if ($LASTEXITCODE -ne 0) { exit 1 }
    foreach ($linha in Get-Content $EnvFile) {
        if ($linha -match '^\s*([A-Za-z_][A-Za-z0-9_]*)=(.*)$') { Set-Item "env:$($Matches[1])" $Matches[2] }
    }
    $tb = Start-Process $Toolbox -NoNewWindow -PassThru `
        -RedirectStandardOutput (Join-Path $Log 'toolbox.log') -RedirectStandardError (Join-Path $Log 'toolbox.err.log') `
        -ArgumentList (@('--config', (Q (Join-Path $Root 'core\toolbox\postgres.yaml')), '--address', '127.0.0.1', '--port', $TbPort,
            '--allowed-origins', "http://127.0.0.1:$Port", '--disable-reload') -join ' ')
    $ll = Start-Process $Llamafile -NoNewWindow -PassThru `
        -RedirectStandardOutput (Join-Path $Log 'llama.log') -RedirectStandardError (Join-Path $Log 'llama.err.log') `
        -ArgumentList ((@('-m', (Q $Model), '--server', '--host', '127.0.0.1', '--port', $Port, '--jinja', '-c', '8192') + $Ngl) -join ' ')
    $i = 0
    while (-not (Responde "http://127.0.0.1:$Port/health" $true)) {
        $i++
        if ($ll.HasExited) { Write-Host "llamafile morreu (log em $Log\llama.log)"; exit 1 }
        if ($i -gt 180) { Write-Host "llamafile nao subiu (log em $Log\llama.log)"; exit 1 }
        Start-Sleep 1
    }
    $i = 0
    while (-not (Responde "http://127.0.0.1:$TbPort/" $false)) {
        $i++
        if ($i -gt 30) { Write-Host "toolbox nao subiu (log em $Log\toolbox.log)"; exit 1 }
        Start-Sleep 1
    }
    Write-Host "Modelo: $(Split-Path -Leaf $Model)  $(if ($Ngl) { $Ngl -join ' ' } else { 'GPU' })"
    & node (Join-Path $Here 'run-eval.mjs')
    $code = $LASTEXITCODE
} finally {
    # Com Stop, o stderr redirecionado de um comando nativo (o progresso do docker) viraria erro aqui.
    $ErrorActionPreference = 'Continue'
    foreach ($p in $tb, $ll) { if ($p -and -not $p.HasExited) { & taskkill /T /F /PID $p.Id 2>$null | Out-Null } }
    & docker @Compose down 2>$null | Out-Null
}
exit $code
