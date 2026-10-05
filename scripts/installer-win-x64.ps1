param(
  [string]$Version = "1.1.0",
  [string]$CertThumbprint = "",
  [switch]$SkipBuild,
  [switch]$Update
)
$ErrorActionPreference = "Stop"
$App = "Spotigent"
if (-not $env:USERPROFILE) { throw "USERPROFILE is not set." }
$Base = Join-Path $env:USERPROFILE "AppData\$App"
$Root = Split-Path -Parent $PSScriptRoot
$ExeName = "spotigent.exe"
function Msg($t, $c) { Write-Host $t -ForegroundColor $c }

if (-not $SkipBuild) {
  Msg "-- building $ExeName ($Version)" Yellow
  go build -trimpath -ldflags "-X main.version=$Version" -o (Join-Path $Root $ExeName) (Join-Path $Root "cmd\spotigent")
  if ($LASTEXITCODE -ne 0 -or !(Test-Path (Join-Path $Root $ExeName))) { throw "build failed" }
  $SignScript = Join-Path $PSScriptRoot "sign-exe.ps1"
  if ($CertThumbprint -or (Get-ChildItem Cert:\CurrentUser\My -ErrorAction SilentlyContinue | Where-Object { $_.HasPrivateKey -and $_.EnhancedKeyUsageList.ObjectId.Value -contains "1.3.6.1.5.5.7.3.3" -and $_.Subject -like "*Dashti Technologies LLC*" })) {
    & $SignScript -Path (Join-Path $Root $ExeName) -CertThumbprint $CertThumbprint
  } else {
    Msg "no matching code-signing certificate; installing unsigned development build" Yellow
  }
}

$Source = Join-Path $Root $ExeName
if (!(Test-Path $Source)) { throw "Build executable not found: $Source" }
if ($Update) {
  $SelfUpdate = @'
param([string]$Source, [string]$Target, [string]$Base)
$ErrorActionPreference = "Stop"
Start-Sleep -Milliseconds 800
$next = "$Target.new"
Copy-Item -LiteralPath $Source -Destination $next -Force
Move-Item -LiteralPath $next -Destination $Target -Force
$hash = (Get-FileHash -LiteralPath $Target -Algorithm SHA256).Hash.ToLowerInvariant()
Set-Content -LiteralPath "$Target.sha256" -Value "$hash  spotigent.exe" -Encoding ASCII
Write-Host "Updated SpotiGent in $Base" -ForegroundColor Green
'@
  $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($SelfUpdate))
  Start-Process -FilePath "powershell.exe" -ArgumentList @("-NoProfile", "-NonInteractive", "-EncodedCommand", $encoded, $Source, (Join-Path $Base $ExeName), $Base) -WindowStyle Hidden
  Msg "update started; it will replace the executable after this CLI exits" Yellow
  exit 0
}
$PidFile = Join-Path $Base "spotigent.pid"
if (Test-Path $PidFile) {
  $serverPid = Get-Content $PidFile -ErrorAction SilentlyContinue
  if ($serverPid -match '^\d+$') { & taskkill.exe /PID $serverPid /T /F 2>$null | Out-Null }
  Remove-Item $PidFile -Force -ErrorAction SilentlyContinue
  Start-Sleep -Milliseconds 500
}
New-Item -ItemType Directory -Force $Base | Out-Null
Copy-Item $Source (Join-Path $Base $ExeName) -Force
$installedHash = (Get-FileHash -Path (Join-Path $Base $ExeName) -Algorithm SHA256).Hash.ToLowerInvariant()
Set-Content -Path (Join-Path $Base "$ExeName.sha256") -Value "$installedHash  $ExeName" -Encoding ASCII

# Keep settings and history under the requested installation directory.
$DataDir = Join-Path $Base "data"
New-Item -ItemType Directory -Force $DataDir | Out-Null
# The path is resolved by the application from USERPROFILE; remove legacy override.
[Environment]::SetEnvironmentVariable("SPOTIGENT_DATA_DIR", $null, "User")
[Environment]::SetEnvironmentVariable("SPOTIGENT_SOURCE_DIR", $Root, "User")
$env:SPOTIGENT_DATA_DIR = $null
$env:SPOTIGENT_SOURCE_DIR = $Root
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
$Entries = @($UserPath -split ";" | Where-Object { $_ -and $_.TrimEnd('\') -ine $Base.TrimEnd('\') })
$Entries += $Base
[Environment]::SetEnvironmentVariable("Path", ($Entries -join ";"), "User")
$env:Path = "$Base;$env:Path"

try {
  if (-not ('Spotigent.NativeMethods' -as [type])) {
    Add-Type -Namespace Spotigent -Name NativeMethods -MemberDefinition @'
[System.Runtime.InteropServices.DllImport("user32.dll", CharSet=System.Runtime.InteropServices.CharSet.Auto, SetLastError=true)]
public static extern System.IntPtr SendMessageTimeout(System.IntPtr hWnd, uint Msg, System.UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out System.UIntPtr lpdwResult);
'@
  }
  $result = [UIntPtr]::Zero
  [void][Spotigent.NativeMethods]::SendMessageTimeout([IntPtr]0xffff, 0x001A, [UIntPtr]::Zero, "Environment", 2, 5000, [ref]$result)
} catch { Write-Verbose "Environment change notification skipped: $_" }
Msg "installed $ExeName to $Base" Green
Msg "CLI available: spotigent help (in new terminals)" Green
