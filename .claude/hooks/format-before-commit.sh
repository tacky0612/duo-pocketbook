#!/usr/bin/env bash
# PreToolUse フック: `git commit` を実行する Bash コマンドの直前に、
# ステージ済みの Go ファイルと Terraform ファイルをフォーマットして再ステージする。
#
# Claude Code から stdin に渡される JSON の tool_input.command を見て、
# git commit の場合のみ処理する。それ以外のコマンドでは何もしない。
set -euo pipefail

# go / terraform を確実に見つけられるように PATH を補う
export PATH="/opt/homebrew/bin:$PATH"

input="$(cat)"
command="$(printf '%s' "$input" | jq -r '.tool_input.command // ""')"

# `git commit` を含むコマンドのときだけフォーマットする
case "$command" in
  *"git commit"*) ;;
  *) exit 0 ;;
esac

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || echo .)"
cd "$repo_root"

changed=0

# --- Go: ステージ済みの .go をフォーマットして再ステージ ---
if command -v gofmt >/dev/null 2>&1; then
  go_files="$(git diff --cached --name-only --diff-filter=ACM -- '*.go')"
  if [ -n "$go_files" ]; then
    printf '%s\n' "$go_files" | while IFS= read -r f; do
      [ -n "$f" ] || continue
      gofmt -w "$f"
      git add -- "$f"
    done
    changed=1
  fi
fi

# --- Terraform: terraform/ をフォーマットしてステージ済みの .tf を再ステージ ---
if command -v terraform >/dev/null 2>&1 && [ -d terraform ]; then
  terraform -chdir=terraform fmt >/dev/null 2>&1 || true
  tf_files="$(git diff --cached --name-only --diff-filter=ACM -- 'terraform/*.tf')"
  if [ -n "$tf_files" ]; then
    printf '%s\n' "$tf_files" | while IFS= read -r f; do
      [ -n "$f" ] || continue
      git add -- "$f"
    done
    changed=1
  fi
fi

if [ "$changed" -eq 1 ]; then
  echo "コミット前に gofmt / terraform fmt を適用し、対象ファイルを再ステージしました。" >&2
fi

exit 0
