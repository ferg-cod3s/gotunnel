$ErrorActionPreference = 'Stop'

$packageName = 'gotunnel'
$toolsDir = "$(Split-Path -parent $MyInvocation.MyCommand.Definition)"
$url64 = 'https://github.com/johncferguson/gotunnel/releases/download/v0.1.0-beta/gotunnel-v0.1.0-beta-windows-amd64.exe'
$checksum64 = 'PLACEHOLDER_SHA256'

$packageArgs = @{
  packageName   = $packageName
  unzipLocation = $toolsDir
  fileType      = 'exe'
  url64bit      = $url64
  softwareName  = 'gotunnel*'
  checksum64    = $checksum64
  checksumType64= 'sha256'
  silentArgs    = '/VERYSILENT /SUPPRESSMSGBOXES /NORESTART /SP-'
  validExitCodes= @(0)
}

# Download and install the package
Install-ChocolateyPackage @packageArgs

# Add to PATH if not already there
$binPath = Join-Path $toolsDir 'gotunnel.exe'
if (Test-Path $binPath) {
    Write-Host "gotunnel installed successfully to: $binPath" -ForegroundColor Green
    Write-Host ""
    Write-Host "Quick start:" -ForegroundColor Yellow
    Write-Host "  gotunnel --help" -ForegroundColor Cyan
    Write-Host "  gotunnel --proxy=builtin start --port 3000 --domain myapp" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Documentation: https://github.com/johncferguson/gotunnel" -ForegroundColor Magenta
}