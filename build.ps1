# Build script for Windows (PowerShell)

Write-Host "Building WireGuard Mesh VPN..." -ForegroundColor Cyan
Write-Host ""

# Create bin directory if it doesn't exist
if (-not (Test-Path "bin")) {
    New-Item -ItemType Directory -Path "bin" | Out-Null
}

Write-Host "Downloading dependencies..." -ForegroundColor Yellow
go mod download
go mod tidy
Write-Host ""

Write-Host "Building server..." -ForegroundColor Yellow
go build -ldflags "-w -s" -o bin\vpn-server.exe .\cmd\server
if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to build server" -ForegroundColor Red
    exit $LASTEXITCODE
}
Write-Host "Server built successfully: bin\vpn-server.exe" -ForegroundColor Green
Write-Host ""

Write-Host "Building client..." -ForegroundColor Yellow
go build -ldflags "-w -s" -o bin\vpn-client.exe .\cmd\client
if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to build client" -ForegroundColor Red
    exit $LASTEXITCODE
}
Write-Host "Client built successfully: bin\vpn-client.exe" -ForegroundColor Green
Write-Host ""

Write-Host "Build completed successfully!" -ForegroundColor Green
Write-Host ""
Write-Host "To run the server: .\bin\vpn-server.exe" -ForegroundColor Cyan
Write-Host "To run the client: .\bin\vpn-client.exe -server http://SERVER_IP:8080" -ForegroundColor Cyan
