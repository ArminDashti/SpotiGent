<#
.SYNOPSIS
  Reproducible SpotiGent release build for Windows.

  - Embeds CompanyName "Dashti Technologies LLC" + icon into spotigent.exe
  - Stamps the version, writes SHA256 hashes for every artifact
  - Requires an Authenticode signature from a Dashti Technologies LLC code-signing certificate (SHA256)

.EXAMPLE
  .\scripts\build-release.ps1 -Version 1.1.0

.EXAMPLE
  .\scripts\build-release.ps1 -Version 1.1.0 -CertThumbprint ABC123...

.NOTES
  A SHA256 hash lets users verify a download, but it does NOT stop
  antivirus/SmartScreen warnings by itself. Only a real (OV/EV)
  code-signing certificate issued to "Dashti Technologies LLC", plus
  reputation built over time and a Microsoft Defender sample submission,
  removes those warnings. A self-signed certificate only helps on machines
  where you installed that certificate as trusted (see the snippet at the
  bottom of this file).
#>
param(
  [string]$Version = "1.0.0",
  [string]$CertThumbprint = "",
  [switch]$SkipWeb,
  [string]$OutputDirectory = ""
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $PSCommandPath)
$OutDir = if ($OutputDirectory) { $OutputDirectory } else { Join-Path $Root "release" }
New-Item -ItemType Directory -Force $OutDir | Out-Null

# 1. Frontend (embeds manifest, service worker, icons into web/dist).
if (-not $SkipWeb) {
  Write-Host "-- building web UI" -ForegroundColor Cyan
  Push-Location (Join-Path $Root "web")
  try {
    npm run build
  } finally { Pop-Location }
}

# 2. Windows version resource (company name, description, icon).
$Syso = Join-Path $Root "cmd\spotigent\resource.syso"
$VersionJson = Join-Path $Root "build\windows\versioninfo.json"
if (Get-Command goversioninfo -ErrorAction SilentlyContinue) {
  Write-Host "-- embedding version resource" -ForegroundColor Cyan
  Push-Location (Join-Path $Root "cmd\spotigent")
  try {
    Copy-Item (Join-Path $Root "build\windows\icon.ico") ".\_icon.tmp.ico"
    $parts = $Version.Split(".")
    $major = $parts[0]
    $minor = if ($parts.Count -gt 1) { $parts[1] } else { "0" }
    $patch = if ($parts.Count -gt 2) { $parts[2] } else { "0" }
    (Get-Content $VersionJson) -replace '"VersionString": "[^"]*"', ('"VersionString": "{0}"' -f $Version) `
      -replace '"FileVersion": "[^"]*"', ('"FileVersion": "{0}"' -f $Version) `
      -replace '"ProductVersion": "[^"]*"', ('"ProductVersion": "{0}"' -f $Version) `
      -replace '"Major": \d+', ('"Major": {0}' -f $major) `
      -replace '"Minor": \d+', ('"Minor": {0}' -f $minor) `
      -replace '"Patch": \d+', ('"Patch": {0}' -f $patch) `
      -replace '"IconPath": "[^"]*"', '"IconPath": "_icon.tmp.ico"' |
      Set-Content "_versioninfo.tmp.json"
    goversioninfo -o $Syso "_versioninfo.tmp.json"
    if ($LASTEXITCODE -ne 0) { throw "goversioninfo failed." }
  } finally {
    Remove-Item "_versioninfo.tmp.json", "_icon.tmp.ico" -ErrorAction SilentlyContinue
    Pop-Location
  }
} else {
  Write-Host "!! goversioninfo not found - exe will lack CompanyName metadata." -ForegroundColor Yellow
  Write-Host "   Install it once (needs network): go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest"
}

# 3. Backend binary with stamped version.
Write-Host "-- building spotigent.exe ($Version)" -ForegroundColor Cyan
$Exe = Join-Path $OutDir "spotigent.exe"
go build -trimpath -ldflags "-X main.version=$Version" -o $Exe (Join-Path $Root "cmd\spotigent")
if ($LASTEXITCODE -ne 0) { throw "go build failed." }
if (Test-Path $Syso) { Remove-Item $Syso }

# 4. Authenticode signing is mandatory; never publish an unsigned executable.
& (Join-Path $PSScriptRoot "sign-exe.ps1") -Path $Exe -CertThumbprint $CertThumbprint

# 5. SHA256 hashes for every release artifact.
# (.NET implementation: works even where Get-FileHash is unavailable.)
function Get-Sha256Hex($Path) {
  $sha = [System.Security.Cryptography.SHA256]::Create()
  try {
    $fs = [System.IO.File]::OpenRead($Path)
    try {
      $hash = $sha.ComputeHash($fs)
    } finally { $fs.Close() }
    return ([BitConverter]::ToString($hash)).Replace("-", "").ToLower()
  } finally { $sha.Dispose() }
}
Write-Host "-- hashing artifacts" -ForegroundColor Cyan
$Checksums = Join-Path $OutDir "CHECKSUMS.sha256"
Get-ChildItem $OutDir -File |
  Where-Object { $_.Name -ne "CHECKSUMS.sha256" } |
  ForEach-Object { "{0}  {1}" -f (Get-Sha256Hex $_.FullName), $_.Name } |
  Set-Content $Checksums -Encoding ASCII
Get-Content $Checksums
Write-Host "Release ready in $OutDir" -ForegroundColor Green

<#
Self-signed certificate for INTERNAL testing only (does not stop AV warnings
on other machines):

  $cert = New-SelfSignedCertificate -Type CodeSigningCert `
    -Subject "CN=Dashti Technologies LLC" `
    -CertStoreLocation Cert:\CurrentUser\My `
    -NotAfter (Get-Date).AddYears(3)
  # trust it on this machine:
  $store = New-Object System.Security.Cryptography.X509Certificates.X509Store("TrustedPublisher","CurrentUser")
  $store.Open("ReadWrite"); $store.Add($cert); $store.Close()
  # then build with: .\scripts\build-release.ps1 -Version 1.1.0 -CertThumbprint $cert.Thumbprint
#>
