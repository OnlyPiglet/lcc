param(
  [string]$Output = "dist\every.exe"
)

$ErrorActionPreference = "Stop"

$go = Get-Command go.exe -ErrorAction SilentlyContinue
if (-not $go) {
  Write-Error "go.exe not found; install Go 1.21 or newer"
  exit 1
}

$Output = [IO.Path]::GetFullPath($Output)
$outputDir = Split-Path -Parent $Output
New-Item -ItemType Directory -Path $outputDir -Force | Out-Null

& $go.Source build -trimpath -o $Output ./cmd/every
if ($LASTEXITCODE -ne 0) {
  exit $LASTEXITCODE
}

Write-Host "built $Output"
