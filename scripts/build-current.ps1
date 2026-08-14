$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$wails = Join-Path (go env GOPATH) 'bin\wails.exe'

if (-not (Test-Path -LiteralPath $wails)) {
    go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
}

Push-Location $projectRoot
try {
    go test ./...
    & $wails build -clean -webview2 embed
    if ($LASTEXITCODE -ne 0) { throw 'Wails build failed' }
    Write-Host "Build ready: $projectRoot\build\bin\HarnessStudio.exe"
} finally {
    Pop-Location
}
