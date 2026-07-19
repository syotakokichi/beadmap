# read-only 契約ルール

beadmap は「正本の読み取り用地図」。正本（beads の Dolt DB / `.beads/issues.jsonl`）を
壊さない保証を、運用の注意ではなく **構造** で持つ。契約の一次ソースは
[README の契約表](../../README.md) で、本ルールはエージェント向けの実装規約。

## 契約 5 項目と対応テスト

| # | 契約 | 実装 | テスト |
| --- | --- | --- | --- |
| 1 | `127.0.0.1` にのみバインドする | `server.ListenLocal`（それ以外のバインド手段を持たない） | `TestListenLocalBindsLoopbackOnly` / `TestListenLocalAvoidsBusyPort` |
| 2 | GET 以外の HTTP メソッドは 405 | `server.getOnly` ミドルウェア | `TestWriteMethodsRejected` |
| 3 | bd の実行は読み取り専用の固定引数のみ | `collector.bdReadyArgs` / `bdBlockedArgs`（動的組み立てなし） | `TestBDArgsAreFixed` |
| 4 | 外部サービスへの自動アクセスなし | 依存ゼロ・HTTP クライアント不使用（ブラウザ起動もローカル URL のみ） | 構造で担保（依存が増えないことは go.mod で確認） |
| 5 | 取得失敗時は前回スナップショットを stale と明示 | `server.load` の lastGood 保持 | `TestStaleServesLastGoodSnapshot` |

## 変更時のルール

- 契約テストを弱める・削る変更は **契約変更**。README の契約表と同時に更新し、
  人間の承認を得てからマージする
- 新しいエンドポイントは GET のみ。起票・更新・close 等の書き込み API は追加しない
  （v1.1 の拡張候補にも含めない）。書き込みは bd CLI の役割
- `-dir` で指定されたリポジトリ内のファイルを開くのは読み取りのみ。
  ファイル作成・変更・削除を行うコードを書かない
