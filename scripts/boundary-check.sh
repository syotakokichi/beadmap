#!/usr/bin/env bash
# 境界チェックゲート（push 前に必ず実行する）
#
# 非公開の語リスト（repo 外）に載っている語が repo 内に 1 件でもあれば失敗する。
# リストが読めない場合も失敗する（fail-closed: 検査できない状態では通さない）。
# CI では実行しない: 語リストが非公開で CI から参照できないため、
# ローカルの push 前ゲートとして運用する。運用ルール: .claude/rules/boundary-check.md
set -euo pipefail

LIST="${BOUNDARY_WORDS_FILE:-$HOME/.config/beadmap/boundary-words.txt}"

cd "$(git rev-parse --show-toplevel)"

if [[ ! -r "$LIST" ]]; then
  echo "NG: 語リストが読めません: $LIST" >&2
  echo "    fail-closed のため停止します。リストを配置してから再実行してください。" >&2
  exit 1
fi

# 検査対象1: git 追跡ファイル + 未追跡ファイル（.gitignore 対象は除外）
list_files() { git ls-files --cached --others --exclude-standard -z; }

# 検査対象2: 全コミット履歴。公開時は過去コミットもすべて push されるため、
# 現在のツリーから削除済みの内容も検査する（fail-closed の対象を履歴まで広げる）
commits="$(git rev-list --all 2>/dev/null || true)"

status=0
words=0
while IFS= read -r word; do
  # 空行と # で始まるコメント行はスキップ
  [[ -z "$word" || "$word" == \#* ]] && continue
  words=$((words + 1))
  hits="$(list_files | xargs -0 grep -nIiF -- "$word" 2>/dev/null || true)"
  if [[ -n "$hits" ]]; then
    echo "NG: 禁止語ヒット:" >&2
    echo "$hits" >&2
    status=1
  fi
  if [[ -n "$commits" ]]; then
    hist_hits="$(git grep -nIiF -- "$word" $commits 2>/dev/null || true)"
    if [[ -n "$hist_hits" ]]; then
      echo "NG: 禁止語ヒット（git 履歴内）:" >&2
      echo "$hist_hits" | head -20 >&2
      status=1
    fi
  fi
done < "$LIST"

if [[ "$status" -ne 0 ]]; then
  echo "NG: boundary-check failed" >&2
  exit 1
fi

file_count="$(git ls-files --cached --others --exclude-standard | wc -l | tr -d ' ')"
echo "OK: boundary-check passed（検査語 ${words} 語・ヒット 0 件・対象 ${file_count} ファイル）"
