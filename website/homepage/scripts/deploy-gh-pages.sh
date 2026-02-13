#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
SITE_DIR="$ROOT/website/homepage"
DIST_DIR="$SITE_DIR/dist"
WORKTREE="$ROOT/.gh-pages"
BRANCH="gh-pages"
pushed=false

cleanup() {
  git worktree prune >/dev/null 2>&1 || true
  if [ -d "$WORKTREE" ]; then
    git worktree remove "$WORKTREE" --force >/dev/null 2>&1 || rm -rf "$WORKTREE"
  fi
}

trap cleanup EXIT

cd "$SITE_DIR"

npm run build

if [ -d "$WORKTREE" ]; then
  cleanup
fi

git worktree prune >/dev/null 2>&1 || true

if git show-ref --verify --quiet "refs/heads/$BRANCH"; then
  git worktree add -f "$WORKTREE" "$BRANCH"
else
  git worktree add -f -B "$BRANCH" "$WORKTREE"
fi

# Sync homepage files to gh-pages root while preserving docs and the worktree git marker.
rsync -a --delete --exclude '.git' --exclude 'docs' "$DIST_DIR"/ "$WORKTREE"/

cd "$WORKTREE"

git add -A
if git diff --cached --quiet; then
  echo "No changes to deploy."
else
  git commit -m "Deploy homepage"
  git push -u origin "$BRANCH"
  pushed=true
fi

if [ "$pushed" = true ]; then
  echo "Homepage push complete: origin/$BRANCH updated."
else
  echo "Homepage push skipped: no new changes."
fi
