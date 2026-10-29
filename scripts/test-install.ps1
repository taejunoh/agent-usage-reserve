$ErrorActionPreference = 'Stop'

function Assert-That {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) {
        throw "Assertion failed: $Message"
    }
}

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$installer = Join-Path $repoRoot 'install.ps1'
$version = '0.2.0'
$temp = Join-Path ([IO.Path]::GetTempPath()) ("agent-usage-reserve-installer-test-" + [guid]::NewGuid())
$buildDir = Join-Path $temp 'build'
$fixtureDir = Join-Path $temp 'fixtures'
$environmentNames = @(
    'AGENT_USAGE_RESERVE_VERSION',
    'AGENT_USAGE_RESERVE_RELEASE_BASE_URL',
    'AGENT_USAGE_RESERVE_INSTALL_DIR',
    'AGENT_USAGE_RESERVE_CODEX_HOME',
    'AGENT_USAGE_RESERVE_CLAUDE_HOME',
    'CODEX_HOME',
    'CLAUDE_CONFIG_DIR',
    'HOME',
    'USERPROFILE'
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
    if ($Uri -ne 'https://api.github.com/repos/taejunoh/agent-usage-reserve/releases/latest') {
        throw "Unexpected REST request: $Uri"
    }
    [pscustomobject]@{ tag_name = 'v0.2.0' }
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

# Fresh sandbox for one case: fake HOME, install dir and agent homes. Real homes are never touched.
function New-Case {
    param([string]$Name)
    $case = [pscustomobject]@{
        Root = Join-Path $temp $Name
        Home = Join-Path $temp "$Name/home"
        InstallDir = Join-Path $temp "$Name/bin"
        CodexHome = Join-Path $temp "$Name/codex-home"
        ClaudeHome = Join-Path $temp "$Name/claude-home"
    }
    New-Item -ItemType Directory -Path $case.Home, $case.InstallDir, $case.CodexHome, $case.ClaudeHome -Force | Out-Null
    $env:HOME = $case.Home
    $env:USERPROFILE = $case.Home
    $env:AGENT_USAGE_RESERVE_INSTALL_DIR = $case.InstallDir
    Remove-Item Env:AGENT_USAGE_RESERVE_VERSION, Env:AGENT_USAGE_RESERVE_CODEX_HOME, Env:AGENT_USAGE_RESERVE_CLAUDE_HOME, Env:CODEX_HOME, Env:CLAUDE_CONFIG_DIR -ErrorAction SilentlyContinue
    $mockCallLog.LatestRequestCount = 0
    $mockCallLog.DownloadUrls.Clear()
    return $case
}

try {
    New-Item -ItemType Directory -Path $buildDir, $fixtureDir -Force | Out-Null
    Push-Location $repoRoot
    try {
        $binary = Join-Path $buildDir 'agent-usage-reserve.exe'
        & go build -trimpath -ldflags "-s -w -X main.version=$version" -o $binary ./cmd/agent-usage-reserve
        if ($LASTEXITCODE -ne 0) {
            throw "Go build failed with exit code $LASTEXITCODE"
        }
    } finally {
        Pop-Location
    }

    $arch = if ($env:PROCESSOR_ARCHITECTURE -match 'ARM64') { 'arm64' } else { 'amd64' }
    $asset = "agent-usage-reserve_$($version)_windows_$($arch).tar.gz"
    $archivePath = Join-Path $fixtureDir $asset
    & tar.exe -czf $archivePath -C $buildDir 'agent-usage-reserve.exe'
    if ($LASTEXITCODE -ne 0) {
        throw "Could not create the Windows release fixture (exit code $LASTEXITCODE)"
    }
    $digest = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    $checksumPath = Join-Path $fixtureDir 'checksums.txt'
    $goodChecksum = "$digest  $asset"
    Set-Content -LiteralPath $checksumPath -Value $goodChecksum -Encoding ascii
    $env:AGENT_USAGE_RESERVE_RELEASE_BASE_URL = 'https://fixture.invalid/releases/download'

    # Latest lookup happens once; both agents are enabled through explicit homes (no prompt),
    # and the old executable is removed after the new one is installed.
    $case = New-Case 'latest'
    $oldExe = Join-Path $case.InstallDir 'codex-usage-reserve.exe'
    Set-Content -LiteralPath $oldExe -Value 'old executable' -Encoding ascii
    $env:AGENT_USAGE_RESERVE_CODEX_HOME = $case.CodexHome
    $env:AGENT_USAGE_RESERVE_CLAUDE_HOME = $case.ClaudeHome
    $output = (& $installer 2>&1 | Out-String)
    Assert-That ($mockCallLog.LatestRequestCount -eq 1) 'latest release tag should be resolved once'
    Assert-That ($mockCallLog.DownloadUrls.Count -eq 2) 'expected archive and checksum downloads'
    foreach ($url in $mockCallLog.DownloadUrls) {
        Assert-That ($url.StartsWith("https://fixture.invalid/releases/download/v$version/")) "download was not pinned to v${version}: $url"
    }
    $installed = Join-Path $case.InstallDir 'agent-usage-reserve.exe'
    Assert-That (Test-Path -LiteralPath $installed -PathType Leaf) 'installer did not install the executable'
    $reportedVersion = (& $installed --version | Out-String).Trim()
    Assert-That ($LASTEXITCODE -eq 0 -and $reportedVersion -eq $version) 'installed executable reported the wrong version'
    Assert-That ($output.Contains('Installed and enabled')) 'installer did not report enabling'
    $codexFile = Join-Path $case.CodexHome 'AGENTS.md'
    $claudeFile = Join-Path $case.ClaudeHome 'CLAUDE.md'
    Assert-That ((Get-Content -Raw -LiteralPath $codexFile).Contains('agent-usage-reserve:managed:v1:begin agent=codex')) 'Codex marker was not written'
    Assert-That ((Get-Content -Raw -LiteralPath $claudeFile).Contains('agent-usage-reserve:managed:v1:begin agent=claude')) 'Claude marker was not written'
    Assert-That (-not (Test-Path -LiteralPath $oldExe)) 'old executable was not removed'
    Assert-That ($output.Contains("Removed old codex-usage-reserve: $oldExe")) 'removal of the old executable was not reported'
    $status = (& $installed status --codex-home $case.CodexHome --claude-home $case.ClaudeHome | Out-String)
    Assert-That ($status.Contains('10%')) 'new install should use the default 10% threshold'

    # An explicit version without v is accepted and repeat enable preserves a custom threshold.
    & $installed set 15 --codex-home $case.CodexHome --claude-home $case.ClaudeHome
    $mockCallLog.DownloadUrls.Clear()
    $env:AGENT_USAGE_RESERVE_VERSION = $version
    & $installer
    Assert-That ($mockCallLog.LatestRequestCount -eq 1) 'explicit version should skip the latest lookup'
    Assert-That ($mockCallLog.DownloadUrls.Count -eq 2) 'explicit version should download exactly two assets'
    foreach ($url in $mockCallLog.DownloadUrls) {
        Assert-That ($url.StartsWith("https://fixture.invalid/releases/download/v$version/")) "explicit version URL was not normalized and pinned: $url"
    }
    $status = (& $installed status --codex-home $case.CodexHome --claude-home $case.ClaudeHome | Out-String)
    Assert-That ($status.Contains('15%')) 'repeat installer enable changed the custom threshold'

    # No detectable agent: installed, installer succeeds, policy not enabled, guidance repeated.
    $case = New-Case 'noagent'
    $oldExe = Join-Path $case.InstallDir 'codex-usage-reserve.exe'
    Set-Content -LiteralPath $oldExe -Value 'old executable' -Encoding ascii
    $output = (& $installer 2>&1 | Out-String)
    Assert-That (Test-Path -LiteralPath (Join-Path $case.InstallDir 'agent-usage-reserve.exe') -PathType Leaf) 'no-agent run did not install the executable'
    Assert-That ($output.Contains('Installed (policy not enabled yet)')) 'no-agent run did not report the policy as not enabled'
    Assert-That ($output.Contains('Codex or Claude was not found')) 'no-agent run did not repeat the guidance'
    Assert-That (-not (Test-Path -LiteralPath $oldExe)) 'old executable was not removed in the no-agent run'

    # Any other enable failure (a home that is not a directory) is propagated.
    $case = New-Case 'refused'
    $notADirectory = Join-Path $case.Root 'not-a-directory'
    Set-Content -LiteralPath $notADirectory -Value 'file' -Encoding ascii
    $env:AGENT_USAGE_RESERVE_CODEX_HOME = $notADirectory
    $enableRejected = $false
    try {
        & $installer 2>&1 | Out-Null
    } catch {
        $enableRejected = $true
    }
    Assert-That $enableRejected 'installer did not fail when enable was refused'
    Assert-That (Test-Path -LiteralPath (Join-Path $case.InstallDir 'agent-usage-reserve.exe') -PathType Leaf) 'binary should stay installed after a refused enable'

    # A bad checksum must preserve the installed and old executables and every instruction file.
    $case = New-Case 'badsum'
    $installed = Join-Path $case.InstallDir 'agent-usage-reserve.exe'
    $oldExe = Join-Path $case.InstallDir 'codex-usage-reserve.exe'
    Set-Content -LiteralPath $installed -Value 'previous install' -Encoding ascii
    Set-Content -LiteralPath $oldExe -Value 'old executable' -Encoding ascii
    $installedHash = (Get-FileHash -LiteralPath $installed -Algorithm SHA256).Hash
    $oldHash = (Get-FileHash -LiteralPath $oldExe -Algorithm SHA256).Hash
    $env:AGENT_USAGE_RESERVE_CODEX_HOME = $case.CodexHome
    $env:AGENT_USAGE_RESERVE_CLAUDE_HOME = $case.ClaudeHome
    Set-Content -LiteralPath $checksumPath -Value ("{0}  {1}" -f ('0' * 64), $asset) -Encoding ascii
    $checksumRejected = $false
    try {
        & $installer 2>&1 | Out-Null
    } catch {
        $checksumRejected = $true
    }
    Assert-That $checksumRejected 'bad checksum was not rejected'
    Assert-That ((Get-FileHash -LiteralPath $installed -Algorithm SHA256).Hash -eq $installedHash) 'bad checksum overwrote the installed executable'
    Assert-That (Test-Path -LiteralPath $oldExe -PathType Leaf) 'bad checksum removed the old executable'
    Assert-That ((Get-FileHash -LiteralPath $oldExe -Algorithm SHA256).Hash -eq $oldHash) 'bad checksum modified the old executable'
    Assert-That (-not (Test-Path -LiteralPath (Join-Path $case.CodexHome 'AGENTS.md'))) 'bad checksum wrote Codex instructions'
    Assert-That (-not (Test-Path -LiteralPath (Join-Path $case.ClaudeHome 'CLAUDE.md'))) 'bad checksum wrote Claude instructions'
    Set-Content -LiteralPath $checksumPath -Value $goodChecksum -Encoding ascii
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
