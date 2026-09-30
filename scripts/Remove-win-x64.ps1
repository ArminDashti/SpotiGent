param()
$ErrorActionPreference = "Stop"
$App = "SpotiGent"; $Base = "$env:LOCALAPPDATA\$App"
function Msg($t, $c) { Write-Host $t -ForegroundColor $c }
Get-Process $App -EA SilentlyContinue | Stop-Process -Force -EA SilentlyContinue
Msg "stopped $App" Yellow
if (Test-Path $Base) { Remove-Item $Base -Recurse -Force; Msg "removed $Base" Green }
else { Msg "base path absent" Yellow }
$up = [Environment]::GetEnvironmentVariable("Path", "User")
if (($up -split ";") -contains $Base) { [Environment]::SetEnvironmentVariable("Path", (($up -split ";") | Where-Object { $_ -ne "" -and $_ -ne $Base }) -join ";", "User"); Msg "removed from PATH" Green }
else { Msg "PATH already clean" Yellow }
[Environment]::SetEnvironmentVariable("SPOTIGENT_DATA_DIR", $null, "User")
Msg "removed" Green
