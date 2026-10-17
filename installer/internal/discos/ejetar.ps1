param([string]$Letter)
$ErrorActionPreference = 'Stop'
try {
    (New-Object -ComObject Shell.Application).NameSpace(17).ParseName("${Letter}:").InvokeVerb('Eject')
} catch {
    [Console]::Error.WriteLine($_.Exception.Message)
    exit 1
}
