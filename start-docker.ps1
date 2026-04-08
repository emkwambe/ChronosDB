# Start Docker Desktop on Windows
# Run this script with admin privileges

Write-Host "Starting Docker Desktop..." -ForegroundColor Green

# Path to Docker Desktop executable
$dockerDesktopPath = "C:\Program Files\Docker\Docker\Docker.exe"

# Check if Docker Desktop is already running
$dockerProcess = Get-Process "Docker Desktop" -ErrorAction SilentlyContinue

if ($dockerProcess) {
    Write-Host "Docker Desktop is already running." -ForegroundColor Yellow
} else {
    # Start Docker Desktop
    if (Test-Path $dockerDesktopPath) {
        Start-Process $dockerDesktopPath
        Write-Host "Docker Desktop is starting..." -ForegroundColor Green
        Write-Host "Waiting for Docker daemon to be ready (this may take 30-60 seconds)..." -ForegroundColor Cyan
        
        # Wait up to 60 seconds for Docker to be ready
        $maxRetries = 60
        $retries = 0
        
        while ($retries -lt $maxRetries) {
            try {
                $dockerInfo = docker info 2>$null
                if ($?) {
                    Write-Host "Docker is now active and ready!" -ForegroundColor Green
                    docker ps
                    break
                }
            } catch {
                # Docker not ready yet
            }
            
            $retries++
            Start-Sleep -Seconds 1
            
            if ($retries % 10 -eq 0) {
                Write-Host "Still waiting... ($retries seconds elapsed)" -ForegroundColor Yellow
            }
        }
        
        if ($retries -eq $maxRetries) {
            Write-Host "Docker startup timed out. Please check Docker Desktop manually." -ForegroundColor Red
        }
    } else {
        Write-Host "Docker Desktop not found at: $dockerDesktopPath" -ForegroundColor Red
        Write-Host "Please install Docker Desktop or verify the installation path." -ForegroundColor Yellow
    }
}

# Show final status
Write-Host "`nChecking Docker status:" -ForegroundColor Green
docker --version
docker ps
