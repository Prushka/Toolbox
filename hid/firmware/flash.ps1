[CmdletBinding()]
param(
    [string]$Port,
    [string]$ArduinoCLI,
    [switch]$CompileOnly
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$sketch = Join-Path $PSScriptRoot 'leonardo'

if (-not $ArduinoCLI) {
    $candidate = Get-ChildItem -LiteralPath (Join-Path $repositoryRoot '.tools\arduino-cli') `
        -Filter 'arduino-cli.exe' -File -Recurse -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1
    if ($candidate) {
        $ArduinoCLI = $candidate.FullName
    }
}
if (-not $ArduinoCLI) {
    $command = Get-Command arduino-cli -ErrorAction SilentlyContinue
    if ($command) {
        $ArduinoCLI = $command.Source
    }
}
if (-not $ArduinoCLI -or -not (Test-Path -LiteralPath $ArduinoCLI)) {
    throw 'Arduino CLI was not found. Run .\install-arduino-cli.ps1 first.'
}

if (-not $CompileOnly -and -not $Port) {
    $matchingPorts = @(Get-CimInstance Win32_SerialPort |
        Where-Object { $_.PNPDeviceID -match 'VID_046D&PID_C223' })
    if ($matchingPorts.Count -ne 1) {
        throw "Expected one 046D:C223 serial port, found $($matchingPorts.Count). Pass -Port explicitly."
    }
    $Port = $matchingPorts[0].DeviceID
}

# These commands install Arduino's signed/official AVR board package only.
& $ArduinoCLI core update-index
if ($LASTEXITCODE -ne 0) { throw 'arduino-cli core update-index failed.' }
& $ArduinoCLI core install arduino:avr
if ($LASTEXITCODE -ne 0) { throw 'arduino-cli core install failed.' }
& $ArduinoCLI core upgrade arduino:avr
if ($LASTEXITCODE -ne 0) { throw 'arduino-cli core upgrade failed.' }
& $ArduinoCLI lib install Keyboard
if ($LASTEXITCODE -ne 0) { throw 'arduino-cli Keyboard library install failed.' }
& $ArduinoCLI lib upgrade Keyboard
if ($LASTEXITCODE -ne 0) { throw 'arduino-cli Keyboard library upgrade failed.' }

$arguments = @(
    'compile',
    '--fqbn', 'arduino:avr:leonardo',
    '--build-property', 'build.vid=0x046D',
    '--build-property', 'build.pid=0xC223',
    '--build-property', 'build.usb_manufacturer="Logitech"',
    '--build-property', 'build.usb_product="Logitech G15 Gaming Keyboard"'
)
if (-not $CompileOnly) {
    $arguments += @('--upload', '--port', $Port)
}
$arguments += $sketch

& $ArduinoCLI @arguments
if ($LASTEXITCODE -ne 0) { throw 'Arduino compile/upload failed.' }
