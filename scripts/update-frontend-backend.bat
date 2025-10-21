@echo off
setlocal enabledelayedexpansion

set "SCRIPT_DIR=%~dp0"
pushd "%SCRIPT_DIR%\.."

where docker >nul 2>&1
if errorlevel 1 (
  echo Docker is required but was not found. Install Docker Desktop.
  popd
  exit /b 1
)

docker compose version >nul 2>&1
if not errorlevel 1 (
  set "COMPOSE_EXE=docker"
  set "COMPOSE_ARGS=compose"
) else (
  where docker-compose >nul 2>&1
  if errorlevel 1 (
    echo Docker Compose is required but was not found. Install Docker Desktop or docker-compose.
    popd
    exit /b 1
  )
  set "COMPOSE_EXE=docker-compose"
  set "COMPOSE_ARGS="
)

set "STASHED=0"
echo Checking for local changes...
for /f "delims=" %%i in ('git status --porcelain') do (
  set "STASHED=1"
  goto doStash
)
:doStash
if "%STASHED%"=="1" (
  echo Local changes detected. Temporarily stashing...
  git stash push --include-untracked >nul
  if errorlevel 1 (
    echo Failed to stash local changes. Resolve them manually and rerun.
    popd
    exit /b 1
  )
)

echo Pulling latest changes...
git pull --ff-only
if errorlevel 1 (
  if "%STASHED%"=="1" (
    echo Restoring stashed changes after failed pull...
    git stash pop
    if errorlevel 1 (
      echo Automatic restore of stashed changes failed. Run "git stash pop" manually.
    )
  )
  popd
  exit /b %ERRORLEVEL%
)

if "%STASHED%"=="1" (
  echo Restoring local changes...
  git stash pop
  if errorlevel 1 (
    echo Automatic restore failed. Run "git stash pop" manually.
    popd
    exit /b 1
  )
)

echo Rebuilding frontend and backend containers...
for /f "delims=" %%i in ('git rev-parse --short HEAD') do set "MAGPIE_GIT_COMMIT=%%i"
echo Embedding frontend commit %MAGPIE_GIT_COMMIT%
if "%COMPOSE_ARGS%"=="" (
  call "%COMPOSE_EXE%" up -d --build frontend backend
) else (
  call "%COMPOSE_EXE%" %COMPOSE_ARGS% up -d --build frontend backend
)

if errorlevel 1 (
  popd
  exit /b %ERRORLEVEL%
)

echo Done. Frontend is available at http://localhost:8080 and backend API at http://localhost:8082/api
popd
exit /b 0
