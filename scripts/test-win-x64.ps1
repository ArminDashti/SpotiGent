$ErrorActionPreference = "Stop"
$App = "SpotiGent"; $Base = "$env:LOCALAPPDATA\$App"; $Root = Split-Path -Parent $PSScriptRoot
$Inst = "$PSScriptRoot\installer-win-x64.ps1"; $Rem = "$PSScriptRoot\Remove-win-x64.ps1"
$pass = 0; $fail = 0
function Ok($t, $c) { if ($c) { $script:pass++ ; Write-Host "PASS: $t" -ForegroundColor Green } else { $script:fail++; Write-Host "FAIL: $t" -ForegroundColor Red } }
Write-Host "== build ==" -ForegroundColor Yellow
go build -trimpath -o "$Root\spotigent.exe" "$Root\cmd\spotigent"
if ($LASTEXITCODE -ne 0 -or !(Test-Path "$Root\spotigent.exe")) { throw "build failed" }
& (Join-Path $PSScriptRoot "sign-exe.ps1") -Path "$Root\spotigent.exe"
$signature = Get-AuthenticodeSignature "$Root\spotigent.exe"
Ok "build creates signed spotigent.exe for Dashti Technologies LLC" ($signature.SignerCertificate -and $signature.SignerCertificate.Subject -like "*Dashti Technologies LLC*")
Ok "build writes SHA256 sidecar" ((Get-Content "$Root\spotigent.exe.sha256") -match '^[0-9a-f]{64}\s+spotigent\.exe$')
Write-Host "== install ==" -ForegroundColor Yellow
& $Inst
Ok "install copies exe" (Test-Path "$Base\$App.exe")
$installedSignature = Get-AuthenticodeSignature "$Base\$App.exe"
Ok "install preserves Dashti Technologies LLC signature" ($installedSignature.SignerCertificate -and $installedSignature.SignerCertificate.Subject -like "*Dashti Technologies LLC*")
$installedHash = (Get-FileHash -Path "$Base\$App.exe" -Algorithm SHA256).Hash.ToLowerInvariant()
Ok "install writes matching SHA256 sidecar" ((Get-Content "$Base\$App.exe.sha256") -eq "$installedHash  $App.exe")
Ok "install creates Settings.json" (Test-Path "$Base\Settings.json")
Ok "install creates Data.db" (Test-Path "$Base\Data.db")
Ok "install updates PATH" (([Environment]::GetEnvironmentVariable("Path", "User") -split ";") -contains $Base)
Write-Host "== update ==" -ForegroundColor Yellow
& $Inst
Ok "update keeps exe" (Test-Path "$Base\$App.exe")
Write-Host "== remove ==" -ForegroundColor Yellow
& $Rem
Ok "remove deletes base" (!(Test-Path $Base))
Ok "remove cleans PATH" (([Environment]::GetEnvironmentVariable("Path", "User") -split ";") -notcontains $Base)
Write-Host "result: $pass passed, $fail failed" -ForegroundColor $(if ($fail -eq 0) { "Green" } else { "Red" })
if ($fail -gt 0) { exit 1 }
