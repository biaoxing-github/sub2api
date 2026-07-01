<#
.SYNOPSIS
  sub2api 本地蓝绿发布收敛脚本。

.DESCRIPTION
  默认只输出发布计划，不构建、不部署、不切流。
  只有显式传入 -Execute 才会构建 committed HEAD 并部署到 idle 颜色。
  只有同时传入 -Execute -Cutover 才会在候选验证通过后切换代理 upstream。

.EXAMPLE
  .\deploy\release-bluegreen.ps1 -ImageVersion v0.1.140.1

.EXAMPLE
  .\deploy\release-bluegreen.ps1 -ImageVersion v0.1.140.1 -Execute -AllowDirty

.EXAMPLE
  .\deploy\release-bluegreen.ps1 -ImageVersion v0.1.140.1 -Execute -Cutover -AllowDirty
#>

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^v\d+\.\d+\.\d+\.\d+$')]
    [string]$ImageVersion,

    [switch]$Execute,

    [switch]$Cutover,

    [switch]$AllowDirty,

    [string]$DeployRoot = 'D:\sub2api-deploy',

    [string]$ImageRepository = 'sub2api',

    [int]$CandidateObserveSeconds = 75,

    [int]$PostCutoverObserveSeconds = 65
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Write-Step {
    param([string]$Message)
    Write-Host ''
    Write-Host "==> $Message"
}

function Format-CommandLine {
    param(
        [string]$File,
        [string[]]$Arguments
    )

    $formattedArgs = $Arguments | ForEach-Object {
        if ($_ -match '\s') {
            '"' + $_ + '"'
        } else {
            $_
        }
    }
    return ($File + ' ' + ($formattedArgs -join ' ')).Trim()
}

function Invoke-External {
    param(
        [string]$File,
        [string[]]$Arguments
    )

    $display = Format-CommandLine -File $File -Arguments $Arguments
    if (-not $Execute) {
        Write-Host "PLAN> $display"
        return
    }

    Write-Host "RUN> $display"
    & $File @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed with exit code $LASTEXITCODE`: $display"
    }
}

function Invoke-ExternalOutput {
    param(
        [string]$File,
        [string[]]$Arguments
    )

    $output = & $File @Arguments
    if ($LASTEXITCODE -ne 0) {
        $display = Format-CommandLine -File $File -Arguments $Arguments
        throw "Command failed with exit code $LASTEXITCODE`: $display"
    }
    return ($output | Out-String).Trim()
}

function Get-ActiveColor {
    param([string]$ActiveConfPath)

    if (-not (Test-Path -LiteralPath $ActiveConfPath)) {
        throw "Active upstream file not found: $ActiveConfPath"
    }

    $content = Get-Content -LiteralPath $ActiveConfPath -Raw
    $match = [regex]::Match($content, 'sub2api-(blue|green):8080')
    if (-not $match.Success) {
        throw "Cannot detect active color from $ActiveConfPath"
    }
    return $match.Groups[1].Value
}

function Get-OppositeColor {
    param([string]$Color)

    if ($Color -eq 'blue') {
        return 'green'
    }
    if ($Color -eq 'green') {
        return 'blue'
    }
    throw "Unsupported color: $Color"
}

function Get-DeployEnv {
    param([string]$EnvPath)

    $result = @{}
    if (-not (Test-Path -LiteralPath $EnvPath)) {
        throw "Deploy env file not found: $EnvPath"
    }

    foreach ($line in Get-Content -LiteralPath $EnvPath) {
        if ($line -match '^\s*#' -or $line -notmatch '=') {
            continue
        }
        $name, $value = $line -split '=', 2
        $result[$name.Trim()] = $value.Trim()
    }
    return $result
}

function Set-DeployEnvValue {
    param(
        [string]$EnvPath,
        [string]$Name,
        [string]$Value
    )

    if (-not $Execute) {
        Write-Host "PLAN> set $Name=$Value in $EnvPath"
        return
    }

    $lines = [System.Collections.Generic.List[string]]::new()
    if (Test-Path -LiteralPath $EnvPath) {
        foreach ($line in [System.IO.File]::ReadAllLines($EnvPath)) {
            [void]$lines.Add($line)
        }
    }

    $found = $false
    $pattern = '^\s*' + [regex]::Escape($Name) + '='
    for ($i = 0; $i -lt $lines.Count; $i++) {
        if ($lines[$i] -match $pattern) {
            $lines[$i] = "$Name=$Value"
            $found = $true
            break
        }
    }
    if (-not $found) {
        [void]$lines.Add("$Name=$Value")
    }

    $encoding = [System.Text.UTF8Encoding]::new($false)
    [System.IO.File]::WriteAllLines($EnvPath, $lines, $encoding)
}

function Get-ColorPort {
    param(
        [string]$Color,
        [hashtable]$Env
    )

    $key = "SUB2API_$($Color.ToUpperInvariant())_PORT"
    if ($Env.ContainsKey($key) -and $Env[$key]) {
        return [int]$Env[$key]
    }
    if ($Color -eq 'blue') {
        return 18083
    }
    return 18082
}

function Assert-GitBoundary {
    param([string]$RepoRoot)

    $commit = Invoke-ExternalOutput -File 'git' -Arguments @('-C', $RepoRoot, 'rev-parse', '--short=12', 'HEAD')
    $branch = Invoke-ExternalOutput -File 'git' -Arguments @('-C', $RepoRoot, 'branch', '--show-current')
    $dirty = Invoke-ExternalOutput -File 'git' -Arguments @('-C', $RepoRoot, 'status', '--porcelain')

    Write-Host "Branch: $branch"
    Write-Host "Committed HEAD: $commit"

    if ($dirty) {
        Write-Host "Dirty working tree:"
        Write-Host $dirty
        if ($Execute -and -not $AllowDirty) {
            throw "Working tree is dirty. Commit/stash first, or rerun with -AllowDirty when dirty files are known unrelated. Build still uses committed HEAD."
        }
    }

    return $commit
}

function Assert-ImageTagAvailable {
    param([string]$ImageTag)

    if (-not $Execute) {
        Write-Host "PLAN> verify image tag does not already exist: $ImageTag"
        return
    }

    & docker image inspect $ImageTag *> $null
    if ($LASTEXITCODE -eq 0) {
        throw "Immutable image tag already exists locally and will not be overwritten: $ImageTag"
    }
    $global:LASTEXITCODE = 0
}

function Invoke-ImageBuild {
    param(
        [string]$RepoRoot,
        [string]$ImageTag,
        [string]$MajorVersion,
        [string]$ImageVersion,
        [string]$Commit
    )

    $display = "git -C `"$RepoRoot`" archive --format=tar HEAD | docker build --pull=false -t $ImageTag --label org.opencontainers.image.version=$MajorVersion --label org.opencontainers.image.revision=$Commit --build-arg VERSION=$MajorVersion --build-arg IMAGE_VERSION=$ImageVersion --build-arg COMMIT=$Commit -"
    if (-not $Execute) {
        Write-Host "PLAN> $display"
        return
    }

    Write-Host "RUN> $display"
    git -C $RepoRoot archive --format=tar HEAD | docker build --pull=false `
        -t $ImageTag `
        --label "org.opencontainers.image.version=$MajorVersion" `
        --label "org.opencontainers.image.revision=$Commit" `
        --build-arg "VERSION=$MajorVersion" `
        --build-arg "IMAGE_VERSION=$ImageVersion" `
        --build-arg "COMMIT=$Commit" `
        -
    if ($LASTEXITCODE -ne 0) {
        throw "Docker build failed for $ImageTag"
    }
}

function Invoke-HttpStatus {
    param(
        [string]$Method,
        [string]$Url,
        [string]$Body
    )

    $params = @{
        Uri             = $Url
        Method          = $Method
        TimeoutSec      = 10
        UseBasicParsing = $true
    }
    if ($PSBoundParameters.ContainsKey('Body')) {
        $params['Body'] = $Body
        $params['ContentType'] = 'application/json'
    }

    try {
        $response = Invoke-WebRequest @params
        return [int]$response.StatusCode
    } catch {
        if ($_.Exception.Response) {
            return [int]$_.Exception.Response.StatusCode
        }
        throw
    }
}

function Assert-HttpStatus {
    param(
        [string]$Method,
        [string]$Url,
        [int]$Expected,
        [string]$Body
    )

    if (-not $Execute) {
        Write-Host "PLAN> expect HTTP $Expected from $Method $Url"
        return
    }

    if ($PSBoundParameters.ContainsKey('Body')) {
        $status = Invoke-HttpStatus -Method $Method -Url $Url -Body $Body
    } else {
        $status = Invoke-HttpStatus -Method $Method -Url $Url
    }
    if ($status -ne $Expected) {
        throw "Unexpected HTTP status for $Method $Url`: expected $Expected, got $status"
    }
    Write-Host "PASS> $Method $Url -> $status"
}

function Wait-Health {
    param(
        [string]$BaseUrl,
        [int]$TimeoutSeconds
    )

    if (-not $Execute) {
        Write-Host "PLAN> wait until $BaseUrl/health returns 200 within ${TimeoutSeconds}s"
        return
    }

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        try {
            $status = Invoke-HttpStatus -Method 'GET' -Url "$BaseUrl/health"
            if ($status -eq 200) {
                Write-Host "PASS> $BaseUrl/health -> 200"
                return
            }
        } catch {
            Start-Sleep -Seconds 2
        }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)

    throw "Health check timed out: $BaseUrl/health"
}

function Assert-StaticAsset {
    param([string]$BaseUrl)

    if (-not $Execute) {
        Write-Host "PLAN> fetch $BaseUrl/ and verify one /assets/*.js or /assets/*.css resource"
        return
    }

    $home = Invoke-WebRequest -Uri "$BaseUrl/" -Method GET -UseBasicParsing -TimeoutSec 10
    if ([int]$home.StatusCode -ne 200) {
        throw "Home page did not return 200 from $BaseUrl/"
    }

    $match = [regex]::Match($home.Content, "/assets/[^`"'\s>]+\.(?:js|css)")
    if (-not $match.Success) {
        throw "No static asset path found in $BaseUrl/"
    }

    Assert-HttpStatus -Method 'GET' -Url ($BaseUrl + $match.Value) -Expected 200
}

function Get-ContainerSummary {
    param([string]$Container)

    return Invoke-ExternalOutput -File 'docker' -Arguments @(
        'inspect',
        $Container,
        '--format',
        'Image={{.Config.Image}} ImageID={{.Image}} Health={{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}} RestartCount={{.RestartCount}} Status={{.State.Status}}'
    )
}

function Assert-ContainerHealthy {
    param(
        [string]$Container,
        [string]$ExpectedImage
    )

    if (-not $Execute) {
        Write-Host "PLAN> inspect $Container and require image=$ExpectedImage, Health=healthy, Status=running"
        return
    }

    $summary = Get-ContainerSummary -Container $Container
    Write-Host "INSPECT> $summary"
    if ($summary -notmatch [regex]::Escape("Image=$ExpectedImage")) {
        throw "$Container is not running expected image $ExpectedImage"
    }
    if ($summary -notmatch 'Health=healthy') {
        throw "$Container is not healthy"
    }
    if ($summary -notmatch 'Status=running') {
        throw "$Container is not running"
    }
}

function Assert-CleanLogs {
    param(
        [string]$Container,
        [int]$SinceSeconds
    )

    $pattern = '(?i)panic|fatal|migration.*fail|checksum|pq:|bind:|address already in use|listen tcp|rebuild failed'
    if (-not $Execute) {
        Write-Host "PLAN> scan docker logs --since ${SinceSeconds}s $Container for release-critical patterns"
        return
    }

    $logLines = docker logs --since "${SinceSeconds}s" $Container 2>&1
    $matches = $logLines | Select-String -Pattern $pattern
    if ($matches) {
        $sample = ($matches | Select-Object -First 8 | ForEach-Object { $_.Line }) -join [Environment]::NewLine
        throw "Critical log pattern detected in $Container logs:$([Environment]::NewLine)$sample"
    }
    Write-Host "PASS> $Container logs clean for last ${SinceSeconds}s"
}

function Invoke-Smoke {
    param(
        [string]$BaseUrl
    )

    Wait-Health -BaseUrl $BaseUrl -TimeoutSeconds 120
    Assert-StaticAsset -BaseUrl $BaseUrl
    Assert-HttpStatus -Method 'GET' -Url "$BaseUrl/api/v1/admin/users" -Expected 401
    Assert-HttpStatus -Method 'POST' -Url "$BaseUrl/responses" -Expected 401 -Body '{"model":"gpt-5.5","input":"ping"}'
    Assert-HttpStatus -Method 'POST' -Url "$BaseUrl/v1/responses" -Expected 401 -Body '{"model":"gpt-5.5","input":"ping"}'
}

$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$deployRootResolved = (Resolve-Path -LiteralPath $DeployRoot).Path
$envPath = Join-Path $deployRootResolved '.env'
$activeConfPath = Join-Path $deployRootResolved 'proxy\upstreams\active.conf'
$majorVersion = (Get-Content -LiteralPath (Join-Path $repoRoot 'backend\cmd\server\VERSION') -Raw).Trim()
$imageTag = "$ImageRepository`:$ImageVersion"

Write-Step 'Read release boundary'
$commit = Assert-GitBoundary -RepoRoot $repoRoot
$activeColor = Get-ActiveColor -ActiveConfPath $activeConfPath
$idleColor = Get-OppositeColor -Color $activeColor
$env = Get-DeployEnv -EnvPath $envPath
$candidatePort = Get-ColorPort -Color $idleColor -Env $env
$candidateBaseUrl = "http://127.0.0.1:$candidatePort"
$activeImage = Get-ContainerSummary -Container "sub2api-$activeColor"

Write-Host "Major version: $majorVersion"
Write-Host "Image version: $ImageVersion"
Write-Host "Image tag: $imageTag"
Write-Host "Active color: $activeColor"
Write-Host "Idle color: $idleColor"
Write-Host "Candidate URL: $candidateBaseUrl"
Write-Host "Active container: $activeImage"

Write-Step 'Build committed HEAD image'
Assert-ImageTagAvailable -ImageTag $imageTag
Invoke-ImageBuild -RepoRoot $repoRoot -ImageTag $imageTag -MajorVersion $majorVersion -ImageVersion $ImageVersion -Commit $commit

Write-Step 'Deploy idle candidate container'
$idleEnvName = "SUB2API_$($idleColor.ToUpperInvariant())_IMAGE"
Set-DeployEnvValue -EnvPath $envPath -Name $idleEnvName -Value $imageTag
Push-Location -LiteralPath $deployRootResolved
try {
    Invoke-External -File 'docker' -Arguments @(
        'compose',
        '--env-file',
        '.env',
        '-f',
        "docker-compose.$idleColor.yml",
        'up',
        '-d',
        '--no-deps',
        '--force-recreate',
        "sub2api-$idleColor"
    )
} finally {
    Pop-Location
}

Write-Step 'Candidate smoke and log checks'
Invoke-Smoke -BaseUrl $candidateBaseUrl
if ($Execute) {
    Start-Sleep -Seconds $CandidateObserveSeconds
}
Assert-ContainerHealthy -Container "sub2api-$idleColor" -ExpectedImage $imageTag
Assert-CleanLogs -Container "sub2api-$idleColor" -SinceSeconds ([Math]::Max($CandidateObserveSeconds + 30, 120))

if (-not $Cutover) {
    Write-Step 'Stop at candidate boundary'
    if ($Execute) {
        Write-Host "Candidate is ready on $candidateBaseUrl. Rerun with -Execute -Cutover to switch active traffic."
    } else {
        Write-Host "Dry-run only. Add -Execute to build/deploy candidate; add -Cutover as well to switch traffic after validation."
    }
    exit 0
}

Write-Step 'Switch proxy upstream'
$newActiveConf = @"
upstream sub2api_active {
  server sub2api-$($idleColor):8080;
  keepalive 32;
}
"@

if (-not $Execute) {
    Write-Host "PLAN> write $activeConfPath to point to sub2api-$($idleColor):8080"
} else {
    $encoding = [System.Text.UTF8Encoding]::new($false)
    [System.IO.File]::WriteAllText($activeConfPath, $newActiveConf, $encoding)
}
Invoke-External -File 'docker' -Arguments @('exec', 'sub2api-proxy', 'nginx', '-t')
Invoke-External -File 'docker' -Arguments @('exec', 'sub2api-proxy', 'nginx', '-s', 'reload')

Write-Step 'Post-cutover public and local proxy smoke'
Invoke-Smoke -BaseUrl 'http://127.0.0.1:8080'
Invoke-Smoke -BaseUrl 'http://127.0.0.1:18081'
if ($Execute) {
    Start-Sleep -Seconds $PostCutoverObserveSeconds
}
Wait-Health -BaseUrl 'http://127.0.0.1:8080' -TimeoutSeconds 30
Wait-Health -BaseUrl 'http://127.0.0.1:18081' -TimeoutSeconds 30
Wait-Health -BaseUrl $candidateBaseUrl -TimeoutSeconds 30
Assert-ContainerHealthy -Container "sub2api-$idleColor" -ExpectedImage $imageTag
Assert-CleanLogs -Container "sub2api-$idleColor" -SinceSeconds ([Math]::Max($PostCutoverObserveSeconds + 30, 120))

Write-Step 'Done'
Write-Host "Active color is now $idleColor with $imageTag."
Write-Host "Rollback target remains sub2api-$activeColor."
