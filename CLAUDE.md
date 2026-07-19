# CLAUDE.md - beadmap 開発ガイド

beads（bd CLI）のタスクデータを **読むだけ** のローカル進捗ビューア。
beads が正本、beadmap はその「読み取り用の地図」。地図は現地を書き換えない
（プロダクト概要は [README](README.md) を参照）。

## 公開境界（最重要）

- この repo は **公開されている**。公開は不可逆なので、**全 push 前に `/boundary-check` を必ず実行する**
- 境界ゲートは fail-closed: 非公開の語リスト（`~/.config/beadmap/boundary-words.txt`）が
  読めなければ exit 1 で停止する
- 境界チェックは CI に含めない（語リストが非公開のため）。ローカルの push 前ゲートとして運用する
- スクリーンショットは画像内テキストを grep で検査できない。公開前に目視チェックの記録を
  devlog に残す
- 詳細: [.claude/rules/boundary-check.md](.claude/rules/boundary-check.md)

## read-only 契約（このツールの本質）

beadmap は閲覧専用に徹する。以下はエージェントへの恒久制約:

- 書き込みエンドポイント（POST 等・起票/更新/close の API）を **作らない**。v1.1 拡張でも作らない
- `127.0.0.1` 以外にバインドする手段を **作らない**
- bd の実行は読み取り専用の固定引数のみ。引数の動的組み立ては **しない**
- 外部サービスへの自動アクセスを **追加しない**
- 契約の各項目には対応するテストがある。テストを弱める変更は契約変更であり、
  README の契約表と同時に更新して人間の承認を得る
- 詳細: [.claude/rules/read-only-contract.md](.claude/rules/read-only-contract.md)

## リポジトリ構成

| パス | 役割 |
| --- | --- |
| `main.go` | エントリポイント（flag・go:embed・ブラウザ起動） |
| `internal/collector/` | データ収集層（jsonl parser・bd ライブクエリ・fallback 計算） |
| `internal/server/` | 配信層（127.0.0.1・GET のみ・JSON API・静的 UI 配信） |
| `ui/` | 表示層（素の HTML/CSS/JS・ビルドなし・go:embed で同梱） |
| `docs/devlog/` | 時系列の開発記録 |
| `docs/adr/` | 設計判断の記録（MADR-lite） |
| `scripts/` | 境界チェック等の運用スクリプト |
| `.claude/` | エージェント向けルール・コマンド |

## タスク運用（beads）

起票 → claim → 実装 → `/verify` → `/boundary-check` → push の順で進める。
この repo 自身のタスクもこの repo の beads（prefix `bm`）で管理し、
beadmap で自分自身を映す（ドッグフーディング）。

<!-- bd 生成の managed block。bd のアップデートと共存させるため内容は変更せず、lint のみ除外する -->
<!-- markdownlint-disable MD031 MD032 MD034 -->
<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:6cd5cc61 -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

**Architecture in one line:** issues live in a local Dolt DB; sync uses `refs/dolt/data` on your git remote; `.beads/issues.jsonl` is a passive export. See https://github.com/gastownhall/beads/blob/main/docs/SYNC_CONCEPTS.md for details and anti-patterns.

## Agent Context Profiles

The managed Beads block is task-tracking guidance, not permission to override repository, user, or orchestrator instructions.

- **Conservative (default)**: Use `bd` for task tracking. Do not run git commits, git pushes, or Dolt remote sync unless explicitly asked. At handoff, report changed files, validation, and suggested next commands.
- **Minimal**: Keep tool instruction files as pointers to `bd prime`; use the same conservative git policy unless active instructions say otherwise.
- **Team-maintainer**: Only when the repository explicitly opts in, agents may close beads, run quality gates, commit, and push as part of session close. A current "do not commit" or "do not push" instruction still wins.

## Session Completion

This protocol applies when ending a Beads implementation workflow. It is subordinate to explicit user, repository, and orchestrator instructions.

1. **File issues for remaining work** - Create beads for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Handle git/sync by active profile**:
   ```bash
   # Conservative/minimal/default: report status and proposed commands; wait for approval.
   git status

   # Team-maintainer opt-in only, unless current instructions forbid it:
   git pull --rebase
   git push
   git status
   ```
5. **Hand off** - Summarize changes, validation, issue status, and any blocked sync/commit/push step

**Critical rules:**
- Explicit user or orchestrator instructions override this Beads block.
- Do not commit or push without clear authority from the active profile or the current user request.
- If a required sync or push is blocked, stop and report the exact command and error.
<!-- END BEADS INTEGRATION -->
<!-- markdownlint-enable MD031 MD032 MD034 -->

## 検証

- `/verify` = `go vet ./...` + `go test ./...` + markdownlint + lychee + 境界ゲート
- fallback と bd の突き合わせは integration テスト（環境変数 `BEADMAP_BD_TEST_DIR` 指定時のみ実行）
- CI: `ci.yml`（go test + vet + build / markdownlint / lychee offline）

## セキュリティ既定

- `.env` 系ファイル・秘密情報を一切持たない（リポジトリにもバイナリにも）
- ネットワークは loopback のみ。テレメトリ・外部送信なし
