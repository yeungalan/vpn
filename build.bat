@echo off
REM Build script for Windows

echo Building WireGuard Mesh VPN...
echo.

REM Create bin directory if it doesn't exist
if not exist "bin" mkdir bin

echo Downloading dependencies...
go mod download
go mod tidy
echo.

echo Building server...
go build -ldflags "-w -s" -o bin\vpn-server.exe .\cmd\server
if %errorlevel% neq 0 (
    echo Failed to build server
    exit /b %errorlevel%
)
echo Server built successfully: bin\vpn-server.exe
echo.

echo Building client...
go build -ldflags "-w -s" -o bin\vpn-client.exe .\cmd\client
if %errorlevel% neq 0 (
    echo Failed to build client
    exit /b %errorlevel%
)
echo Client built successfully: bin\vpn-client.exe
echo.

echo Build completed successfully!
echo.
echo To run the server: bin\vpn-server.exe
echo To run the client: bin\vpn-client.exe -server http://SERVER_IP:8080
