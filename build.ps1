# build.ps1 – build resctl.exe for Windows amd64
$env:GOOS   = "windows"
$env:GOARCH = "amd64"
go build -ldflags="-s -w" -o resctl.exe .
if ($LASTEXITCODE -eq 0) { Write-Host "Built: resctl.exe" }
