$ErrorActionPreference = 'Stop'

$packageName = 'gotunnel'
$softwareName = 'gotunnel*'
$installerType = 'exe'

$silentArgs = '/VERYSILENT /SUPPRESSMSGBOXES /NORESTART /SP-'
$validExitCodes = @(0)

$uninstalled = $false

# Get all uninstall registry keys
[array]$key = Get-UninstallRegistryKey -SoftwareName $softwareName

if ($key.Count -eq 1) {
  $key | % { 
    $packageArgs = @{
      packageName = $packageName
      fileType    = $installerType
      silentArgs  = "$($_.PSChildName) $silentArgs"
      validExitCodes = $validExitCodes
      file        = "$($_.UninstallString)"
    }

    Uninstall-ChocolateyPackage @packageArgs
  }
} elseif ($key.Count -eq 0) {
  Write-Warning "$packageName has already been uninstalled by other means."
} elseif ($key.Count -gt 1) {
  Write-Warning "$($key.Count) matches found!"
  Write-Warning "To prevent accidental data loss, no programs will be uninstalled."
  Write-Warning "Please alert package maintainer the following keys were matched:"
  $key | % {Write-Warning "- $($_.DisplayName)"}
}

Write-Host "gotunnel has been successfully uninstalled." -ForegroundColor Green