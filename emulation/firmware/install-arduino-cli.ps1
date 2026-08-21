[CmdletBinding()]
param(
    [string]$Destination
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
if (-not $Destination) {
    $Destination = Join-Path $repositoryRoot '.tools\arduino-cli'
}

$release = Invoke-RestMethod -Headers @{ 'User-Agent' = 'Toolbox-Arduino-Installer' } `
    -Uri 'https://api.github.com/repos/arduino/arduino-cli/releases/latest'
$archiveAsset = $release.assets | Where-Object { $_.name -match 'Windows_64bit\.zip$' } | Select-Object -First 1
$checksumAsset = $release.assets | Where-Object { $_.name -match 'checksums\.txt$' } | Select-Object -First 1
if (-not $archiveAsset -or -not $checksumAsset) {
    throw 'The official Arduino release does not contain the expected Windows archive/checksum assets.'
}

$version = $release.tag_name.TrimStart('v')
$versionDirectory = Join-Path $Destination $version
$executable = Join-Path $versionDirectory 'arduino-cli.exe'
if (Test-Path -LiteralPath $executable) {
    Write-Output $executable
    return
}

$temporaryDirectory = Join-Path ([System.IO.Path]::GetTempPath()) ("toolbox-arduino-cli-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $temporaryDirectory | Out-Null
try {
    $archivePath = Join-Path $temporaryDirectory $archiveAsset.name
    $checksumPath = Join-Path $temporaryDirectory 'checksums.txt'
    Invoke-WebRequest -UseBasicParsing -Uri $archiveAsset.browser_download_url -OutFile $archivePath
    Invoke-WebRequest -UseBasicParsing -Uri $checksumAsset.browser_download_url -OutFile $checksumPath

    $publishedLine = Get-Content -LiteralPath $checksumPath |
        Where-Object { $_ -match ([regex]::Escape($archiveAsset.name) + '$') } |
        Select-Object -First 1
    if (-not $publishedLine) {
        throw "No published checksum was found for $($archiveAsset.name)."
    }
    $publishedHash = ($publishedLine -split '\s+')[0].ToUpperInvariant()
    $actualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $archivePath).Hash.ToUpperInvariant()
    if ($actualHash -ne $publishedHash) {
        throw "Arduino CLI SHA-256 mismatch: expected $publishedHash, got $actualHash."
    }

    New-Item -ItemType Directory -Force -Path $versionDirectory | Out-Null
    Expand-Archive -LiteralPath $archivePath -DestinationPath $versionDirectory
    if (-not (Test-Path -LiteralPath $executable)) {
        throw 'arduino-cli.exe was not present in the verified official archive.'
    }
    Write-Output $executable
}
finally {
    if (Test-Path -LiteralPath $temporaryDirectory) {
        Remove-Item -LiteralPath $temporaryDirectory -Recurse -Force
    }
}
