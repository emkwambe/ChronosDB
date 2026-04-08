# build-docker.ps1 - Build Docker image on Windows

param(
    [string]$Version = "latest"
)

Write-Host "Building ChronosDB Docker image..." -ForegroundColor Green

# Check if Docker is running
try {
    docker ps | Out-Null
} catch {
    Write-Host "Docker is not running. Please start Docker Desktop first." -ForegroundColor Red
    exit 1
}

# Build the image
docker build -f deployments/docker/Dockerfile -t chronosdb/chronosdb:$Version .

if ($LASTEXITCODE -eq 0) {
    Write-Host "Image built successfully: chronosdb/chronosdb:$Version" -ForegroundColor Green
    Write-Host ""
    Write-Host "Run with: docker run -d --name chronosdb -p 8080:8080 chronosdb/chronosdb:$Version" -ForegroundColor Cyan
} else {
    Write-Host "Build failed!" -ForegroundColor Red
}
