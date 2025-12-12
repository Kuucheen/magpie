$ErrorActionPreference = "Stop"

$repoOwner = if ($env:MAGPIE_REPO_OWNER) { $env:MAGPIE_REPO_OWNER } else { "Kuucheen" }
$repoName  = if ($env:MAGPIE_REPO_NAME)  { $env:MAGPIE_REPO_NAME }  else { "magpie" }
$repoRef   = if ($env:MAGPIE_REPO_REF)   { $env:MAGPIE_REPO_REF }   else { "main" }

$installDir = if ($env:MAGPIE_INSTALL_DIR) { $env:MAGPIE_INSTALL_DIR } else { "magpie" }

if ([string]::IsNullOrWhiteSpace($installDir) -or $installDir -eq "\") {
  throw "MAGPIE_INSTALL_DIR must not be empty or '\'."
}

if (-not (Test-Path -LiteralPath $installDir)) {
  throw "Install directory not found: $installDir. Run the installer first or set MAGPIE_INSTALL_DIR."
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

function Get-ComposeCommand {
  try {
    & docker compose version *> $null
    return @("docker", "compose")
  } catch {
    if (Test-Command "docker-compose") {
      return @("docker-compose")
    }
    throw "Docker Compose is required but was not found. Install Docker Desktop or docker-compose."
  }
}

$composeCmd = Get-ComposeCommand

try {
  & docker info *> $null
} catch {
  throw "Docker daemon not reachable. Start Docker Desktop and rerun."
}

Set-Location -LiteralPath $installDir

if (Test-Path -LiteralPath ".env") {
  $envContent = Get-Content -LiteralPath ".env" -Raw
  if ($envContent -notmatch "(?m)^PROXY_ENCRYPTION_KEY=") {
    throw "Missing PROXY_ENCRYPTION_KEY in $installDir\\.env"
  }
} elseif (-not $env:PROXY_ENCRYPTION_KEY) {
  throw "Missing $installDir\\.env (required for PROXY_ENCRYPTION_KEY). Restore it or set PROXY_ENCRYPTION_KEY and rerun."
}

function Download-File([string]$url, [string]$dest) {
  Invoke-WebRequest -Uri $url -OutFile $dest -UseBasicParsing
}

Write-Host "Downloading latest docker-compose.yml..."
$tmpCompose = "docker-compose.yml.new.$PID"
Download-File $composeUrl $tmpCompose

if (Test-Path -LiteralPath "docker-compose.yml") {
  Copy-Item -LiteralPath "docker-compose.yml" -Destination "docker-compose.yml.bak" -Force
}
Move-Item -LiteralPath $tmpCompose -Destination "docker-compose.yml" -Force

Write-Host "Refreshing .env.example (optional)..."
try {
  Download-File $envExampleUrl ".env.example"
} catch {
  # best-effort
}

Write-Host "Pulling images..."
& $composeCmd -f docker-compose.yml pull

Write-Host "Applying update..."
& $composeCmd -f docker-compose.yml up -d

Write-Host "Done."
