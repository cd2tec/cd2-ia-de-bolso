# CD2 Harness - prepara o pen drive (rode UMA vez, com internet, no Windows)
# Baixa o llamafile e o modelo para dentro desta pasta. Retoma downloads interrompidos
# e pula o que ja estiver completo, entao pode rodar de novo sem medo.
$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$Root = Split-Path -Parent $PSScriptRoot
$ModelUrl  = if ($env:CD2_MODEL_URL)  { $env:CD2_MODEL_URL }  else { 'https://huggingface.co/unsloth/Qwen3-4B-Instruct-2507-GGUF/resolve/main/Qwen3-4B-Instruct-2507-Q4_K_M.gguf' }
$ModelFile = if ($env:CD2_MODEL_FILE) { $env:CD2_MODEL_FILE } else { 'Qwen3-4B-Instruct-2507-Q4_K_M.gguf' }
# SHA-256 do modelo padrao. Com CD2_MODEL_URL proprio, so confere se CD2_MODEL_SHA256 vier junto.
$ModelSha  = if ($env:CD2_MODEL_SHA256) { $env:CD2_MODEL_SHA256.ToLower() } elseif ($env:CD2_MODEL_URL) { '' } else { '3605803b982cb64aead44f6c1b2ae36e3acdb41d8e46c8a94c6533bc4c67e597' }
# Motor (llamafile): versao e SHA-256 fixos, os mesmos que o manifesto do instalador publica.
# Com CD2_LLAMAFILE_URL proprio, so confere se CD2_LLAMAFILE_SHA256 vier junto.
$LlamafileUrl = if ($env:CD2_LLAMAFILE_URL) { $env:CD2_LLAMAFILE_URL } else { 'https://github.com/mozilla-ai/llamafile/releases/download/0.10.6/llamafile-0.10.6' }
$LlamafileSha = if ($env:CD2_LLAMAFILE_SHA256) { $env:CD2_LLAMAFILE_SHA256.ToLower() } elseif ($env:CD2_LLAMAFILE_URL) { '' } else { 'd579f61dcd3a306f518e6d90e599d77793ed5f09543023d09c96ad35fcfa63f0' }
# MCP Toolbox for Databases: versao fixa (pen drive reprodutivel), um binario por sistema.
$ToolboxVersion   = if ($env:CD2_TOOLBOX_VERSION)   { $env:CD2_TOOLBOX_VERSION }   else { '1.14.0' }
$ToolboxPlatforms = if ($env:CD2_TOOLBOX_PLATFORMS) { $env:CD2_TOOLBOX_PLATFORMS -split ' ' } else { @('windows-amd64', 'darwin-arm64', 'darwin-amd64', 'linux-amd64') }
$ToolboxBase = "https://storage.googleapis.com/mcp-toolbox-for-databases/v$ToolboxVersion"
# cd2-ctl (controle do banco pela tela): versao fixa, baixada do release deste repositorio.
# CD2_CTL_LOCAL=1 pula o download e usa o binario que ja estiver em bin\ctl (antes do 1o release).
$CtlVersion = if ($env:CD2_CTL_VERSION) { $env:CD2_CTL_VERSION } else { '0.8.0' }
$CtlBase = "https://github.com/cd2tec/cd2-ia-de-bolso/releases/download/ctl-v$CtlVersion"

New-Item -ItemType Directory -Force -Path (Join-Path $Root 'bin'), (Join-Path $Root 'models') | Out-Null
$curl = Join-Path $env:SystemRoot 'System32\curl.exe'
if (-not (Test-Path $curl)) { throw 'curl.exe nao encontrado (Windows 10 1803+ ou 11 necessario).' }

# Tamanho do arquivo no servidor (segue redirecionamentos). $null se nao der para saber.
function Get-RemoteSize($url) {
    try {
        $r = Invoke-WebRequest -Uri $url -Method Head -MaximumRedirection 10 -UseBasicParsing -Headers @{ 'User-Agent' = 'cd2-harness' }
        $len = $r.Headers['Content-Length']
        if ($len) { return [int64]($len | Select-Object -Last 1) }
    } catch { }
    return $null
}

# Fetch URL DESTINO: pula se ja estiver completo, retoma se estiver pela metade.
function Fetch($url, $dest) {
    if ((Test-Path $dest) -and (Get-Item $dest).Length -gt 0) {
        $have = (Get-Item $dest).Length
        $want = Get-RemoteSize $url
        if ($want -and $want -eq $have) { Write-Host "Ja baixado, pulando: $([IO.Path]::GetFileName($dest)) ($have bytes)"; return }
        & $curl -L --fail -C - -o $dest $url
        if ($LASTEXITCODE -ne 0) { & $curl -L --fail -o $dest $url }
    } else {
        & $curl -L --fail -C - -o $dest $url
    }
    if ($LASTEXITCODE -ne 0) { throw "Falha ao baixar $url" }
}

Write-Host '== Motor (llamafile)'
# O motor nunca e emendado em arquivo de outra versao (o mesmo que o baixar_motor do prepare-unix.sh):
# o download vai para um pedaco com nome tirado da URL, que so a mesma origem retoma, e so vira
# bin\llamafile.exe depois de conferir o SHA.
$exe = Join-Path $Root 'bin\llamafile.exe'
$bin = Join-Path $Root 'bin\llamafile'
if ((Test-Path $exe) -and (Get-Item $exe).Length -gt 0) {
    $ok = if ($LlamafileSha) { (Get-FileHash -Algorithm SHA256 $exe).Hash.ToLower() -eq $LlamafileSha } else { (Get-Item $exe).Length -eq (Get-RemoteSize $LlamafileUrl) }
    if ($ok) { Write-Host 'Ja conferido, pulando: bin\llamafile.exe' }
    else { Write-Host 'O bin\llamafile.exe e de outra versao; baixando o motor fixo.'; Remove-Item -Force $exe }
}
if (-not (Test-Path $exe)) {
    $sha = [Security.Cryptography.SHA256]::Create()
    $chave = (-join ($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($LlamafileUrl)) | ForEach-Object { $_.ToString('x2') })).Substring(0, 12)
    $pedaco = Join-Path $Root "bin\llamafile.part-$chave"
    Get-ChildItem (Join-Path $Root 'bin') -Filter 'llamafile.part*' | Where-Object { $_.FullName -ne $pedaco } | Remove-Item -Force
    Write-Host "Origem: $LlamafileUrl"
    # retoma o pedaco; se o servidor recusar o retomo, baixa inteiro
    & $curl -L --fail -C - -o $pedaco $LlamafileUrl
    if ($LASTEXITCODE -ne 0) { Remove-Item -Force -ErrorAction SilentlyContinue $pedaco; & $curl -L --fail -o $pedaco $LlamafileUrl }
    if ($LASTEXITCODE -ne 0) { Remove-Item -Force -ErrorAction SilentlyContinue $pedaco; throw 'Falha ao baixar o motor. Rode de novo.' }
    if ($LlamafileSha) {
        $got = (Get-FileHash -Algorithm SHA256 $pedaco).Hash.ToLower()
        if ($got -ne $LlamafileSha) { Remove-Item -Force $pedaco; throw "SHA-256 do motor nao confere ($got). Rode de novo." }
        Write-Host 'SHA-256 do motor conferido.'
    }
    Move-Item -Force $pedaco $exe
}
# mesma binaria (APE) serve para Mac/Linux
if (-not (Test-Path $bin) -or (Get-FileHash -Algorithm SHA256 $bin).Hash -ne (Get-FileHash -Algorithm SHA256 $exe).Hash) { Copy-Item -Force $exe $bin }

Write-Host '== Modelo'
Write-Host "Origem: $ModelUrl"
$model = Join-Path $Root "models\$ModelFile"
Fetch $ModelUrl $model
if ((Get-Item $model).Length -lt 1GB) { throw "Modelo com tamanho suspeito. Apague $model e rode de novo." }
if ($ModelSha) {
    $got = (Get-FileHash -Algorithm SHA256 $model).Hash.ToLower()
    if ($got -ne $ModelSha) { throw "SHA-256 do modelo nao confere ($got). Apague $model e rode de novo." }
    Write-Host 'SHA-256 do modelo conferido.'
}
# O launcher usa o primeiro .gguf que encontrar: fica so o modelo atual em models\.
Get-ChildItem (Join-Path $Root 'models') -Filter *.gguf | Where-Object { $_.Name -ne $ModelFile } | ForEach-Object {
    Write-Host "Removendo modelo antigo: $($_.Name)"; Remove-Item -Force $_.FullName
}

Write-Host ''
Write-Host "== Toolbox (MCP Toolbox for Databases v$ToolboxVersion)"
$toolboxFiles = @()
foreach ($plat in $ToolboxPlatforms) {
    $os, $arch = $plat -split '-', 2
    $ext = if ($os -eq 'windows') { '.exe' } else { '' }
    $dir = Join-Path $Root "bin\toolbox\$plat"
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    $dest = Join-Path $dir "toolbox$ext"
    Write-Host "Origem: $ToolboxBase/$os/$arch/toolbox$ext"
    Fetch "$ToolboxBase/$os/$arch/toolbox$ext" $dest
    if ((Get-Item $dest).Length -lt 100MB) { throw "Toolbox $plat com tamanho suspeito. Apague $dest e rode de novo." }
    $toolboxFiles += @{ Path = $dest; Rel = "bin/toolbox/$plat/toolbox$ext" }
}

Write-Host ''
Write-Host "== Controle do banco (cd2-ctl v$CtlVersion)"
$ctlSums = $null
if ($env:CD2_CTL_LOCAL -eq '1') {
    Write-Host 'CD2_CTL_LOCAL=1: usando o cd2-ctl que ja esta em bin\ctl.'
} else {
    try {
        $ctlSums = (Invoke-WebRequest -UseBasicParsing -Uri "$CtlBase/SHA256SUMS" -Headers @{ 'User-Agent' = 'cd2-harness' }).Content
        if ($ctlSums -is [byte[]]) { $ctlSums = [Text.Encoding]::ASCII.GetString($ctlSums) }
    } catch {
        Write-Host "[AVISO] Release ctl-v$CtlVersion indisponivel; vou usar o cd2-ctl local, se existir."
        $ctlSums = $null
    }
}
$ctlFiles = @()
foreach ($plat in $ToolboxPlatforms) {
    $os, $arch = $plat -split '-', 2
    $ext = if ($os -eq 'windows') { '.exe' } else { '' }
    $dir = Join-Path $Root "bin\ctl\$plat"
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    $dest = Join-Path $dir "cd2-ctl$ext"
    $asset = "cd2-ctl-$plat$ext"
    if ($ctlSums) {
        $want = $null
        foreach ($l in ($ctlSums -split "`n")) {
            if ($l -match '^([0-9a-fA-F]{64})\s+\*?(\S+)\s*$' -and $Matches[2] -eq $asset) { $want = $Matches[1].ToLower() }
        }
        $got = $null
        if ((Test-Path $dest) -and (Get-Item $dest).Length -gt 0) { $got = (Get-FileHash -Algorithm SHA256 $dest).Hash.ToLower() }
        if ($want -and $want -eq $got) {
            Write-Host "Ja conferido, pulando: bin/ctl/$plat/cd2-ctl$ext"
        } else {
            # sem retomar (-C -): o arquivo que esta ai e de outra versao, emendar nele corromperia o binario
            Write-Host "Origem: $CtlBase/$asset"
            if (Test-Path $dest) { Remove-Item -Force $dest }
            & $curl -L --fail -H 'User-Agent: cd2-harness' -o $dest "$CtlBase/$asset"
            if ($LASTEXITCODE -ne 0) { if (Test-Path $dest) { Remove-Item -Force $dest }; throw "Falha ao baixar $CtlBase/$asset" }
            $got = (Get-FileHash -Algorithm SHA256 $dest).Hash.ToLower()
        }
        if (-not $want -or $want -ne $got) { Remove-Item -Force $dest; throw "cd2-ctl $plat nao confere com o SHA256SUMS do release. Rode de novo." }
    } elseif ((Test-Path $dest) -and (Get-Item $dest).Length -gt 0) {
        Write-Host "Usando o cd2-ctl local: bin/ctl/$plat/cd2-ctl$ext"
    } else {
        throw "Sem cd2-ctl para $plat. Publique a tag ctl-v$CtlVersion ou compile o cd2-ctl (sh ctl/build.sh) e rode com CD2_CTL_LOCAL=1."
    }
    $ctlFiles += @{ Path = $dest; Rel = "bin/ctl/$plat/cd2-ctl$ext" }
}

Write-Host ''
Write-Host '== Checksums (anote para conferir depois)'
# Mesmo formato do Mac/Linux (caminhos relativos com /), para o mesmo arquivo valer nos tres sistemas.
$lines = @()
# Os lancadores da raiz entram so se estiverem na copia (os nomes tem espaco: -LiteralPath).
foreach ($rel in @('IA de Bolso.app/Contents/MacOS/IA de Bolso', 'IA de Bolso.exe')) {
    $p = Join-Path $Root ($rel -replace '/', '\')
    if (Test-Path -LiteralPath $p -PathType Leaf) { $lines += "$((Get-FileHash -Algorithm SHA256 -LiteralPath $p).Hash.ToLower())  $rel" }
}
$lines += "$((Get-FileHash -Algorithm SHA256 $bin).Hash.ToLower())  bin/llamafile"
$lines += "$((Get-FileHash -Algorithm SHA256 $model).Hash.ToLower())  models/$ModelFile"
foreach ($f in $toolboxFiles) { $lines += "$((Get-FileHash -Algorithm SHA256 $f.Path).Hash.ToLower())  $($f.Rel)" }
foreach ($f in $ctlFiles) { $lines += "$((Get-FileHash -Algorithm SHA256 $f.Path).Hash.ToLower())  $($f.Rel)" }
$lines | ForEach-Object { Write-Host $_ }
[IO.File]::WriteAllText((Join-Path $Root 'CHECKSUMS.sha256'), (($lines -join "`n") + "`n"))
Write-Host ''
Write-Host "Pronto. Pen drive preparado em: $Root"
Write-Host 'Dica: formate o pen drive em exFAT (FAT32 nao aceita arquivos acima de 4 GB).'
