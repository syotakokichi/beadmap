/* 静的デモの設定。Pages のデプロイ時のみ index.html に読み込まれる
   （ローカル起動のライブ UI には同梱されない）。 */
"use strict";

window.BEADMAP_DEMO = {
  initial: "settlebase",
  datasets: [
    {
      id: "settlebase",
      label: "settlebase（連載の公開デモ開発）",
      snapshot: "data/settlebase-snapshot.json",
      closed: "data/settlebase-closed.json",
    },
    {
      id: "beadmap",
      label: "beadmap（このビューア自身）",
      snapshot: "data/beadmap-snapshot.json",
      closed: "data/beadmap-closed.json",
    },
  ],
};
