package collector

import (
	"context"
	"os"
	"slices"
	"testing"
)

// TestBDArgsAreFixed は「bd 実行は読み取り専用の固定引数のみ」という
// read-only 契約を固定する。このテストを変える変更は契約の変更であり、
// README の契約表も同時に更新すること。
func TestBDArgsAreFixed(t *testing.T) {
	if !slices.Equal(bdReadyArgs, []string{"ready", "--json"}) {
		t.Errorf("bdReadyArgs = %v", bdReadyArgs)
	}
	if !slices.Equal(bdBlockedArgs, []string{"blocked", "--json"}) {
		t.Errorf("bdBlockedArgs = %v", bdBlockedArgs)
	}
}

// TestFallbackMatchesBD は fallback 計算と bd ライブクエリの結果を実データで
// 突き合わせる integration テスト。bd と実リポジトリが必要なため、
// 環境変数 BEADMAP_BD_TEST_DIR（.beads/ を含むリポジトリのパス）を指定した
// 時のみ実行される。CI では常にスキップされる。
func TestFallbackMatchesBD(t *testing.T) {
	dir := os.Getenv("BEADMAP_BD_TEST_DIR")
	if dir == "" {
		t.Skip("BEADMAP_BD_TEST_DIR が未設定のためスキップ")
	}
	bdPath := DetectBD()
	if bdPath == "" {
		t.Skip("bd が PATH に見つからないためスキップ")
	}

	snap, err := Load(context.Background(), dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	fbReady, fbBlocked := ReadyBlockedFallback(snap.Issues)
	bdReady, bdBlocked, err := ReadyBlockedFromBD(context.Background(), bdPath, dir)
	if err != nil {
		t.Fatalf("bd クエリ: %v", err)
	}
	slices.Sort(bdReady)
	slices.Sort(bdBlocked)

	if !slices.Equal(fbReady, bdReady) {
		t.Errorf("ready が bd と不一致:\n  fallback: %v\n  bd:       %v", fbReady, bdReady)
	}
	if !slices.Equal(fbBlocked, bdBlocked) {
		t.Errorf("blocked が bd と不一致:\n  fallback: %v\n  bd:       %v", fbBlocked, bdBlocked)
	}
}
