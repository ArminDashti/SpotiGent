param(
  [Parameter(Mandatory = $true)]
  [string]$Path,
  [string]$CertThumbprint = ""
)

$ErrorActionPreference = "Stop"
$Path = (Resolve-Path $Path).Path

$certificates = @(
  Get-ChildItem Cert:\CurrentUser\My, Cert:\LocalMachine\My -ErrorAction SilentlyContinue |
    Where-Object {
      $_.Subject -match '(?i)CN=Dashti Technologies LLC(?:\s+\([^)]*\))?(?:,|$)' -and
      $_.HasPrivateKey -and
      $_.NotBefore -le (Get-Date) -and
      $_.NotAfter -gt (Get-Date) -and
      @($_.Extensions | Where-Object { $_.Oid.Value -eq "2.5.29.37" } |
        ForEach-Object { $_.EnhancedKeyUsages } |
        ForEach-Object { $_.Value }) -contains "1.3.6.1.5.5.7.3.3"
    } |
    Sort-Object Thumbprint -Unique
)

if ($CertThumbprint) {
  $normalizedThumbprint = ($CertThumbprint -replace '\s', '').ToUpperInvariant()
  $certificate = $certificates | Where-Object { $_.Thumbprint -eq $normalizedThumbprint } | Select-Object -First 1
  if (-not $certificate) {
    throw "No valid Dashti Technologies LLC code-signing certificate with a private key matches the requested thumbprint."
  }
} else {
  if ($certificates.Count -eq 0) {
    throw "No valid Dashti Technologies LLC code-signing certificate with a private key was found. Install one or pass -CertThumbprint."
  }
  if ($certificates.Count -gt 1) {
    throw "Multiple valid Dashti Technologies LLC signing certificates were found. Select one with -CertThumbprint."
  }
  $certificate = $certificates[0]
}

$signTool = Get-Command signtool.exe -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty Source
if (-not $signTool) {
  $sdkBin = Join-Path ${env:ProgramFiles(x86)} "Windows Kits\10\bin"
  if (Test-Path $sdkBin) {
    $signTool = Get-ChildItem $sdkBin -Directory |
      Sort-Object Name -Descending |
      ForEach-Object {
        $candidate = Join-Path $_.FullName "x64\signtool.exe"
        if (Test-Path $candidate) { $candidate; break }
      } | Select-Object -First 1
  }
}
if (-not $signTool) { throw "signtool.exe not found. Install the Windows SDK." }

Write-Host "-- signing $Path as $($certificate.Subject) with SHA256" -ForegroundColor Cyan
$signArgs = @("sign")
if ($certificate.PSParentPath -like "*LocalMachine*") { $signArgs += "/sm" }
$signArgs += @("/sha1", $certificate.Thumbprint, "/fd", "SHA256", "/tr", "http://timestamp.digicert.com", "/td", "SHA256", $Path)
& $signTool @signArgs
if ($LASTEXITCODE -ne 0) { throw "signtool signing failed with exit code $LASTEXITCODE." }

$signature = Get-AuthenticodeSignature -FilePath $Path
if (-not $signature.SignerCertificate -or $signature.SignerCertificate.Thumbprint -ne $certificate.Thumbprint) {
  throw "Authenticode signature verification failed: signer does not match the selected certificate."
}
$hash = (Get-FileHash -Path $Path -Algorithm SHA256).Hash.ToLowerInvariant()
Set-Content -Path "$Path.sha256" -Value "$hash  $(Split-Path -Leaf $Path)" -Encoding ASCII
Write-Host "SHA256  $hash  $(Split-Path -Leaf $Path)"
if ($certificate.Subject -eq $certificate.Issuer) {
  Write-Warning "The signing certificate is self-issued; signatures may not be trusted on other machines until the certificate is trusted."
}
