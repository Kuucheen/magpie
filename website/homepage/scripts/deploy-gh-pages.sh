#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
SITE_DIR="$ROOT/website/homepage"
DIST_DIR="$SITE_DIR/dist"
WORKTREE="$ROOT/.gh-pages"
BRANCH="gh-pages"

cd "$SITE_DIR"

npm run build

if [ -d "$WORKTREE" ]; then
  git worktree remove "$WORKTREE" --force || rm -rf "$WORKTREE"
fi

if git show-ref --verify --quiet "refs/heads/$BRANCH"; then
  git worktree add "$WORKTREE" "$BRANCH"
else
  git worktree add -B "$BRANCH" "$WORKTREE"
fi

rm -rf "$WORKTREE"/*
cp -R "$DIST_DIR"/. "$WORKTREE"

cd "$WORKTREE"

git add -A
if git diff --cached --quiet; then
  echo "No changes to deploy."
else
  git commit -m "Deploy homepage"
  git push -u origin "$BRANCH"
fi

cd "$ROOT"

git worktree remove "$WORKTREE" --force
