# beadmap

beads（[bd CLI](https://github.com/gastownhall/beads)）のタスクを **1 画面で俯瞰する**
読み取り専用ビューア。サマリー・親子ツリー・依存関係・停滞が一目で見える。
beads が正本、beadmap はその「読み取り用の地図」— 地図は現地を書き換えない。

AI 開発ハーネス連載（Zenn）の一環として、開発過程（devlog / ADR / beads 履歴）ごと公開する。

## Problem

- CLI ベースのタスク管理（beads）は個々のタスク操作には強いが、「全体でどこまで進んだか」
  「何が詰まっているか」の俯瞰が弱い
- 見える化のために正本へ書き込むツールを増やすと、データを壊すリスクと二重管理を持ち込む

## Solution

- 1 画面で全体像を俯瞰する: クリックで切り替わるサマリータイル・epic⇔子展開のツリー・
  依存を一段先まで見せる詳細ペイン
- 進行中タスクは「更新が古い順」に並べ、**停滞しているものから目に入る**
- 閲覧専用に徹する（起票・更新は bd のまま）。read-only は「気をつける」ではなく
  **構造とテスト** で保証する（下記の契約）。だから安心して常時開いておける

## 使い方

```bash
go install github.com/syotakokichi/beadmap@latest

beadmap              # カレントディレクトリの .beads/ を表示
beadmap -dir <repo>  # 別リポジトリの .beads/ を表示
beadmap -no-open     # ブラウザを自動で開かない
beadmap -port 8080   # 優先ポートの指定
```

- 既定の優先ポートは 23237（電話キーパッドで "beads"）。使用中なら自動で別ポートに回避する
- バイナリ 1 つ・外部依存ゼロ。Node も DB も不要

## Status

v1 実装済み（collector / server / ui の 3 層 + read-only 契約テスト）。
残作業は公開ゲート通過 → GitHub 公開 → 実運用一巡。
install 不要で気軽に試せる **静的デモページ**（サンプルデータ同梱・ブラウザ内描画のみ）も
v1 公開後に用意する予定。

## read-only 契約

beadmap が正本を壊さないことは、以下の 5 項目を **構造** で保証し、
それぞれに対応するテストを置いて固定している。

| # | 契約 | 対応テスト |
| --- | --- | --- |
| 1 | `127.0.0.1` にのみバインドする（LAN・外部公開の手段を持たない） | `TestListenLocalBindsLoopbackOnly` |
| 2 | GET 以外の HTTP メソッドはすべて 405 で拒否する | `TestWriteMethodsRejected` |
| 3 | bd CLI の実行は読み取り専用の固定引数（`ready --json` / `blocked --json`）のみ。任意コマンドを組み立てない | `TestBDArgsAreFixed` |
| 4 | 外部サービスへの自動アクセスをしない（依存ゼロ・テレメトリなし） | `TestGoModHasNoDependencies` / `TestNoOutboundNetworkCalls` |
| 5 | データ取得に失敗した時は、前回スナップショットを「古い」と明示して表示する | `TestStaleServesLastGoodSnapshot` |

書き込み（起票・更新・close）は bd CLI の役割であり、beadmap には今後も追加しない。
エージェント向けの実装規約は [.claude/rules/read-only-contract.md](.claude/rules/read-only-contract.md) を参照。

## データソースと鮮度

- 入力は `.beads/issues.jsonl`。これは beads 公式の位置づけで **viewer・相互運用向けの
  export** であり、正本（Dolt DB）そのものではない
- そのため画面には常に「ファイル更新時刻・取得時刻・ready の算出元」を表示する
- bd が PATH にあれば ready / blocked はライブクエリで取得する。**bd 導入時はライブクエリが正**
- bd 未導入時は jsonl の依存関係から近似計算に fallback する（bd と完全一致する保証はない。
  一致は突き合わせテストで確認している）

## 公開境界

この repo は実務で得た知見を一般化して自分の言葉で再実装した公開デモであり、
実務のコード・画面・固有情報は含まない。それを担保するため:

- `scripts/boundary-check.sh` — 非公開の語リスト（repo 外）による fail-closed の禁止語ゲート。
  全 push 前に必ず実行する
- 公開前チェック 4 点 — (1) コード・プロンプトは自作 (2) スクリーンショットは自分の環境・
  自作物のみ (3) コミュニティ投稿・チャットの引用なし (4) 有料コンテンツをなぞっていない

詳細は [.claude/rules/boundary-check.md](.claude/rules/boundary-check.md) を参照。

## 開発

```bash
go vet ./...
go test ./...

# fallback と bd ライブクエリの突き合わせ（bd 導入環境のみ）
BEADMAP_BD_TEST_DIR=<.beads を含む repo> go test ./internal/collector/ -run TestFallbackMatchesBD -v
```

- アーキテクチャ: collector（収集）/ server（配信）/ ui（表示）の 3 層。
  collector / server に単体テスト・契約テスト（UI ロジックの単体テストは bm-rvv で追補予定）
- 設計判断は [docs/adr/](docs/adr/)、開発記録は [docs/devlog/](docs/devlog/)
- この repo 自身のタスクもこの repo の beads（prefix `bm`）で管理している
  （beadmap で beadmap の開発を映すドッグフーディング）

## License

[MIT](LICENSE)
