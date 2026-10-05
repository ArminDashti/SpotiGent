$ErrorActionPreference = "Stop"
$App = "Spotigent"
$Base = Join-Path $env:USERPROFILE "AppData\$App"
$Root = Split-Path -Parent $PSScriptRoot
$Inst = Join-Path $PSScriptRoot "installer-win-x64.ps1"
$Rem = Join-Path $PSScriptRoot "Remove-win-x64.ps1"
$pass = 0; $fail = 0
function Ok($t, $c) { if ($c) { $script:pass++; Write-Host "PASS: $t" -ForegroundColor Green } else { $script:fail++; Write-Host "FAIL: $t" -ForegroundColor Red } }
Write-Host "== build ==" -ForegroundColor Yellow
go build -trimpath -ldflags "-X main.version=1.1.0" -o (Join-Path $Root "spotigent.exe") (Join-Path $Root "cmd\spotigent")
if ($LASTEXITCODE -ne 0 -or !(Test-Path (Join-Path $Root "spotigent.exe"))) { throw "build failed" }
& $Inst -SkipBuild
$Exe = Join-Path $Base "spotigent.exe"
Ok "install copies executable to AppData\\Spotigent" (Test-Path $Exe)
Ok "install creates data directory" (Test-Path (Join-Path $Base "data"))
Ok "install adds base to user PATH" (([Environment]::GetEnvironmentVariable("Path", "User") -split ";") -contains $Base)
Ok "help command works" ((& $Exe help 2>&1 | Out-String) -match "spotigent start")
Ok "version command works" ((& $Exe version 2>&1 | Out-String).Trim() -eq "1.1.0")
Write-Host "== start/stop ==" -ForegroundColor Yellow
& $Exe start
Start-Sleep -Seconds 2
Ok "start listens on default port 9090" ((Invoke-WebRequest "http://127.0.0.1:9090/api/health" -UseBasicParsing -TimeoutSec 5).StatusCode -eq 200)
& $Exe stop
Start-Sleep -Milliseconds 500
Ok "stop closes default port" (-not (Test-NetConnection 127.0.0.1 -Port 9090 -WarningAction SilentlyContinue).TcpTestSucceeded)
Write-Host "== remove ==" -ForegroundColor Yellow
& $Exe remove
Start-Sleep -Seconds 1
Ok "remove deletes installation" (!(Test-Path $Base))
Ok "remove cleans PATH" (([Environment]::GetEnvironmentVariable("Path", "User") -split ";") -notcontains $Base)
Write-Host "result: $pass passed, $fail failed" -ForegroundColor $(if ($fail -eq 0) { "Green" } else { "Red" })
if ($fail -gt 0) { exit 1 }
