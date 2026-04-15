# build.ps1 – install tools, embed Windows resources, and build resctl.exe

$ErrorActionPreference = "Stop"

Write-Host "Installing goversioninfo..."
go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest

Write-Host "Generating resource.syso..."
goversioninfo -64 -o resource.syso

$version = if ($env:VERSION) { $env:VERSION } else { "dev" }
Write-Host "Building resctl.exe (version=$version)..."

$env:GOOS   = "windows"
$env:GOARCH = "amd64"
go build -ldflags="-s -w -X main.version=$version" -o resctl.exe .

if ($LASTEXITCODE -eq 0) {
    Write-Host "Done: resctl.exe"
    Write-Host "  Set `$env:VERSION=v1.2.3 to embed a version string."
}
