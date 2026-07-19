# /verify - 最小検証

実装の最小検証を実行してください。

## 手順

1. **Go 検証**: `go vet ./...` → `go test ./...`
   - read-only 契約テスト（`internal/server` / `internal/collector`）が含まれる
   - fallback と bd の突き合わせは `BEADMAP_BD_TEST_DIR=<.beads を含む repo>` を
     指定した時のみ実行される integration テスト（CI では常にスキップ）
2. **Markdown lint**: `npx markdownlint-cli2 "*.md" "docs/**/*.md"`
   - 設定は `.markdownlint-cli2.jsonc`（CI の対象 glob と揃える）
3. **リンクチェック**: `lychee --offline --no-progress './**/*.md'`
4. **境界ゲート**: `/boundary-check`（push する場合は必須）

## 報告

- 各ステップの結果（green / 失敗内容と原因）をまとめて報告する
- push するかどうかの判断はユーザーに委ねる
