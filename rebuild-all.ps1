[CmdletBinding()]
param(
    [string]$Output = "new-new-api.exe",
    [switch]$Classic,
    [switch]$SkipTypeCheck,
    [switch]$StopRunning
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$Root = $PSScriptRoot
$DefaultWebDir = Join-Path $Root "web\default"
$ClassicWebDir = Join-Path $Root "web\classic"
$TargetExe = Join-Path $Root $Output

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Require-File {
    param(
        [string]$Path,
        [string]$Hint
    )
    if (-not (Test-Path -LiteralPath $Path)) {
        throw "Missing file: $Path`n$Hint"
    }
}

function Invoke-Native {
    param(
        [string]$Name,
        [string]$FilePath,
        [string[]]$Arguments,
        [string]$WorkingDirectory
    )

    Write-Step $Name
    Push-Location $WorkingDirectory
    try {
        & $FilePath @Arguments
        $exitCode = $LASTEXITCODE
        if ($exitCode -ne 0) {
            throw "$Name failed with exit code $exitCode"
        }
    }
    finally {
        Pop-Location
    }
}

function Get-RunningTargetProcesses {
    if (-not (Test-Path -LiteralPath $TargetExe)) {
        return @()
    }

    $targetPath = (Resolve-Path -LiteralPath $TargetExe).Path
    return @(Get-Process -ErrorAction SilentlyContinue | Where-Object {
        try {
            $_.Path -eq $targetPath
        }
        catch {
            $false
        }
    })
}

function Ensure-TargetNotRunning {
    $processes = @(Get-RunningTargetProcesses)
    if ($processes.Count -eq 0) {
        return
    }

    Write-Host ""
    Write-Host "The target executable is currently running:" -ForegroundColor Yellow
    $processes | Select-Object Id, ProcessName, Path | Format-Table -AutoSize

    if ($StopRunning) {
        Write-Step "Stopping running target process"
        foreach ($process in $processes) {
            Stop-Process -Id $process.Id -Force
        }
        Start-Sleep -Milliseconds 500
        return
    }

    $answer = Read-Host "Stop it now so the new exe can be written? [y/N]"
    if ($answer -match '^(y|yes)$') {
        Write-Step "Stopping running target process"
        foreach ($process in $processes) {
            Stop-Process -Id $process.Id -Force
        }
        Start-Sleep -Milliseconds 500
        return
    }

    throw "Build cancelled because $Output is running. Close it and run this script again."
}

Write-Host "New API rebuild script" -ForegroundColor Green
Write-Host "Project: $Root"
Write-Host "Output : $TargetExe"

$DefaultTsc = Join-Path $DefaultWebDir "node_modules\.bin\tsc.CMD"
$DefaultRsbuild = Join-Path $DefaultWebDir "node_modules\.bin\rsbuild.CMD"
$ClassicRsbuild = Join-Path $ClassicWebDir "node_modules\.bin\rsbuild.CMD"

Require-File -Path $DefaultRsbuild -Hint "Run dependency installation for web/default first."

if (-not $SkipTypeCheck) {
    Require-File -Path $DefaultTsc -Hint "Run dependency installation for web/default first."
    Invoke-Native -Name "Default frontend type check" -FilePath $DefaultTsc -Arguments @("-b") -WorkingDirectory $DefaultWebDir
}

Invoke-Native -Name "Default frontend build" -FilePath $DefaultRsbuild -Arguments @("build") -WorkingDirectory $DefaultWebDir

if ($Classic) {
    Require-File -Path $ClassicRsbuild -Hint "Run dependency installation for web/classic first, or omit -Classic."
    Invoke-Native -Name "Classic frontend build" -FilePath $ClassicRsbuild -Arguments @("build") -WorkingDirectory $ClassicWebDir
}

Ensure-TargetNotRunning

Invoke-Native -Name "Backend build" -FilePath "go" -Arguments @("build", "-o", $TargetExe, ".") -WorkingDirectory $Root

Write-Host ""
Write-Host "Build completed successfully." -ForegroundColor Green
Write-Host "Executable: $TargetExe"
