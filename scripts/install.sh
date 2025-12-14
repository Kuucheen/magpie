#!/usr/bin/env bash
set -euo pipefail

REPO_OWNER="${MAGPIE_REPO_OWNER:-Kuucheen}"
REPO_NAME="${MAGPIE_REPO_NAME:-magpie}"
REPO_REF="${MAGPIE_REPO_REF:-main}"

INSTALL_DIR="${MAGPIE_INSTALL_DIR:-magpie}"
COMPOSE_URL="${MAGPIE_COMPOSE_URL:-https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/${REPO_REF}/docker-compose.yml}"
ENV_EXAMPLE_URL="${MAGPIE_ENV_EXAMPLE_URL:-https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/${REPO_REF}/.env.example}"

if [[ -z "${INSTALL_DIR}" || "${INSTALL_DIR}" == "/" ]]; then
  echo "MAGPIE_INSTALL_DIR must not be empty or '/'." >&2
  exit 1
fi

if docker compose version >/dev/null 2>&1; then
  compose_cmd=(docker compose)
elif command -v docker-compose >/dev/null 2>&1; then
  compose_cmd=(docker-compose)
else
  echo "Docker Compose is required but was not found. Install Docker Desktop or docker-compose." >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is required but was not found in PATH." >&2
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  err="$(docker info 2>&1 || true)"
  echo "Docker daemon not reachable from this shell." >&2
  if [ -n "${err}" ]; then
    echo >&2
    echo "Docker output:" >&2
    echo "${err}" >&2
  fi
  echo >&2
  if printf '%s' "${err}" | grep -qi "permission denied"; then
    echo "Tip (Linux): your user may not have access to the Docker socket." >&2
    echo "  - Try: sudo usermod -aG docker \"$USER\"  (then log out/in)" >&2
    echo "  - Or rerun with sudo (not recommended long-term): curl ... | sudo bash" >&2
  else
    echo "Tip:" >&2
    echo "  - Ensure Docker Desktop/Engine is running" >&2
    echo "  - Check: docker context show && docker context ls" >&2
  fi
  exit 1
fi

if [ -e "${INSTALL_DIR}" ] && [ "${MAGPIE_FORCE:-0}" != "1" ]; then
  echo "Refusing to install into existing path: ${INSTALL_DIR}" >&2
  echo "Delete it or re-run with MAGPIE_FORCE=1." >&2
  exit 1
fi

rm -rf "${INSTALL_DIR}"
mkdir -p "${INSTALL_DIR}"

download() {
  local url="$1"
  local dest="$2"

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "${url}" -o "${dest}"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "${dest}" "${url}"
  else
    echo "Need curl or wget to download files." >&2
    exit 1
  fi
}

echo "Downloading docker-compose.yml..."
download "${COMPOSE_URL}" "${INSTALL_DIR}/docker-compose.yml"

echo "Downloading .env.example..."
if ! download "${ENV_EXAMPLE_URL}" "${INSTALL_DIR}/.env.example"; then
  : # best-effort; we'll still proceed
fi

cd "${INSTALL_DIR}"

if [ -n "${PROXY_ENCRYPTION_KEY:-}" ]; then
  key="${PROXY_ENCRYPTION_KEY}"
else
  echo "Enter PROXY_ENCRYPTION_KEY (will be saved to .env):"
  read -r -s key
  echo
  if [ -z "${key}" ]; then
    echo "PROXY_ENCRYPTION_KEY cannot be empty." >&2
    exit 1
  fi
  echo "Confirm PROXY_ENCRYPTION_KEY:"
  read -r -s key2
  echo
  if [ "${key}" != "${key2}" ]; then
    echo "Keys did not match." >&2
    exit 1
  fi
fi

if [[ "${key}" == *$'\n'* || "${key}" == *$'\r'* ]]; then
  echo "PROXY_ENCRYPTION_KEY must be a single line." >&2
  exit 1
fi

escaped_key="${key//\\/\\\\}"
escaped_key="${escaped_key//\"/\\\"}"

umask 077
{
  printf "PROXY_ENCRYPTION_KEY=\"%s\"\n" "${escaped_key}"
  if [ -n "${MAGPIE_IMAGE_TAG:-}" ]; then
    printf "MAGPIE_IMAGE_TAG=%s\n" "${MAGPIE_IMAGE_TAG}"
  fi
} > .env

echo "Pulling images..."
"${compose_cmd[@]}" -f docker-compose.yml pull

echo "Starting Magpie..."
"${compose_cmd[@]}" -f docker-compose.yml up -d

compose_display="${compose_cmd[*]}"
cat <<EOF

Magpie is up.
- UI:  http://localhost:5050
- API: http://localhost:5656/api

To stop:
  cd "${INSTALL_DIR}" && ${compose_display} down
EOF
