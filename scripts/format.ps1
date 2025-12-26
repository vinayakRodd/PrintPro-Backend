# Format script for Print Pro Backend (PowerShell)
# Formats both Go code and non-Go files (markdown, JSON, etc.)

Write-Host "🔧 Formatting code..." -ForegroundColor Cyan

# Format Go files
Write-Host "📝 Formatting Go files..." -ForegroundColor Yellow
go fmt ./...

# Format non-Go files with Prettier (if pnpm is available)
if (Get-Command pnpm -ErrorAction SilentlyContinue) {
    Write-Host "✨ Formatting markdown, JSON, and other files with Prettier..." -ForegroundColor Yellow
    pnpm format
} else {
    Write-Host "⚠️  pnpm not found. Install pnpm to format non-Go files." -ForegroundColor Yellow
    Write-Host "   Run: npm install -g pnpm" -ForegroundColor Yellow
}

Write-Host "✅ Formatting complete!" -ForegroundColor Green

