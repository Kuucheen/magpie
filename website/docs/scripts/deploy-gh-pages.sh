#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
DOCS_DIR="$ROOT/website/docs"
BUILD_DIR="$DOCS_DIR/build"
WORKTREE="$ROOT/.gh-pages-docs-worktree"
BRANCH="gh-pages"
TARGET_DIR="$WORKTREE/docs"

cleanup() {
  if [ -d "$WORKTREE" ]; then
    git -C "$ROOT" worktree remove "$WORKTREE" --force >/dev/null 2>&1 || rm -rf "$WORKTREE"
  fi
}

trap cleanup EXIT

cd "$DOCS_DIR"
npm run build

if [ -d "$WORKTREE" ]; then
  git -C "$ROOT" worktree remove "$WORKTREE" --force || rm -rf "$WORKTREE"
fi

if git -C "$ROOT" show-ref --verify --quiet "refs/heads/$BRANCH"; then
  git -C "$ROOT" worktree add "$WORKTREE" "$BRANCH"
else
  git -C "$ROOT" worktree add -B "$BRANCH" "$WORKTREE"
fi

mkdir -p "$TARGET_DIR"
find "$TARGET_DIR" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
cp -R "$BUILD_DIR"/. "$TARGET_DIR"

cd "$WORKTREE"
git add -A docs

if git diff --cached --quiet; then
  echo "No docs changes to deploy."
else
  git commit -m "Deploy docs"
  git push origin "$BRANCH"
fi

echo "Docs deployed to $BRANCH/docs"
