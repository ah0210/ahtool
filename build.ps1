# ah系统优化工具箱（ahtool）构建脚本（PowerShell 7 / Windows）
# 用法：pwsh -File build.ps1 [-Release]
param([switch]$Release)

$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$AppVer = '1.0.0'
$BuildDate = Get-Date -Format 'yyyy-MM-dd'
$Go = if (Test-Path "$Root\_toolchain\go\bin\go.exe") { "$Root\_toolchain\go\bin\go.exe" } else { 'go' }
$Version = if ($Release) { $AppVer } else { "$AppVer-dev" }
$Exe = if ($Release) { "$Root\bin\ahtool_v$AppVer.exe" } else { "$Root\bin\ahtool_$AppVer-dev.exe" }

New-Item -ItemType Directory -Force -Path "$Root\bin" | Out-Null
$env:GOOS = 'windows'; $env:GOARCH = 'amd64'; $env:CGO_ENABLED = '0'
$ldflags = "-s -w -H windowsgui -X main.appVersion=$Version -X main.buildDate=$BuildDate"

Push-Location $Root
try {
  & $Go build -buildvcs=false -trimpath -tags desktop,production -ldflags $ldflags -o $Exe .
  if ($LASTEXITCODE -ne 0) { throw "build failed" }
} finally {
  Pop-Location
}
Write-Host "OK: $Exe" -ForegroundColor Green
