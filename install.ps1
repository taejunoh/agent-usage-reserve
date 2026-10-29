$ErrorActionPreference = 'Stop'
$repo = 'taejunoh/agent-usage-reserve'
$version = $env:AGENT_USAGE_RESERVE_VERSION
if (-not $version) {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest"
    $tag = [string]$release.tag_name
} else {
    $tag = $version
}
if ($tag -notmatch '^v?([0-9]+)\.([0-9]+)\.([0-9]+)$') {
    throw "Invalid release version: $tag"
}
$version = $tag -replace '^v', ''
$tag = "v$version"
$arch = if ($env:PROCESSOR_ARCHITECTURE -match 'ARM64') { 'arm64' } else { 'amd64' }
$archive = "agent-usage-reserve_$($version)_windows_$($arch).tar.gz"
$base = if ($env:AGENT_USAGE_RESERVE_RELEASE_BASE_URL) {
    "$($env:AGENT_USAGE_RESERVE_RELEASE_BASE_URL.TrimEnd('/'))/$tag"
} else {
    "https://github.com/$repo/releases/download/$tag"
}
$installDir = if ($env:AGENT_USAGE_RESERVE_INSTALL_DIR) {
    $env:AGENT_USAGE_RESERVE_INSTALL_DIR
} else {
    Join-Path $env:LOCALAPPDATA 'Programs/agent-usage-reserve/bin'
}
$temp = Join-Path ([IO.Path]::GetTempPath()) ("agent-usage-reserve-" + [guid]::NewGuid())
$stage = Join-Path $temp 'unpacked'
New-Item -ItemType Directory -Path $stage -Force | Out-Null
try {
    $archivePath = Join-Path $temp $archive
    $checksumsPath = Join-Path $temp 'checksums.txt'
    Invoke-WebRequest -Uri "$base/$archive" -OutFile $archivePath
    Invoke-WebRequest -Uri "$base/checksums.txt" -OutFile $checksumsPath
    $checksumEntries = @(Get-Content $checksumsPath | Where-Object { $_ -match "^\s*([0-9a-fA-F]{64})\s+$([regex]::Escape($archive))\s*$" })
    if ($checksumEntries.Count -ne 1) {
        throw "No unique SHA-256 checksum found for $archive"
    }
    $expected = [regex]::Match($checksumEntries[0], '^[\s]*([0-9a-fA-F]{64})').Groups[1].Value.ToLowerInvariant()
    $actual = (Get-FileHash -Path $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) {
        throw "SHA-256 mismatch for $archive"
    }
    & tar.exe -xzf $archivePath -C $stage 'agent-usage-reserve.exe'
    if ($LASTEXITCODE -ne 0) {
        throw 'Could not extract the release executable.'
    }
    $staged = Join-Path $stage 'agent-usage-reserve.exe'
    if (-not (Test-Path -LiteralPath $staged -PathType Leaf)) {
        throw 'Release archive did not contain agent-usage-reserve.exe.'
    }
    $reportedVersion = (& $staged --version | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $reportedVersion -ne $version) {
        throw "Release binary version does not match tag $tag"
    }
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    $target = Join-Path $installDir 'agent-usage-reserve.exe'
    Move-Item -LiteralPath $staged -Destination $target -Force

    # The old executable cannot see the new marker and would add a second policy block.
    $removedDirs = @($installDir)
    $legacyDir = $null
    if (-not $env:AGENT_USAGE_RESERVE_INSTALL_DIR -and $env:LOCALAPPDATA) {
        # The default install directory changed name, so the old default one is cleaned too.
        $legacyDir = Join-Path $env:LOCALAPPDATA 'Programs/codex-usage-reserve/bin'
        $removedDirs += $legacyDir
    }
    foreach ($dir in $removedDirs) {
        $old = Join-Path $dir 'codex-usage-reserve.exe'
        $oldItem = Get-Item -LiteralPath $old -Force -ErrorAction SilentlyContinue
        if ($oldItem -and -not $oldItem.PSIsContainer -and -not $oldItem.LinkType) {
            Remove-Item -LiteralPath $old -Force
            Write-Output "Removed old codex-usage-reserve: $old"
            if ($dir -eq $legacyDir) {
                Write-Output "You can remove $legacyDir from your user PATH."
            }
        }
    }

    # The binary reads the agents' own home variables itself; passing them as flags would
    # force-select that agent and skip detection of the other one.
    $enableArgs = @('enable')
    if ($env:AGENT_USAGE_RESERVE_CODEX_HOME) {
        $enableArgs += @('--codex-home', $env:AGENT_USAGE_RESERVE_CODEX_HOME)
    }
    if ($env:AGENT_USAGE_RESERVE_CLAUDE_HOME) {
        $enableArgs += @('--claude-home', $env:AGENT_USAGE_RESERVE_CLAUDE_HOME)
    }
    # The console is used directly, so the binary can prompt when both agents are found.
    # Windows PowerShell 5.1 turns native stderr into a terminating error under Stop.
    $previousPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        & $target @enableArgs
        $enableExit = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousPreference
    }
    if ($enableExit -eq 0) {
        Write-Output "Installed and enabled: $target"
    } elseif ($enableExit -eq 3) {
        Write-Output "Installed (policy not enabled yet): $target"
    } else {
        [Console]::Error.WriteLine("Installed $target, but enabling the policy failed (see the error above).")
        throw "Policy enable failed with exit code $enableExit"
    }
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (($userPath -split ';') -notcontains $installDir) {
        Write-Output "Add to your user PATH: $installDir"
        Write-Output ('$env:Path = "' + $installDir + ';' + '$env:Path"')
    }
} finally {
    Remove-Item -LiteralPath $temp -Recurse -Force -ErrorAction SilentlyContinue
}
