param([Parameter(Mandatory=$true)][int]$DiskNumber, [Parameter(Mandatory=$true)][string]$Label)
$ErrorActionPreference = 'Stop'
try {
    # Releitura antes de apagar: o numero de disco pode ter mudado de dono.
    $alvo = Get-Disk -Number $DiskNumber
    if ($alvo.BusType -notin @('USB', 'SD', 'MMC')) { throw "O disco $DiskNumber nao e um pen drive (barramento $($alvo.BusType)). Nada foi apagado." }
    if ($alvo.IsBoot -or $alvo.IsSystem) { throw "O disco $DiskNumber e o disco do Windows. Nada foi apagado." }
    if ($alvo.Size -gt 260000000000) { throw "O disco $DiskNumber e grande demais para ser um pen drive. Nada foi apagado." }
    Set-Disk -Number $DiskNumber -IsOffline $false -ErrorAction SilentlyContinue
    Set-Disk -Number $DiskNumber -IsReadOnly $false -ErrorAction SilentlyContinue
    Clear-Disk -Number $DiskNumber -RemoveData -RemoveOEM -Confirm:$false
    # Clear-Disk apaga as particoes, mas o disco pode continuar inicializado em MBR.
    # Initialize-Disk so aceita disco cru (RAW); num pen drive ja usado o Windows recusa.
    $disk = Get-Disk -Number $DiskNumber
    if ($disk.PartitionStyle -eq 'RAW') {
        Initialize-Disk -Number $DiskNumber -PartitionStyle MBR
    } elseif ($disk.PartitionStyle -ne 'MBR') {
        throw "O disco ficou em $($disk.PartitionStyle). O pen drive precisa estar em MBR para exFAT."
    }
    $part = New-Partition -DiskNumber $DiskNumber -UseMaximumSize -IsActive -AssignDriveLetter
    Format-Volume -Partition $part -FileSystem exFAT -NewFileSystemLabel $Label -Confirm:$false | Out-Null
    $letter = $null
    for ($i = 0; $i -lt 20 -and -not $letter; $i++) {
        Start-Sleep -Seconds 1
        $letter = (Get-Partition -DiskNumber $DiskNumber | Where-Object { $_.DriveLetter } | Select-Object -First 1).DriveLetter
    }
    if (-not $letter) { throw 'A particao nao recebeu letra.' }
    Write-Output ([string]$letter)
} catch {
    [Console]::Error.WriteLine($_.Exception.Message)
    exit 1
}
