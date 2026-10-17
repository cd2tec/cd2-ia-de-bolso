param()
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$lista = @(Get-Disk | Where-Object { $_.BusType -in @('USB', 'SD', 'MMC') } | Sort-Object Number | ForEach-Object {
    $n = $_.Number
    $letras = @(Get-Partition -DiskNumber $n -ErrorAction SilentlyContinue | Where-Object { $_.DriveLetter } | ForEach-Object { [string]$_.DriveLetter })
    [PSCustomObject]@{
        Number       = $n
        FriendlyName = $_.FriendlyName
        Size         = $_.Size
        IsBoot       = $_.IsBoot
        IsSystem     = $_.IsSystem
        BusType      = [string]$_.BusType
        DriveLetters = $letras
    }
})
ConvertTo-Json -InputObject $lista -Depth 3
