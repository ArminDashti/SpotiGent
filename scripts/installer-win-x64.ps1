param([string]$Version = "1.1.0", [string]$CertThumbprint = "")
$ErrorActionPreference = "Stop"
$App = "SpotiGent"; $Base = "$env:LOCALAPPDATA\$App"; $Root = Split-Path -Parent $PSScriptRoot
function Msg($t, $c) { Write-Host $t -ForegroundColor $c }
Msg "-- building $App.exe ($Version)" Yellow
go build -trimpath -ldflags "-X main.version=$Version" -o "$Root\spotigent.exe" "$Root\cmd\spotigent"
if ($LASTEXITCODE -ne 0 -or !(Test-Path "$Root\spotigent.exe")) { Msg "build failed" Red; exit 1 }
& (Join-Path $PSScriptRoot "sign-exe.ps1") -Path "$Root\spotigent.exe" -CertThumbprint $CertThumbprint
Msg "build ok" Green
Get-Process $App -EA SilentlyContinue | Stop-Process -Force -EA SilentlyContinue
Msg "stopped existing instance" Yellow
New-Item -ItemType Directory -Force $Base | Out-Null
if (!(Test-Path "$Base\Settings.json")) { '{}' | Set-Content "$Base\Settings.json"; Msg "created Settings.json" Yellow }
if (!(Test-Path "$Base\Data.db")) { New-Item "$Base\Data.db" -ItemType File -Force | Out-Null; Msg "created Data.db" Yellow }
Remove-Item "$Base\$App.exe" -Force -EA SilentlyContinue
Copy-Item "$Root\spotigent.exe" "$Base\$App.exe" -Force
$installedHash = (Get-FileHash -Path "$Base\$App.exe" -Algorithm SHA256).Hash.ToLowerInvariant()
Set-Content -Path "$Base\$App.exe.sha256" -Value "$installedHash  $App.exe" -Encoding ASCII
Msg "installed to $Base" Green
[Environment]::SetEnvironmentVariable("SPOTIGENT_DATA_DIR", $Base, "User")
$pf = "$Base"; $up = [Environment]::GetEnvironmentVariable("Path", "User")
if (($up -split ";") -notcontains $pf) { [Environment]::SetEnvironmentVariable("Path", "$up;$pf", "User"); Msg "added PATH" Green }
else { Msg "PATH already set" Yellow }
Msg "done: run $Base\$App.exe" Green
