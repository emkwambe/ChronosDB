# deploy.ps1 - Deploy ChronosDB on Windows

param(
    [string]$Mode = "docker"
)

function Test-DockerRunning {
    try {
        docker ps 2>$null | Out-Null
        return $true
    } catch {
        return $false
    }
}

function Test-KubectlRunning {
    try {
        kubectl version --client 2>$null | Out-Null
        return $true
    } catch {
        return $false
    }
}

switch ($Mode) {
    "docker" {
        Write-Host "Starting ChronosDB with Docker..." -ForegroundColor Green
        
        if (-not (Test-DockerRunning)) {
            Write-Host "Docker is not running. Please start Docker Desktop first." -ForegroundColor Red
            exit 1
        }
        
        # Stop existing container if running
        docker stop chronosdb 2>$null | Out-Null
        docker rm chronosdb 2>$null | Out-Null
        
        # Build image
        Write-Host "Building Docker image..." -ForegroundColor Yellow
        docker build -f deployments/docker/Dockerfile -t chronosdb:latest . 2>&1
        
        # Run container
        Write-Host "Starting ChronosDB container..." -ForegroundColor Yellow
        docker run -d --name chronosdb -p 8080:8080 -p 50051:50051 chronosdb:latest
        
        Write-Host "ChronosDB running at:" -ForegroundColor Green
        Write-Host "  REST API: http://localhost:8080" -ForegroundColor Cyan
        Write-Host "  gRPC: localhost:50051" -ForegroundColor Cyan
    }
    
    "docker-kafka" {
        Write-Host "Starting ChronosDB with Kafka..." -ForegroundColor Green
        
        if (-not (Test-DockerRunning)) {
            Write-Host "Docker is not running. Please start Docker Desktop first." -ForegroundColor Red
            exit 1
        }
        
        docker-compose -f deployments/docker/docker-compose.yml up -d
        Write-Host "ChronosDB with Kafka running at:" -ForegroundColor Green
        Write-Host "  REST API: http://localhost:8081" -ForegroundColor Cyan
        Write-Host "  Kafka: localhost:9092" -ForegroundColor Cyan
    }
    
    "local" {
        Write-Host "Starting ChronosDB locally..." -ForegroundColor Green
        .\bin\chronosd.exe -rest-port=8080 -grpc-port=50051 -data-dir=./data
    }
    
    default {
        Write-Host "Usage: .\deploy.ps1 -Mode [docker|docker-kafka|local]" -ForegroundColor Yellow
        Write-Host ""
        Write-Host "Examples:" -ForegroundColor White
        Write-Host "  .\deploy.ps1 -Mode local        # Run local binary"
        Write-Host "  .\deploy.ps1 -Mode docker       # Run in Docker"
        Write-Host "  .\deploy.ps1 -Mode docker-kafka # Run with Kafka"
    }
}
