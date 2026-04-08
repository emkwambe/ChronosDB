# test_runner.ps1 - Run all E2E tests

param(
    [string]$TestType = "all"
)

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "ChronosDB End-to-End Test Suite" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Check if ChronosDB is running
Write-Host "Checking ChronosDB status..." -ForegroundColor Yellow
try {
    $health = Invoke-RestMethod -Uri "http://localhost:8080/v1/db/test/health" -Method GET -ErrorAction Stop
    Write-Host "✓ ChronosDB is running" -ForegroundColor Green
} catch {
    Write-Host "✗ ChronosDB is not running. Please start ChronosDB first." -ForegroundColor Red
    Write-Host "Run: .\bin\chronosd.exe -rest-port=8080 -data-dir=./data" -ForegroundColor Yellow
    exit 1
}

Write-Host ""

switch ($TestType) {
    "all" {
        Write-Host "Running all tests..." -ForegroundColor Yellow
        
        # Generate synthetic data
        Write-Host "Generating synthetic data..." -ForegroundColor Yellow
        go run test/e2e/data_generator.go
        
        # Run E2E tests
        Write-Host "Running E2E tests..." -ForegroundColor Yellow
        go test -v ./test/e2e/ -run TestEndToEnd
        
        # Run load tests
        Write-Host "Running load tests..." -ForegroundColor Yellow
        go run test/e2e/load_test.go
    }
    "e2e" {
        Write-Host "Running E2E tests only..." -ForegroundColor Yellow
        go test -v ./test/e2e/ -run TestEndToEnd
    }
    "load" {
        Write-Host "Running load tests only..." -ForegroundColor Yellow
        go run test/e2e/load_test.go
    }
    "synthetic" {
        Write-Host "Generating synthetic data only..." -ForegroundColor Yellow
        go run test/e2e/data_generator.go
    }
    default {
        Write-Host "Unknown test type. Options: all, e2e, load, synthetic" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Test suite completed!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Cyan
