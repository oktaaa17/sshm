<#
.SYNOPSIS
    Signs Windows release binaries (sshm.exe and installer EXE) with Authenticode + RFC 3161 timestamp.
    Copyright © 2026 Oktanetflow

.DESCRIPTION
    Supports two production signing modes so binaries run cleanly under Windows Defender & SmartScreen
    without asking users to disable Windows Security:
      1. PFX certificate (-PfxPath + -PfxPassword or env AUTHENTICODE_PFX_PASSWORD)
      2. Certificate Thumbprint in Windows Cert Store (-Thumbprint)

    For Microsoft Trusted Signing in GitHub Actions, see .github/workflows/release.yml and
    docs/WINDOWS_PUBLISHER_GUIDE.md.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string[]]$Files,

    [string]$PfxPath = $env:AUTHENTICODE_PFX_PATH,
    [string]$PfxPassword = $env:AUTHENTICODE_PFX_PASSWORD,
    [string]$Thumbprint = $env:AUTHENTICODE_THUMBPRINT,
    [string]$TimestampUrl = "http://timestamp.digicert.com",
    [string]$Description = "sshm — Minimalist CLI/TUI SSH Client by Oktanetflow",
    [string]$DescriptionUrl = "https://github.com/Oktanetflow/sshm"
)

$ErrorActionPreference = "Stop"

function Find-SignTool {
    $cmd = Get-Command signtool.exe -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    $kits = Get-ChildItem "C:\Program Files (x86)\Windows Kits\10\bin\*\x64\signtool.exe" -ErrorAction SilentlyContinue |
        Sort-Object FullName -Descending | Select-Object -First 1
    if ($kits) { return $kits.FullName }
    throw "signtool.exe not found. Install Windows 10/11 SDK or run in GitHub Actions windows-latest."
}

$signtool = Find-SignTool

foreach ($file in $Files) {
    if (-not (Test-Path $file)) {
        throw "File not found for signing: $file"
    }
    Write-Host "Signing $file ..." -ForegroundColor Cyan
    $argsList = @(
        "sign",
        "/fd", "SHA256",
        "/tr", $TimestampUrl,
        "/td", "SHA256",
        "/d", $Description,
        "/du", $DescriptionUrl
    )

    if ($PfxPath) {
        $argsList += @("/f", $PfxPath)
        if ($PfxPassword) {
            $argsList += @("/p", $PfxPassword)
        }
    } elseif ($Thumbprint) {
        $argsList += @("/sha1", $Thumbprint)
    } else {
        throw "Provide either -PfxPath (with optional -PfxPassword) or -Thumbprint."
    }

    $argsList += $file
    & $signtool @argsList
    if ($LASTEXITCODE -ne 0) {
        throw "signtool failed with exit code $LASTEXITCODE for $file"
    }

    $sig = Get-AuthenticodeSignature -FilePath $file
    Write-Host ("  Status : {0} | Subject: {1}" -f $sig.Status, $sig.SignerCertificate.Subject) -ForegroundColor Green
}
