# build.ps1 - Build StreamForge deployment artifacts on Windows.
# Run from the project root:
#   powershell -NoProfile -ExecutionPolicy Bypass -File deploy\StreamForge\build.ps1

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = (Resolve-Path (Join-Path $ScriptDir '..\..')).Path

$JdkHome = 'C:\Users\admin\.jdks\ms-17.0.16'
$GoExe = 'C:\Users\admin\sdk\go1.25.6\bin\go.exe'

function Require-Command {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Name,

        [Parameter(Mandatory = $true)]
        [string]$Hint
    )

    if (-not (Get-Command -Name $Name -ErrorAction SilentlyContinue)) {
        throw "[error] Command not found: $Name. $Hint"
    }
}

function Require-File {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path,

        [Parameter(Mandatory = $true)]
        [string]$Description
    )

    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "[error] $Description not found: $Path"
    }
}

Write-Host "==> Project root: $ProjectRoot"
Write-Host "==> Deployment directory: $ScriptDir"

# ---------------- 0. Toolchain ----------------
Require-File -Path (Join-Path $JdkHome 'bin\java.exe') -Description 'JDK 17 java.exe'
Require-File -Path $GoExe -Description 'Go 1.25.6 executable'
Require-Command -Name 'npm.cmd' -Hint 'Install Node.js 22 or 24 and add it to PATH.'

$env:JAVA_HOME = $JdkHome
$env:PATH = "$JdkHome\bin;$env:PATH"

Write-Host "==> JDK: $JdkHome"
Write-Host "==> Go: $GoExe"

# Use same-origin reverse proxies by default. A literal '/' is used instead of an
# empty value because empty environment variables may be omitted from Vite's
# import.meta.env, which would activate the localhost fallback in frontend code.
# Existing non-empty caller values are preserved for custom deployments.
if ([string]::IsNullOrWhiteSpace($env:VITE_USER_API_BASE_URL)) {
    $env:VITE_USER_API_BASE_URL = '/'
}
if ([string]::IsNullOrWhiteSpace($env:VITE_MEDIA_API_BASE_URL)) {
    $env:VITE_MEDIA_API_BASE_URL = '/'
}

Write-Host "==> Frontend API base URLs: user=$env:VITE_USER_API_BASE_URL media=$env:VITE_MEDIA_API_BASE_URL"

# ---------------- 1. user-service ----------------
Write-Host ''
Write-Host '==> [1/3] Building user-service (Spring Boot)'

$userServiceDir = Join-Path $ProjectRoot 'user-service'
$userServiceJar = Join-Path $userServiceDir 'target\user-service-0.0.1-SNAPSHOT.jar'
$userServiceDest = Join-Path $ScriptDir 'user-service\user-service.jar'

Push-Location $userServiceDir
try {
    & .\mvnw.cmd -B -DskipTests package
    if ($LASTEXITCODE -ne 0) {
        throw "Maven build failed with exit code $LASTEXITCODE."
    }

    Require-File -Path $userServiceJar -Description 'user-service build artifact'
    Copy-Item -LiteralPath $userServiceJar -Destination $userServiceDest -Force
    Write-Host "==> Copied user-service artifact to: $userServiceDest"
}
finally {
    Pop-Location
}

# ---------------- 2. media-service ----------------
Write-Host ''
Write-Host '==> [2/3] Building media-service (Go, linux/amd64)'

$mediaServiceDir = Join-Path $ProjectRoot 'media-service'
$mediaServiceDest = Join-Path $ScriptDir 'media-service\media-service'
$previousCgoEnabled = $env:CGO_ENABLED
$previousGoos = $env:GOOS
$previousGoarch = $env:GOARCH

Push-Location $mediaServiceDir
try {
    $env:CGO_ENABLED = '0'
    $env:GOOS = 'linux'
    $env:GOARCH = 'amd64'

    & $GoExe build -trimpath -ldflags='-s -w' -o $mediaServiceDest .\cmd\media-service
    if ($LASTEXITCODE -ne 0) {
        throw "Go build failed with exit code $LASTEXITCODE."
    }

    Require-File -Path $mediaServiceDest -Description 'media-service build artifact'
    Write-Host "==> Generated media-service artifact: $mediaServiceDest"
}
finally {
    if ($null -eq $previousCgoEnabled) {
        Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
    }
    else {
        $env:CGO_ENABLED = $previousCgoEnabled
    }

    if ($null -eq $previousGoos) {
        Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    }
    else {
        $env:GOOS = $previousGoos
    }

    if ($null -eq $previousGoarch) {
        Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    }
    else {
        $env:GOARCH = $previousGoarch
    }

    Pop-Location
}

# ---------------- 3. frontend ----------------
Write-Host ''
Write-Host '==> [3/3] Building frontend (Vite)'

$frontendDir = Join-Path $ProjectRoot 'frontend'
$frontendDist = Join-Path $frontendDir 'dist'
$frontendIndex = Join-Path $frontendDist 'index.html'
$frontendDest = Join-Path $ScriptDir 'frontend\dist'

Push-Location $frontendDir
try {
    if (-not (Test-Path -LiteralPath 'node_modules' -PathType Container)) {
        & npm.cmd ci
        if ($LASTEXITCODE -ne 0) {
            throw "npm ci failed with exit code $LASTEXITCODE."
        }
    }

    & npm.cmd run build
    if ($LASTEXITCODE -ne 0) {
        throw "Frontend build failed with exit code $LASTEXITCODE."
    }

    if (-not (Test-Path -LiteralPath $frontendDist -PathType Container)) {
        throw "[error] Frontend build output not found: $frontendDist"
    }
    Require-File -Path $frontendIndex -Description 'frontend index.html'

    $frontendScripts = Get-ChildItem -LiteralPath $frontendDist -Recurse -File -Filter '*.js'
    if ($env:VITE_USER_API_BASE_URL -eq '/' -and
        ($frontendScripts | Select-String -SimpleMatch 'http://localhost:8081' -Quiet)) {
        throw '[error] Frontend artifact still contains http://localhost:8081.'
    }
    if ($env:VITE_MEDIA_API_BASE_URL -eq '/' -and
        ($frontendScripts | Select-String -SimpleMatch 'http://localhost:8080' -Quiet)) {
        throw '[error] Frontend artifact still contains http://localhost:8080.'
    }

    if (Test-Path -LiteralPath $frontendDest) {
        Remove-Item -LiteralPath $frontendDest -Recurse -Force
    }
    Copy-Item -LiteralPath $frontendDist -Destination $frontendDest -Recurse -Force
    Write-Host "==> Copied frontend artifact to: $frontendDest"
}
finally {
    Pop-Location
}

Write-Host ''
Write-Host '======================================================'
Write-Host 'Build completed. Next steps:'
Write-Host '  1) tar -czf StreamForge.tgz -C deploy StreamForge'
Write-Host '  2) Upload and extract the archive, then enter StreamForge'
Write-Host '  3) cp .env.example .env and edit .env'
Write-Host '  4) bash deploy.sh'
Write-Host '======================================================'
