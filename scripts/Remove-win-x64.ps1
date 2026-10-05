param()
$ErrorActionPreference = "Stop"
$App = "Spotigent"
if (-not $env:USERPROFILE) { throw "USERPROFILE is not set." }
$Base = Join-Path $env:USERPROFILE "AppData\$App"
$PidFile = Join-Path $Base "spotigent.pid"
if (Test-Path $PidFile) {
  $serverPid = Get-Content $PidFile -ErrorAction SilentlyContinue
  if ($serverPid -match '^\d+$') {
    & taskkill.exe /PID $serverPid /T /F 2>$null | Out-Null
  }
  Remove-Item $PidFile -Force -ErrorAction SilentlyContinue
  Start-Sleep -Milliseconds 500
}
if (Test-Path $Base) { Remove-Item -LiteralPath $Base -Recurse -Force; Write-Host "Removed $Base" -ForegroundColor Green }
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
$Entries = @($UserPath -split ";" | Where-Object { $_ -and $_.TrimEnd('\') -ine $Base.TrimEnd('\') })
[Environment]::SetEnvironmentVariable("Path", ($Entries -join ";"), "User")
[Environment]::SetEnvironmentVariable("SPOTIGENT_DATA_DIR", $null, "User")
[Environment]::SetEnvironmentVariable("SPOTIGENT_SOURCE_DIR", $null, "User")
Write-Host "Removed SpotiGent and its user PATH entry." -ForegroundColor Green
