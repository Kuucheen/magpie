$ErrorActionPreference = "Stop"

$repoOwner = if ($env:MAGPIE_REPO_OWNER) { $env:MAGPIE_REPO_OWNER } else { "Kuucheen" }
$repoName  = if ($env:MAGPIE_REPO_NAME)  { $env:MAGPIE_REPO_NAME }  else { "magpie" }
$repoRef   = if ($env:MAGPIE_REPO_REF)   { $env:MAGPIE_REPO_REF }   else { "main" }

$installDir = if ($env:MAGPIE_INSTALL_DIR) { $env:MAGPIE_INSTALL_DIR } else { "magpie" }

if ([string]::IsNullOrWhiteSpace($installDir) -or $installDir -eq "\") {
  throw "MAGPIE_INSTALL_DIR must not be empty or '\'."
}

$composeUrl = if ($env:MAGPIE_COMPOSE_URL) {
  $env:MAGPIE_COMPOSE_URL
} else {
  "https://raw.githubusercontent.com/$repoOwner/$repoName/$repoRef/docker-compose.yml"
}

$envExampleUrl = if ($env:MAGPIE_ENV_EXAMPLE_URL) {
  $env:MAGPIE_ENV_EXAMPLE_URL
} else {
  "https://raw.githubusercontent.com/$repoOwner/$repoName/$repoRef/.env.example"
}

function Test-Command([string]$name) {
  return $null -ne (Get-Command $name -ErrorAction SilentlyContinue)
}

if (-not (Test-Command "docker")) {
  throw "Docker is required but was not found in PATH. Install Docker Desktop."
}

function Invoke-NativeCommand {
  param(
    [Parameter(Mandatory)][string]$FilePath,
    [Parameter()][string[]]$Arguments = @(),
    [switch]$Quiet
  )

  $oldPreference = $ErrorActionPreference
  $ErrorActionPreference = "Continue"
  try {
    if ($Quiet) {
      & $FilePath @Arguments 1>$null 2>$null
    } else {
      & $FilePath @Arguments
    }
    return $LASTEXITCODE
  } finally {
    $ErrorActionPreference = $oldPreference
  }
}

function Invoke-NativeOrThrow {
  param(
    [Parameter(Mandatory)][string]$FilePath,
    [Parameter()][string[]]$Arguments = @(),
    [Parameter(Mandatory)][string]$What
  )

  $exitCode = Invoke-NativeCommand -FilePath $FilePath -Arguments $Arguments
  if ($exitCode -ne 0) {
    throw "$What failed (exit code $exitCode)."
  }
}

function Get-DockerInfoText {
  $oldPreference = $ErrorActionPreference
  $ErrorActionPreference = "Continue"
  try {
    $text = (& cmd /d /c "docker info 2>&1" | Out-String).Trim()
    $exitCode = $LASTEXITCODE
    return @{ ExitCode = $exitCode; Text = $text }
  } finally {
    $ErrorActionPreference = $oldPreference
  }
}

function Get-ComposeCommand {
  $exitCode = Invoke-NativeCommand -FilePath "docker" -Arguments @("compose", "version") -Quiet
  if ($exitCode -eq 0) {
    return @{ FilePath = "docker"; BaseArgs = @("compose") }
  }
  if (Test-Command "docker-compose") {
    return @{ FilePath = "docker-compose"; BaseArgs = @() }
  }
  throw "Docker Compose is required but was not found. Install Docker Desktop or docker-compose."
}

$compose = Get-ComposeCommand
$composeFile = $compose.FilePath
$composeBaseArgs = $compose.BaseArgs

if ((Invoke-NativeCommand -FilePath "docker" -Arguments @("info") -Quiet) -ne 0) {
  $details = (Get-DockerInfoText).Text
  $message = "Docker daemon not reachable from this PowerShell session."
  if (-not [string]::IsNullOrWhiteSpace($details)) {
    $message += "`n`nDocker output:`n$details"
  }
  $message += "`n`nTips:`n- Ensure Docker Desktop shows 'Docker is running'`n- Try: docker context ls; docker context use default`n- If you're using WSL, ensure WSL integration is enabled"
  throw $message
}

$force = $env:MAGPIE_FORCE -eq "1"
if (Test-Path -LiteralPath $installDir) {
  if (-not $force) {
    throw "Refusing to install into existing path: $installDir. Delete it or set MAGPIE_FORCE=1."
  }
  Remove-Item -LiteralPath $installDir -Recurse -Force
}

New-Item -ItemType Directory -Path $installDir | Out-Null

function Download-File([string]$url, [string]$dest) {
  $params = @{ Uri = $url; OutFile = $dest }
  if ((Get-Command Invoke-WebRequest).Parameters.ContainsKey("UseBasicParsing")) {
    $params.UseBasicParsing = $true
  }
  Invoke-WebRequest @params
}

Write-Host "Downloading docker-compose.yml..."
Download-File $composeUrl (Join-Path $installDir "docker-compose.yml")

Write-Host "Downloading .env.example..."
try {
  Download-File $envExampleUrl (Join-Path $installDir ".env.example")
} catch {
  # best-effort
}

Set-Location -LiteralPath $installDir

function Escape-DotenvValue([string]$value) {
  $escaped = $value.Replace('\', '\\').Replace('"', '\"')
  return $escaped
}

if ($env:PROXY_ENCRYPTION_KEY) {
  $key = $env:PROXY_ENCRYPTION_KEY
} else {
  Write-Host "Enter PROXY_ENCRYPTION_KEY (will be saved to .env):"
  $s1 = Read-Host -AsSecureString
  Write-Host "Confirm PROXY_ENCRYPTION_KEY:"
  $s2 = Read-Host -AsSecureString

  $p1 = [Runtime.InteropServices.Marshal]::PtrToStringBSTR([Runtime.InteropServices.Marshal]::SecureStringToBSTR($s1))
  $p2 = [Runtime.InteropServices.Marshal]::PtrToStringBSTR([Runtime.InteropServices.Marshal]::SecureStringToBSTR($s2))

  if ([string]::IsNullOrEmpty($p1)) { throw "PROXY_ENCRYPTION_KEY cannot be empty." }
  if ($p1 -ne $p2) { throw "Keys did not match." }
  if ($p1.Contains("`n") -or $p1.Contains("`r")) { throw "PROXY_ENCRYPTION_KEY must be a single line." }

  $key = $p1
}

$escapedKey = Escape-DotenvValue $key

$envLines = @()
$envLines += "PROXY_ENCRYPTION_KEY=""$escapedKey"""
if ($env:MAGPIE_IMAGE_TAG) {
  $envLines += "MAGPIE_IMAGE_TAG=$($env:MAGPIE_IMAGE_TAG)"
}

$envPath = Join-Path (Get-Location) ".env"
$envContent = ($envLines -join "`n") + "`n"

# best-effort to restrict file ACLs; still works if it fails.
try {
  $null = New-Item -ItemType File -Path $envPath -Force
  & icacls $envPath /inheritance:r /grant:r "$($env:USERNAME):(R,W)" *> $null
} catch { }

[IO.File]::WriteAllText($envPath, $envContent, [Text.UTF8Encoding]::new($false))

Write-Host "Pulling images..."
Invoke-NativeOrThrow -FilePath $composeFile -Arguments @($composeBaseArgs + @("-f", "docker-compose.yml", "pull")) -What "docker compose pull"

Write-Host "Starting Magpie..."
Invoke-NativeOrThrow -FilePath $composeFile -Arguments @($composeBaseArgs + @("-f", "docker-compose.yml", "up", "-d")) -What "docker compose up"

$composeDisplay = ((@($composeFile) + $composeBaseArgs) -join " ")
Write-Host ""
Write-Host "Magpie is up."
Write-Host "- UI:  http://localhost:5050"
Write-Host "- API: http://localhost:5656/api"
Write-Host ""
Write-Host "To stop:"
Write-Host "  cd `"$installDir`" && $composeDisplay down"
