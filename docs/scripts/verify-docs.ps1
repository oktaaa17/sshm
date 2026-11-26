# Automated Documentation, ADR & Media Asset Verifier for sshm (Oktanetflow)
# Usage: powershell -ExecutionPolicy Bypass -File docs/scripts/verify-docs.ps1

$ErrorActionPreference = "Stop"
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$Failures = New-Object System.Collections.Generic.List[string]

function Add-Failure([string]$msg) {
    $Failures.Add($msg)
    Write-Host "  [FAIL] $msg" -ForegroundColor Red
}

Write-Host "=== 1. Verifying Local Markdown Links & Image Paths ===" -ForegroundColor Cyan
$mdFiles = @()
foreach ($top in @("README.md", "CLAUDE.md")) {
    $p = Join-Path $RepoRoot $top
    if (Test-Path $p) { $mdFiles += Get-Item $p }
}
$mdFiles += Get-ChildItem -Path (Join-Path $RepoRoot "docs") -Recurse -Filter "*.md"

$linkRegex = [regex]'!?\[[^\]]*\]\(([^)]+)\)'
$checkedLinks = 0
foreach ($file in $mdFiles) {
    $content = Get-Content -Path $file.FullName -Raw
    foreach ($m in $linkRegex.Matches($content)) {
        $rawTarget = $m.Groups[1].Value.Trim()
        if ($rawTarget -match '^(https?://|mailto:|#|file://)') { continue }
        $cleanTarget = ($rawTarget -split '#')[0].Trim()
        if ([string]::IsNullOrWhiteSpace($cleanTarget)) { continue }
        $resolved = Join-Path $file.DirectoryName $cleanTarget
        $checkedLinks++
        if (-not (Test-Path $resolved)) {
            $relSrc = $file.FullName.Substring($RepoRoot.Length + 1)
            Add-Failure "Broken link in $relSrc -> $rawTarget"
        }
    }
}
Write-Host "  Checked $checkedLinks local links across $($mdFiles.Count) Markdown files." -ForegroundColor Green

Write-Host "`n=== 2. Verifying ADR Structure (docs/decisions/ADR-*.md) ===" -ForegroundColor Cyan
$adrFiles = Get-ChildItem -Path (Join-Path $RepoRoot "docs\decisions") -Filter "ADR-*.md" | Sort-Object Name
if ($adrFiles.Count -lt 6) {
    Add-Failure "Expected at least 6 ADRs, found $($adrFiles.Count)"
}
$requiredSections = @("## Status", "## Date", "## Context", "## Decision", "## Alternatives Considered", "## Consequences")
foreach ($adr in $adrFiles) {
    $txt = Get-Content -Path $adr.FullName -Raw
    foreach ($sec in $requiredSections) {
        if ($txt -notmatch [regex]::Escape($sec)) {
            Add-Failure "$($adr.Name) missing required section: $sec"
        }
    }
}
Write-Host "  Verified $($adrFiles.Count) ADRs for all required sections." -ForegroundColor Green

Write-Host "`n=== 3. Verifying Binary Media Headers & Size Budgets ===" -ForegroundColor Cyan
$expectedAssets = @(
    @{ Path = "docs\assets\illustrations\hero-banner.png"; Type = "PNG"; MaxMB = 5 },
    @{ Path = "docs\assets\illustrations\security-credential-vault.png"; Type = "PNG"; MaxMB = 5 },
    @{ Path = "docs\assets\illustrations\network-proxyjump-tunnels.png"; Type = "PNG"; MaxMB = 5 },
    @{ Path = "docs\assets\illustrations\architecture-overview.svg"; Type = "SVG"; MaxMB = 1 },
    @{ Path = "docs\assets\illustrations\windows-publisher-trust.svg"; Type = "SVG"; MaxMB = 1 },
    @{ Path = "docs\assets\mockups\tui-minimal-dark.png"; Type = "PNG"; MaxMB = 3 },
    @{ Path = "docs\assets\mockups\tui-minimal-light.png"; Type = "PNG"; MaxMB = 3 },
    @{ Path = "docs\assets\mockups\tui-mono.png"; Type = "PNG"; MaxMB = 3 },
    @{ Path = "docs\assets\mockups\tui-add-host-modal.png"; Type = "PNG"; MaxMB = 3 },
    @{ Path = "docs\assets\mockups\tui-port-forwards-modal.png"; Type = "PNG"; MaxMB = 3 },
    @{ Path = "docs\assets\mockups\tui-help-group-modal.png"; Type = "PNG"; MaxMB = 3 },
    @{ Path = "docs\assets\videos\demo-tui-agile-workflow.gif"; Type = "GIF"; MaxMB = 4 },
    @{ Path = "docs\assets\videos\demo-tui-agile-workflow.webm"; Type = "WEBM"; MaxMB = 5 },
    @{ Path = "docs\assets\videos\demo-add-host-wincred-connect.gif"; Type = "GIF"; MaxMB = 4 },
    @{ Path = "docs\assets\videos\demo-add-host-wincred-connect.webm"; Type = "WEBM"; MaxMB = 5 },
    @{ Path = "docs\assets\videos\demo-tunnels-batch-sftp.gif"; Type = "GIF"; MaxMB = 4 },
    @{ Path = "docs\assets\videos\demo-tunnels-batch-sftp.webm"; Type = "WEBM"; MaxMB = 5 }
)

foreach ($asset in $expectedAssets) {
    $full = Join-Path $RepoRoot $asset.Path
    if (-not (Test-Path $full)) {
        Add-Failure "Missing asset: $($asset.Path)"
        continue
    }
    $len = (Get-Item $full).Length
    if ($len -lt 100) {
        Add-Failure "Asset too small / empty: $($asset.Path) ($len bytes)"
        continue
    }
    if ($len -gt ($asset.MaxMB * 1024 * 1024)) {
        Add-Failure "Asset exceeds $($asset.MaxMB) MB budget: $($asset.Path) ($([math]::Round($len/1MB,2)) MB)"
    }
    $bytes = [System.IO.File]::ReadAllBytes($full)
    switch ($asset.Type) {
        "PNG" {
            if ($bytes[0] -ne 0x89 -or $bytes[1] -ne 0x50 -or $bytes[2] -ne 0x4E -or $bytes[3] -ne 0x47) {
                Add-Failure "Invalid PNG header: $($asset.Path)"
            }
        }
        "GIF" {
            $hdr = [System.Text.Encoding]::ASCII.GetString($bytes, 0, 6)
            if ($hdr -ne "GIF89a") {
                Add-Failure "Invalid GIF89a header ($hdr): $($asset.Path)"
            }
        }
        "WEBM" {
            if ($bytes[0] -ne 0x1A -or $bytes[1] -ne 0x45 -or $bytes[2] -ne 0xDF -or $bytes[3] -ne 0xA3) {
                Add-Failure "Invalid WebM/EBML header: $($asset.Path)"
            }
        }
        "SVG" {
            $txt = [System.Text.Encoding]::UTF8.GetString($bytes)
            if ($txt -notmatch '<svg\b') {
                Add-Failure "Invalid SVG root element: $($asset.Path)"
            }
        }
    }
    Write-Host ("  [OK] {0,-58} {1,9:N0} bytes ({2})" -f $asset.Path, $len, $asset.Type) -ForegroundColor Gray
}

if ($Failures.Count -gt 0) {
    Write-Host "`nVerification FAILED with $($Failures.Count) issue(s)." -ForegroundColor Red
    exit 1
}

Write-Host "`nALL DOCUMENTATION, ADR, AND MEDIA CHECKS PASSED!" -ForegroundColor Green
exit 0
