<#
.SYNOPSIS
    Runs all static analysis, unit/concurrency tests, vulnerability, and security scans for sshm.
    Copyright © 2026 Oktanetflow
#>
[CmdletBinding()]
param(
    [switch]$SkipGosec,
    [switch]$SkipGovulncheck
)

$ErrorActionPreference = "Stop"

Write-Host "==> [1/4] Running go vet ./..." -ForegroundColor Cyan
go vet ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "==> [2/4] Running go test ./..." -ForegroundColor Cyan
$hasCgoCompiler = [bool](Get-Command gcc -ErrorAction SilentlyContinue)
if ($hasCgoCompiler) {
    go test -race -count=1 ./...
} else {
    Write-Host "    (gcc not found on Windows PATH; running go test without -race; CI runs with race detector)" -ForegroundColor Yellow
    go test -count=1 ./...
}
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

if (-not $SkipGovulncheck) {
    Write-Host "==> [3/4] Running govulncheck ./..." -ForegroundColor Cyan
    go run golang.org/x/vuln/cmd/govulncheck@latest ./...
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if (-not $SkipGosec) {
    Write-Host "==> [4/4] Running gosec ./..." -ForegroundColor Cyan
    go run github.com/securego/gosec/v2/cmd/gosec@latest -severity medium ./...
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

Write-Host "`nAll security & quality gates passed!" -ForegroundColor Green
