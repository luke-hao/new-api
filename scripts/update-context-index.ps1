[CmdletBinding()]
param(
    [string]$Root,
    [string]$OutDir,
    [switch]$IncludeChatArchive
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Split-Path -Parent $PSScriptRoot
}

$RootPath = (Resolve-Path -LiteralPath $Root).Path

if ([string]::IsNullOrWhiteSpace($OutDir)) {
    $OutDirPath = Join-Path $RootPath "docs\ai-context\generated"
}
elseif ([System.IO.Path]::IsPathRooted($OutDir)) {
    $OutDirPath = $OutDir
}
else {
    $OutDirPath = Join-Path $RootPath $OutDir
}

New-Item -ItemType Directory -Force -Path $OutDirPath | Out-Null
$SnapshotPath = Join-Path $OutDirPath "repo-snapshot.md"

$ExcludedDirectoryNames = @(
    ".git",
    ".idea",
    ".vscode",
    ".zed",
    ".history",
    ".cache",
    ".gocache",
    ".gocache-temp",
    ".gomodcache",
    ".gopath",
    ".claude",
    ".cursor",
    ".playwright-mcp",
    "bin",
    "build",
    "data",
    "dist",
    "logs",
    "node_modules",
    "target",
    "tiktoken_cache",
    "upload",
    "vendor"
)

$ExcludedExtensions = @(
    ".db",
    ".db-journal",
    ".exe"
)

function ConvertTo-RepoRelativePath {
    param([Parameter(Mandatory = $true)][string]$Path)

    $fullPath = [System.IO.Path]::GetFullPath($Path)
    if ($fullPath.StartsWith($RootPath, [System.StringComparison]::OrdinalIgnoreCase)) {
        return ($fullPath.Substring($RootPath.Length).TrimStart([char[]]"\/") -replace "\\", "/")
    }

    return ($fullPath -replace "\\", "/")
}

function Test-IsExcludedRelativePath {
    param([Parameter(Mandatory = $true)][string]$RelativePath)

    $normalized = $RelativePath -replace "\\", "/"
    if ([string]::IsNullOrWhiteSpace($normalized)) {
        return $true
    }

    if ($normalized -like "docs/ai-context/generated/*") {
        return $true
    }

    foreach ($part in ($normalized -split "/")) {
        if ($ExcludedDirectoryNames -contains $part) {
            return $true
        }
    }

    $extension = [System.IO.Path]::GetExtension($normalized)
    return ($ExcludedExtensions -contains $extension)
}

function Get-ProjectFilesWithRg {
    $rg = Get-Command rg -ErrorAction SilentlyContinue
    if (-not $rg) {
        return @()
    }

    $rgArgs = @(
        "--files",
        "--hidden",
        "--no-ignore",
        "-g", "!.git/**",
        "-g", "!**/.git/**",
        "-g", "!**/.idea/**",
        "-g", "!**/.vscode/**",
        "-g", "!**/.zed/**",
        "-g", "!**/.history/**",
        "-g", "!**/.cache/**",
        "-g", "!**/.gocache/**",
        "-g", "!**/.gocache-temp/**",
        "-g", "!**/.gomodcache/**",
        "-g", "!**/.gopath/**",
        "-g", "!**/.claude/**",
        "-g", "!**/.cursor/**",
        "-g", "!**/.playwright-mcp/**",
        "-g", "!**/bin/**",
        "-g", "!**/build/**",
        "-g", "!**/data/**",
        "-g", "!**/dist/**",
        "-g", "!**/logs/**",
        "-g", "!**/node_modules/**",
        "-g", "!**/target/**",
        "-g", "!**/tiktoken_cache/**",
        "-g", "!**/upload/**",
        "-g", "!**/vendor/**",
        "-g", "!docs/ai-context/generated/**",
        "-g", "!*.db",
        "-g", "!**/*.db",
        "-g", "!*.db-journal",
        "-g", "!**/*.db-journal",
        "-g", "!*.exe",
        "-g", "!**/*.exe"
    )

    Push-Location $RootPath
    try {
        $relativePaths = @(& $rg.Source @rgArgs)
    }
    finally {
        Pop-Location
    }

    $files = New-Object "System.Collections.Generic.List[System.IO.FileInfo]"
    foreach ($relativePath in $relativePaths) {
        if (Test-IsExcludedRelativePath -RelativePath $relativePath) {
            continue
        }

        $fullPath = Join-Path $RootPath $relativePath
        if (Test-Path -LiteralPath $fullPath -PathType Leaf) {
            $files.Add((Get-Item -LiteralPath $fullPath))
        }
    }

    return $files
}

function Get-ProjectFilesFallback {
    $files = New-Object "System.Collections.Generic.List[System.IO.FileInfo]"
    $queue = New-Object "System.Collections.Generic.Queue[System.IO.DirectoryInfo]"
    $queue.Enqueue((Get-Item -LiteralPath $RootPath))

    while ($queue.Count -gt 0) {
        $directory = $queue.Dequeue()

        foreach ($filePath in [System.IO.Directory]::EnumerateFiles($directory.FullName)) {
            $relativePath = ConvertTo-RepoRelativePath -Path $filePath
            if (-not (Test-IsExcludedRelativePath -RelativePath $relativePath)) {
                $files.Add((Get-Item -LiteralPath $filePath))
            }
        }

        foreach ($directoryPath in [System.IO.Directory]::EnumerateDirectories($directory.FullName)) {
            $childDirectory = Get-Item -LiteralPath $directoryPath
            if ($ExcludedDirectoryNames -contains $childDirectory.Name) {
                continue
            }

            $relativePath = ConvertTo-RepoRelativePath -Path $childDirectory.FullName
            if (-not (Test-IsExcludedRelativePath -RelativePath $relativePath)) {
                $queue.Enqueue($childDirectory)
            }
        }
    }

    return $files
}

function Get-ProjectFiles {
    $rgFiles = @(Get-ProjectFilesWithRg)
    if ($rgFiles.Count -gt 0) {
        return $rgFiles
    }

    return @(Get-ProjectFilesFallback)
}

function Format-Size {
    param([long]$Bytes)

    if ($Bytes -ge 1GB) {
        return ("{0:N1} GB" -f ($Bytes / 1GB))
    }
    if ($Bytes -ge 1MB) {
        return ("{0:N1} MB" -f ($Bytes / 1MB))
    }
    if ($Bytes -ge 1KB) {
        return ("{0:N1} KB" -f ($Bytes / 1KB))
    }

    return ("{0} B" -f $Bytes)
}

function Escape-InlineCode {
    param([string]$Text)
    return ($Text -replace '`', '``')
}

function Add-Matches {
    param(
        [string]$Title,
        [System.IO.FileInfo[]]$Files,
        [string]$Pattern,
        [int]$Max = 160,
        [int]$MaxPerFile = 30
    )

    Add-Line ("## {0}" -f $Title)
    Add-Line

    if (-not $Files -or $Files.Count -eq 0) {
        Add-Line "_No matching files found._"
        Add-Line
        return
    }

    $totalMatches = 0
    $emittedMatches = 0
    $foundMatches = $false

    foreach ($file in ($Files | Sort-Object FullName)) {
        $fileMatches = @(Select-String -LiteralPath $file.FullName -Pattern $Pattern)
        $totalMatches += $fileMatches.Count

        if ($fileMatches.Count -eq 0) {
            continue
        }

        $foundMatches = $true

        foreach ($match in ($fileMatches | Select-Object -First $MaxPerFile)) {
            if ($emittedMatches -ge $Max) {
                break
            }

            $relativePath = ConvertTo-RepoRelativePath -Path $match.Path
            $line = $match.Line.Trim()
            if ($line.Length -gt 180) {
                $line = $line.Substring(0, 177) + "..."
            }

            Add-Line ('- `{0}:{1}` `{2}`' -f $relativePath, $match.LineNumber, (Escape-InlineCode -Text $line))
            $emittedMatches++
        }

        if ($fileMatches.Count -gt $MaxPerFile -and $emittedMatches -lt $Max) {
            Add-Line ('- ... omitted {0} additional matches from `{1}`.' -f ($fileMatches.Count - $MaxPerFile), (ConvertTo-RepoRelativePath -Path $file.FullName))
        }

        if ($emittedMatches -ge $Max) {
            break
        }
    }

    if (-not $foundMatches) {
        Add-Line "_No matches found._"
        Add-Line
        return
    }

    if ($totalMatches -gt $emittedMatches) {
        Add-Line ("- ... omitted {0} additional matches." -f ($totalMatches - $emittedMatches))
    }

    Add-Line
}

$AllFiles = @(Get-ProjectFiles | Sort-Object FullName -Unique)

$Lines = New-Object "System.Collections.Generic.List[string]"
function Add-Line {
    param([string]$Line = "")
    $script:Lines.Add($Line)
}

Add-Line "# Repo Snapshot"
Add-Line
Add-Line '> Generated by `scripts/update-context-index.ps1`. Do not edit this file manually.'
Add-Line
Add-Line ('- Root: `{0}`' -f ($RootPath -replace "\\", "/"))
Add-Line ('- Generated: `{0}`' -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss zzz'))
Add-Line ('- Indexed files: `{0}`' -f $AllFiles.Count)
Add-Line

Add-Line "## Top-Level File Counts"
Add-Line
Add-Line "| Files | Top-level path |"
Add-Line "| ---: | --- |"
$topLevelStats = $AllFiles |
    Group-Object { ((ConvertTo-RepoRelativePath -Path $_.FullName) -split "/")[0] } |
    Sort-Object Count, Name -Descending

foreach ($group in $topLevelStats) {
    Add-Line ('| {0} | `{1}` |' -f $group.Count, $group.Name)
}
Add-Line

Add-Line "## Extension Counts"
Add-Line
Add-Line "| Files | Extension |"
Add-Line "| ---: | --- |"
$extensionStats = $AllFiles |
    Group-Object {
        if ([string]::IsNullOrWhiteSpace($_.Extension)) {
            "(none)"
        }
        else {
            $_.Extension
        }
    } |
    Sort-Object Count, Name -Descending |
    Select-Object -First 40

foreach ($group in $extensionStats) {
    Add-Line ('| {0} | `{1}` |' -f $group.Count, $group.Name)
}
Add-Line

$routerDir = Join-Path $RootPath "router"
$routerFiles = @()
if (Test-Path -LiteralPath $routerDir -PathType Container) {
    $routerFiles = @(Get-ChildItem -LiteralPath $routerDir -File -Filter "*.go")
}
Add-Matches `
    -Title "Backend Router Summary" `
    -Files $routerFiles `
    -Pattern "func Set.*Router|Group\(|\.(GET|POST|PUT|PATCH|DELETE|OPTIONS)\("

$routesDir = Join-Path $RootPath "web\default\src\routes"
$routeFiles = @()
if (Test-Path -LiteralPath $routesDir -PathType Container) {
    $routeFiles = @(Get-ChildItem -LiteralPath $routesDir -Recurse -File -Include "*.ts", "*.tsx")
}
Add-Matches `
    -Title "Frontend Route Summary" `
    -Files $routeFiles `
    -Pattern "createFileRoute|createRootRoute"

Add-Line "## Relay Channels"
Add-Line
$relayChannelDir = Join-Path $RootPath "relay\channel"
if (Test-Path -LiteralPath $relayChannelDir -PathType Container) {
    $channelDirs = @(Get-ChildItem -LiteralPath $relayChannelDir -Directory | Sort-Object Name)
    foreach ($channelDir in $channelDirs) {
        Add-Line ('- `{0}`' -f $channelDir.Name)
    }
}
else {
    Add-Line "_No relay channel directory found._"
}
Add-Line

Add-Line "## Frontend Features"
Add-Line
$featureDir = Join-Path $RootPath "web\default\src\features"
if (Test-Path -LiteralPath $featureDir -PathType Container) {
    $featureDirs = @(Get-ChildItem -LiteralPath $featureDir -Directory | Sort-Object Name)
    foreach ($feature in $featureDirs) {
        Add-Line ('- `{0}`' -f $feature.Name)
    }
}
else {
    Add-Line "_No frontend feature directory found._"
}
Add-Line

Add-Line "## Docs"
Add-Line
$docsDir = Join-Path $RootPath "docs"
if (Test-Path -LiteralPath $docsDir -PathType Container) {
    $docsFiles = @(
        Get-ChildItem -LiteralPath $docsDir -Recurse -File -Filter "*.md" |
            Where-Object {
                (ConvertTo-RepoRelativePath -Path $_.FullName) -notlike "docs/ai-context/generated/*"
            } |
            Sort-Object FullName
    )
    foreach ($docFile in $docsFiles) {
        Add-Line ('- `{0}`' -f (ConvertTo-RepoRelativePath -Path $docFile.FullName))
    }
}
else {
    Add-Line "_No docs directory found._"
}
Add-Line

Add-Line "## Chat Archive Metadata"
Add-Line
$archiveDir = Join-Path (Split-Path -Parent $RootPath) ".chat-archive"
if (Test-Path -LiteralPath $archiveDir -PathType Container) {
    $archiveFiles = @(Get-ChildItem -LiteralPath $archiveDir -File -Filter "*.md" | Sort-Object LastWriteTime -Descending)
    $archiveSize = ($archiveFiles | Measure-Object -Property Length -Sum).Sum
    if ($null -eq $archiveSize) {
        $archiveSize = 0
    }

    Add-Line ('- Path: `{0}`' -f ($archiveDir -replace "\\", "/"))
    Add-Line ('- Files: `{0}`' -f $archiveFiles.Count)
    Add-Line ('- Total size: `{0}`' -f (Format-Size -Bytes ([long]$archiveSize)))

    if ($IncludeChatArchive) {
        Add-Line
        Add-Line "| Last write | Size | File |"
        Add-Line "| --- | ---: | --- |"
        foreach ($archiveFile in ($archiveFiles | Select-Object -First 50)) {
            Add-Line ('| {0} | {1} | `{2}` |' -f $archiveFile.LastWriteTime.ToString('yyyy-MM-dd HH:mm:ss'), (Format-Size -Bytes $archiveFile.Length), $archiveFile.Name)
        }
        if ($archiveFiles.Count -gt 50) {
            Add-Line ("| ... | ... | omitted {0} older files |" -f ($archiveFiles.Count - 50))
        }
    }
    else {
        Add-Line '- Per-file metadata omitted. Run with `-IncludeChatArchive` to list file names and sizes.'
    }
}
else {
    Add-Line '_No sibling `.chat-archive` directory found._'
}
Add-Line

Add-Line "## Refresh Notes"
Add-Line
Add-Line '- Generated files under `docs/ai-context/generated/` are excluded from the scan to avoid self-referential churn.'
Add-Line '- Binary/runtime-heavy paths such as `node_modules`, `logs`, `data`, `*.db`, and `*.exe` are excluded.'
Add-Line "- Use this snapshot as a pointer map, then verify behavior in the source files."

$utf8NoBom = New-Object System.Text.UTF8Encoding $false
[System.IO.File]::WriteAllLines($SnapshotPath, [string[]]$Lines, $utf8NoBom)

Write-Host "Context index updated:" -ForegroundColor Green
Write-Host $SnapshotPath
