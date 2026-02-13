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

# Replace the gh-pages root content with homepage build output while preserving docs and worktree metadata.
find "$WORKTREE" -mindepth 1 -maxdepth 1 ! -name '.git' ! -name 'docs' -exec rm -rf {} +
cp -R "$DIST_DIR"/. "$WORKTREE"

# Sanity check: ensure index.html references existing asset files.
main_js="$(sed -n 's/.*<script type=\"module\" crossorigin src=\"\\(\\/assets\\/[^\" ]*\\.js\\)\".*/\\1/p' "$WORKTREE/index.html" | head -n 1)"
main_css="$(sed -n 's/.*<link rel=\"stylesheet\" crossorigin href=\"\\(\\/assets\\/[^\" ]*\\.css\\)\".*/\\1/p' "$WORKTREE/index.html" | head -n 1)"
if [ -n "$main_js" ] && [ ! -f "$WORKTREE/${main_js#/}" ]; then
  echo "Deploy aborted: missing referenced JS asset $main_js"
  exit 1
fi
if [ -n "$main_css" ] && [ ! -f "$WORKTREE/${main_css#/}" ]; then
  echo "Deploy aborted: missing referenced CSS asset $main_css"
  exit 1
fi

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
