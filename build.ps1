param(
    [string]$Command = "build"
)

switch ($Command) {
    "build" {
        Write-Host "Building ChronosDB..." -ForegroundColor Green
        go build -o bin\chronosd.exe .\cmd\chronosd\
        if ($LASTEXITCODE -eq 0) {
            Write-Host "Build successful! Binary: bin\chronosd.exe" -ForegroundColor Green
        } else {
            Write-Host "Build failed!" -ForegroundColor Red
        }
    }
    "test" {
        Write-Host "Running tests..." -ForegroundColor Green
        go test ./...
    }
    "run" {
        Write-Host "Running ChronosDB..." -ForegroundColor Green
        .\bin\chronosd.exe -port=50051
    }
    "proto" {
        Write-Host "Generating protobuf code..." -ForegroundColor Green
        protoc --proto_path=proto --go_out=proto --go_opt=paths=source_relative `
            --go-grpc_out=proto --go-grpc_opt=paths=source_relative `
            proto\chronosdb.proto
        Write-Host "Protobuf generation complete!" -ForegroundColor Green
    }
    "clean" {
        Write-Host "Cleaning build artifacts..." -ForegroundColor Green
        Remove-Item -Path "bin\*" -Force -ErrorAction SilentlyContinue
        Write-Host "Clean complete!" -ForegroundColor Green
    }
    default {
        Write-Host "Usage: .\build.ps1 [build|test|run|proto|clean]" -ForegroundColor Yellow
    }
}
