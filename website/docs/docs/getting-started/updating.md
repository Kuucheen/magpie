# Updating

## If you installed via one-command installer

### macOS/Linux

```bash
curl -fsSL https://raw.githubusercontent.com/Kuucheen/magpie/refs/heads/master/scripts/update.sh | bash
```

### Windows (PowerShell)

```powershell
iwr -useb https://raw.githubusercontent.com/Kuucheen/magpie/refs/heads/master/scripts/update.ps1 | iex
```

## If you cloned manually

### macOS/Linux

```bash
./scripts/update-frontend-backend.sh
```

### Windows (CMD)

```bat
scripts\update-frontend-backend.bat
```

## Key safety requirement

Keep `PROXY_ENCRYPTION_KEY` stable during updates. If you rotate it unintentionally, previously encrypted data becomes unreadable.
