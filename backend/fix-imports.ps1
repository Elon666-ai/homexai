# HomeX Backend - Fix Import Paths Script (Windows PowerShell)
# This script fixes any incorrect import paths in Go files

Write-Host "🔧 Fixing import paths in HomeX backend..." -ForegroundColor Cyan
Write-Host ""

$ErrorCount = 0
$FixedCount = 0

# Get all Go files recursively
Get-ChildItem -Recurse -Filter *.go | ForEach-Object {
    $file = $_.FullName
    $content = Get-Content $file -Raw -ErrorAction SilentlyContinue
    
    if ($content -and $content -match '"homexai/') {
        Write-Host "Fixing: $($_.FullName)" -ForegroundColor Yellow
        
        try {
            # Replace homexai with correct module path
            $newContent = $content -replace '"homexai/', '"homexai/'
            Set-Content -Path $file -Value $newContent -NoNewline
            $FixedCount++
        }
        catch {
            Write-Host "Error fixing $($_.FullName): $_" -ForegroundColor Red
            $ErrorCount++
        }
    }
}

Write-Host ""
if ($FixedCount -gt 0) {
    Write-Host "✅ Fixed $FixedCount file(s)!" -ForegroundColor Green
}
else {
    Write-Host "✅ No files needed fixing!" -ForegroundColor Green
}

if ($ErrorCount -gt 0) {
    Write-Host "❌ $ErrorCount error(s) occurred!" -ForegroundColor Red
}

Write-Host ""
Write-Host "Next steps:" -ForegroundColor Cyan
Write-Host "  1. go mod tidy"
Write-Host "  2. make build (or: go build -o bin/homex-api.exe ./cmd/api/main.go)"
