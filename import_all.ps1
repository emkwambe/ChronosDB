# import_all.ps1 - One-click import all data

Write-Host "ChronosDB Data Import Tool" -ForegroundColor Cyan
Write-Host "=========================" -ForegroundColor Cyan

# Check ChronosDB
try {
    $health = Invoke-RestMethod -Uri "http://localhost:8080/v1/db/test/health" -Method GET -ErrorAction Stop
    Write-Host "✅ ChronosDB is running" -ForegroundColor Green
} catch {
    Write-Host "❌ ChronosDB is not running. Please start it first." -ForegroundColor Red
    exit 1
}

# Import CSV
Write-Host "`n1. Importing CSV..." -ForegroundColor Yellow
& .\bin\importer.exe -file test/data/sample.csv -format csv -label Customer -timestamp timestamp

# Import JSON
Write-Host "`n2. Importing JSON..." -ForegroundColor Yellow
& .\bin\importer.exe -file test/data/sample.json -format json -label Person

# Import SQL
Write-Host "`n3. Importing SQL..." -ForegroundColor Yellow
& .\bin\importer.exe -file test/data/sample.sql -format sql -label User

Write-Host "`n✅ All imports completed!" -ForegroundColor Green

# Show summary
Write-Host "`n=== Import Summary ===" -ForegroundColor Cyan
$body = @{query="MATCH (n) RETURN labels(n) as type, count(n) as count"} | ConvertTo-Json
Invoke-RestMethod -Uri "http://localhost:8080/v1/db/test/query" -Method POST -Body $body -ContentType "application/json"
