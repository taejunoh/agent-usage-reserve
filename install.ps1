$ErrorActionPreference = 'Stop'
$repo = 'taejunoh/codex-usage-reserve'
$version = $env:CODEX_USAGE_RESERVE_VERSION
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
$archive = "codex-usage-reserve_$($version)_windows_$($arch).tar.gz"
$base = if ($env:CODEX_USAGE_RESERVE_RELEASE_BASE_URL) {
    "$($env:CODEX_USAGE_RESERVE_RELEASE_BASE_URL.TrimEnd('/'))/$tag"
} else {
    "https://github.com/$repo/releases/download/$tag"
}
$installDir = if ($env:CODEX_USAGE_RESERVE_INSTALL_DIR) {
    $env:CODEX_USAGE_RESERVE_INSTALL_DIR
} else {
    Join-Path $env:LOCALAPPDATA 'Programs/codex-usage-reserve/bin'
}
$temp = Join-Path ([IO.Path]::GetTempPath()) ("codex-usage-reserve-" + [guid]::NewGuid())
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
    & tar.exe -xzf $archivePath -C $stage 'codex-usage-reserve.exe'
    if ($LASTEXITCODE -ne 0) {
        throw 'Could not extract the release executable.'
    }
    $staged = Join-Path $stage 'codex-usage-reserve.exe'
    if (-not (Test-Path -LiteralPath $staged -PathType Leaf)) {
        throw 'Release archive did not contain codex-usage-reserve.exe.'
    }
    $reportedVersion = (& $staged --version | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $reportedVersion -ne $version) {
        throw "Release binary version does not match tag $tag"
    }
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    $target = Join-Path $installDir 'codex-usage-reserve.exe'
    Move-Item -LiteralPath $staged -Destination $target -Force

    $enableArgs = @('enable')
    $codexHome = if ($env:CODEX_USAGE_RESERVE_CODEX_HOME) { $env:CODEX_USAGE_RESERVE_CODEX_HOME } else { $env:CODEX_HOME }
    if ($codexHome) {
        $enableArgs += @('--codex-home', $codexHome)
    }
    & $target @enableArgs
    if ($LASTEXITCODE -ne 0) {
        throw "Policy enable failed with exit code $LASTEXITCODE"
    }
    Write-Output "Installed and enabled: $target"
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (($userPath -split ';') -notcontains $installDir) {
        Write-Output "Add to your user PATH: $installDir"
        Write-Output ('$env:Path = "' + $installDir + ';' + '$env:Path"')
    }
} finally {
    Remove-Item -LiteralPath $temp -Recurse -Force -ErrorAction SilentlyContinue
}
