#!/usr/bin/env bash
# 境界チェックゲート（push 前に必ず実行する）
#
# 非公開の語リスト（repo 外）に載っている語が repo 内に 1 件でもあれば失敗する。
# fail-closed: 検査できない状態では通さない。
#   - リストが読めない / 有効語が 0 語 → 失敗
#   - grep・git の実行エラー（exit 2 以上） → 「ヒットなし」と区別して失敗
# 検査対象: 作業ツリー（追跡+未追跡）/ 全コミット履歴の blob / コミットメタデータ
# （author・committer・メッセージ）。
# 通常はローカルの push 前ゲート。デモデータ自動更新の CI では語リストを
# Actions secret から BOUNDARY_WORDS_FILE 経由で渡す（運用: .claude/rules/boundary-check.md）
set -euo pipefail

LIST="${BOUNDARY_WORDS_FILE:-$HOME/.config/beadmap/boundary-words.txt}"

cd "$(git rev-parse --show-toplevel)"

if [[ ! -r "$LIST" ]]; then
  echo "NG: 語リストが読めません: $LIST" >&2
  echo "    fail-closed のため停止します。リストを配置してから再実行してください。" >&2
  exit 1
fi

# 全コミット履歴（取得失敗は fail-closed で即停止。shallow clone では履歴全体を
# 検査できないため、CI 側は fetch-depth: 0 で呼ぶこと）
commits="$(git rev-list --all)"

# コミットメタデータ（author / committer / メッセージ）。blob 検査では拾えない
metadata="$(git log --all --format='%H %an <%ae> %cn <%ce>%n%s%n%b')"

# grep の終了コードを「0=ヒット / 1=ヒットなし / 2以上=検査失敗」として扱う
# 戻り値: 0=ヒットなし / 1=ヒットあり / 2=検査失敗
scan() { # $1=説明 $2..=コマンド
  local desc="$1"; shift
  local out st
  set +e
  out="$("$@" 2>/dev/null)"
  st=$?
  set -e
  if [[ $st -eq 0 && -n "$out" ]]; then
    echo "NG: 禁止語ヒット（${desc}）:" >&2
    echo "$out" | head -20 >&2
    return 1
  elif [[ $st -ge 2 ]]; then
    echo "NG: 検査の実行に失敗（${desc}・exit ${st}）。fail-closed のため通しません。" >&2
    return 2
  fi
  return 0
}

status=0
words=0
while IFS= read -r word; do
  word="${word%$'\r'}" # CRLF 由来の \r を除去（偽陰性防止）
  word="${word#"${word%%[![:space:]]*}"}" # 前後空白を除去してから無効行判定（空白付きコメントの素通り防止）
  word="${word%"${word##*[![:space:]]}"}"
  [[ -z "$word" || "$word" == \#* ]] && continue
  words=$((words + 1))

  scan "作業ツリー" git grep --untracked -nIiF -- "$word" || status=1
  if [[ -n "$commits" ]]; then
    # shellcheck disable=SC2086
    scan "git 履歴 blob" git grep -nIiF -- "$word" $commits || status=1
  fi
  scan "コミットメタデータ" grep -niF -- "$word" <<<"$metadata" || status=1
done < "$LIST"

# 有効語 0 語は「検査していない」のと同じ（コメントだけのリスト等）— fail-closed
if [[ "$words" -eq 0 ]]; then
  echo "NG: 語リストに有効な語がありません（${LIST}）。fail-closed のため通しません。" >&2
  exit 1
fi

if [[ "$status" -ne 0 ]]; then
  echo "NG: boundary-check failed" >&2
  exit 1
fi

file_count="$(git ls-files --cached --others --exclude-standard | wc -l | tr -d ' ')"
echo "OK: boundary-check passed（検査語 ${words} 語・ヒット 0 件・対象 ${file_count} ファイル・履歴/メタデータ込み）"
