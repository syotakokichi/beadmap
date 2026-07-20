# 境界チェックルール

公開 repo に「公開してはならない情報」が混入するのを push 前に検査する共通ルール。
このリポジトリは公開が不可逆である前提で、push を唯一のゲートポイントとする。

## 5 項目

| # | 項目 | 内容 |
| --- | --- | --- |
| 1 | 禁止語 grep 0 件 | 非公開の語リスト（`~/.config/beadmap/boundary-words.txt`）に載る語が repo 内に 0 件。`scripts/boundary-check.sh` で機械検査する |
| 2 | 公開前チェック 4 点 | (1) コード・プロンプトは自作 (2) スクリーンショットは自分の環境・自作物のみ (3) コミュニティ投稿・チャットの引用なし (4) 有料コンテンツをなぞっていない |
| 3 | 入力ソース宣言 | 作業セッションの入力が公開可能な情報のみであることを devlog に記録する |
| 4 | devlog 境界 | 意思決定支援ツール（候補比較・赤入れレビュー等）を使った場合は採用結果と理由のみを記録する。プロンプト・スクリーンショット・生成過程は含めない |
| 5 | スクリーンショット境界 | 画像内のテキストは grep で検査できない。公開するスクリーンショットは、境界チェック通過後の repo・自作データのみを映していることを **目視** で確認し、確認日・確認観点を devlog に記録する |

## fail-closed

`scripts/boundary-check.sh` は「検査できない状態では通さない」を既定とする。

- 語リストが読めない → exit 1
- 語リストに有効な語が 0 語（コメント・空行のみ）→ exit 1
- grep / git の実行エラー（exit 2 以上）→ 「ヒットなし」と区別して exit 1

検査対象は 作業ツリー / 全コミット履歴の blob / コミットメタデータ
（author・committer・メッセージ）。shallow clone では履歴全体を検査できないため、
CI から呼ぶ場合は `fetch-depth: 0` を必須とする。

## CI での扱い（原則ローカル・例外はデモデータ自動更新のみ）

語リストは非公開（repo 外・`~/.config/beadmap/`）で管理するため、
**通常の push 前ゲートはローカル実行を一次とする**（PR/push の CI には含めない）。

例外として、デモデータ自動更新（`.github/workflows/update-demo-data.yml`）に限り、
語リストを GitHub Actions の encrypted secret `BOUNDARY_WORDS` として渡し、
`BOUNDARY_WORDS_FILE` 経由で同じスクリプトを CI 実行する。この例外の条件:

- secret 未設定・空なら更新を中止する（fail-closed 維持）
- ログには語の内容もヒット詳細も出さない（ヒット時は中止の事実のみ）
- ローカルの語リストを更新したら secret も更新する
  （`gh secret set BOUNDARY_WORDS -R syotakokichi/beadmap < ~/.config/beadmap/boundary-words.txt`）

## 運用

- 全 push 前に `/boundary-check`（`.claude/commands/boundary-check.md`）を実行する
- 5 項目のチェックリストを devlog に記録する（語リストの内容は記録しない）
- 実装中に公開してはならない語に気づいたら、非公開側のリストに追記してから再実行する
- 新しいブランチ・デモ用 PR も公開物であり、同じゲートの対象とする
