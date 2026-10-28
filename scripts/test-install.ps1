$ErrorActionPreference = 'Stop'

function Assert-That {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) {
        throw "Assertion failed: $Message"
    }
}

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$installer = Join-Path $repoRoot 'install.ps1'
$temp = Join-Path ([IO.Path]::GetTempPath()) ("codex-usage-reserve-installer-test-" + [guid]::NewGuid())
$buildDir = Join-Path $temp 'build'
$fixtureDir = Join-Path $temp 'fixtures'
$installDir = Join-Path $temp 'bin'
$codexHome = Join-Path $temp 'codex-home'
$environmentNames = @(
    'CODEX_USAGE_RESERVE_VERSION',
    'CODEX_USAGE_RESERVE_RELEASE_BASE_URL',
    'CODEX_USAGE_RESERVE_INSTALL_DIR',
    'CODEX_USAGE_RESERVE_CODEX_HOME'
)
$oldEnvironment = @{}
foreach ($name in $environmentNames) {
    $oldEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}
$mockCallLog = [pscustomobject]@{
    LatestRequestCount = 0
    DownloadUrls = [System.Collections.Generic.List[string]]::new()
}
$originalRestFunction = Get-Item Function:\Invoke-RestMethod -ErrorAction SilentlyContinue
$originalWebFunction = Get-Item Function:\Invoke-WebRequest -ErrorAction SilentlyContinue
$restMock = {
    param([string]$Uri)
    $mockCallLog.LatestRequestCount++
    if ($Uri -ne 'https://api.github.com/repos/taejunoh/codex-usage-reserve/releases/latest') {
        throw "Unexpected REST request: $Uri"
    }
    [pscustomobject]@{ tag_name = 'v0.1.0' }
}.GetNewClosure()
Set-Item Function:\Invoke-RestMethod -Value $restMock

$webMock = {
    param([string]$Uri, [string]$OutFile)
    $mockCallLog.DownloadUrls.Add($Uri)
    $name = [IO.Path]::GetFileName(([uri]$Uri).AbsolutePath)
    $source = Join-Path $fixtureDir $name
    if (-not (Test-Path -LiteralPath $source -PathType Leaf)) {
        throw "No fixture for download: $Uri"
    }
    Copy-Item -LiteralPath $source -Destination $OutFile -Force
}.GetNewClosure()
Set-Item Function:\Invoke-WebRequest -Value $webMock

try {
    New-Item -ItemType Directory -Path $buildDir, $fixtureDir -Force | Out-Null
    Push-Location $repoRoot
    try {
        $binary = Join-Path $buildDir 'codex-usage-reserve.exe'
        & go build -trimpath -ldflags '-s -w -X main.version=0.1.0' -o $binary ./cmd/codex-usage-reserve
        if ($LASTEXITCODE -ne 0) {
            throw "Go build failed with exit code $LASTEXITCODE"
        }
    } finally {
        Pop-Location
    }

    $arch = if ($env:PROCESSOR_ARCHITECTURE -match 'ARM64') { 'arm64' } else { 'amd64' }
    $asset = "codex-usage-reserve_0.1.0_windows_$($arch).tar.gz"
    $archivePath = Join-Path $fixtureDir $asset
    & tar.exe -czf $archivePath -C $buildDir 'codex-usage-reserve.exe'
    if ($LASTEXITCODE -ne 0) {
        throw "Could not create the Windows release fixture (exit code $LASTEXITCODE)"
    }
    $digest = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    $checksumPath = Join-Path $fixtureDir 'checksums.txt'
    Set-Content -LiteralPath $checksumPath -Value "$digest  $asset" -Encoding ascii

    $env:CODEX_USAGE_RESERVE_RELEASE_BASE_URL = 'https://fixture.invalid/releases/download'
    $env:CODEX_USAGE_RESERVE_INSTALL_DIR = $installDir
    $env:CODEX_USAGE_RESERVE_CODEX_HOME = $codexHome
    Remove-Item Env:CODEX_USAGE_RESERVE_VERSION -ErrorAction SilentlyContinue

    # Latest lookup happens once; both fixture downloads use its exact tag.
    & $installer
    Assert-That ($mockCallLog.LatestRequestCount -eq 1) 'latest release tag should be resolved once'
    Assert-That ($mockCallLog.DownloadUrls.Count -eq 2) 'expected archive and checksum downloads'
    foreach ($url in $mockCallLog.DownloadUrls) {
        Assert-That ($url.StartsWith('https://fixture.invalid/releases/download/v0.1.0/')) "download was not pinned to v0.1.0: $url"
    }
    $installed = Join-Path $installDir 'codex-usage-reserve.exe'
    Assert-That (Test-Path -LiteralPath $installed -PathType Leaf) 'installer did not install the executable'
    $reportedVersion = (& $installed --version | Out-String).Trim()
    Assert-That ($LASTEXITCODE -eq 0 -and $reportedVersion -eq '0.1.0') 'installed executable reported the wrong version'
    $instructions = Join-Path $codexHome 'AGENTS.md'
    Assert-That (Test-Path -LiteralPath $instructions -PathType Leaf) 'installer did not enable the policy'
    Assert-That ((Get-Content -Raw -LiteralPath $instructions).Contains('codex-usage-reserve:managed')) 'enabled policy marker was not written'
    $status = (& $installed status --codex-home $codexHome | Out-String)
    Assert-That ($status.Contains('10%')) 'new install should use the default 10% threshold'

    # An explicit version without v is accepted and repeat enable preserves a custom threshold.
    & $installed set 15 --codex-home $codexHome
    $mockCallLog.DownloadUrls.Clear()
    $env:CODEX_USAGE_RESERVE_VERSION = '0.1.0'
    & $installer
    Assert-That ($mockCallLog.LatestRequestCount -eq 1) 'explicit version should skip the latest lookup'
    Assert-That ($mockCallLog.DownloadUrls.Count -eq 2) 'explicit version should download exactly two assets'
    foreach ($url in $mockCallLog.DownloadUrls) {
        Assert-That ($url.StartsWith('https://fixture.invalid/releases/download/v0.1.0/')) "explicit version URL was not normalized and pinned: $url"
    }
    $status = (& $installed status --codex-home $codexHome | Out-String)
    Assert-That ($status.Contains('15%')) 'repeat installer enable changed the custom threshold'

    # A bad checksum must preserve both the installed executable and instructions.
    $installedHash = (Get-FileHash -LiteralPath $installed -Algorithm SHA256).Hash
    $instructionsHash = (Get-FileHash -LiteralPath $instructions -Algorithm SHA256).Hash
    Set-Content -LiteralPath $checksumPath -Value ("{0}  {1}" -f ('0' * 64), $asset) -Encoding ascii
    $checksumRejected = $false
    try {
        & $installer
    } catch {
        $checksumRejected = $true
    }
    Assert-That $checksumRejected 'bad checksum was not rejected'
    Assert-That ((Get-FileHash -LiteralPath $installed -Algorithm SHA256).Hash -eq $installedHash) 'bad checksum overwrote the installed executable'
    Assert-That ((Get-FileHash -LiteralPath $instructions -Algorithm SHA256).Hash -eq $instructionsHash) 'bad checksum mutated Codex instructions'
    Write-Output 'Windows installer integration checks passed'
} finally {
    if ($null -eq $originalRestFunction) {
        Remove-Item Function:\Invoke-RestMethod -ErrorAction SilentlyContinue
    } else {
        Set-Item Function:\Invoke-RestMethod -Value $originalRestFunction.ScriptBlock
    }
    if ($null -eq $originalWebFunction) {
        Remove-Item Function:\Invoke-WebRequest -ErrorAction SilentlyContinue
    } else {
        Set-Item Function:\Invoke-WebRequest -Value $originalWebFunction.ScriptBlock
    }
    foreach ($name in $environmentNames) {
        if ($null -eq $oldEnvironment[$name]) {
            Remove-Item "Env:$name" -ErrorAction SilentlyContinue
        } else {
            Set-Item "Env:$name" -Value $oldEnvironment[$name]
        }
    }
    Remove-Item -LiteralPath $temp -Recurse -Force -ErrorAction SilentlyContinue
}
