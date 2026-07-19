package collector

import (
	"slices"
	"strings"
	"testing"
)

func TestReadyBlockedFallback(t *testing.T) {
	src := strings.Join([]string{
		// open・依存なし → ready
		`{"_type":"issue","id":"bm-1","title":"a","status":"open"}`,
		// open・open な blocker あり → blocked
		`{"_type":"issue","id":"bm-2","title":"b","status":"open","dependencies":[{"issue_id":"bm-2","depends_on_id":"bm-1","type":"blocks"}]}`,
		// open・closed な blocker のみ → ready
		`{"_type":"issue","id":"bm-3","title":"c","status":"open","dependencies":[{"issue_id":"bm-3","depends_on_id":"bm-9","type":"blocks"}]}`,
		`{"_type":"issue","id":"bm-9","title":"done","status":"closed"}`,
		// in_progress → ready にも blocked にも入らない
		`{"_type":"issue","id":"bm-4","title":"d","status":"in_progress"}`,
		// deferred → 同上
		`{"_type":"issue","id":"bm-5","title":"e","status":"deferred"}`,
		// open・jsonl に存在しない blocker → 無視して ready
		`{"_type":"issue","id":"bm-6","title":"f","status":"open","dependencies":[{"issue_id":"bm-6","depends_on_id":"ghost-1","type":"blocks"}]}`,
		// open な epic（親）→ ready に含まれる（bd の実挙動と同じ）
		`{"_type":"issue","id":"bm-7","title":"g","status":"open","issue_type":"epic"}`,
		// open・in_progress な blocker → blocked（closed 以外は未解決扱い）
		`{"_type":"issue","id":"bm-8","title":"h","status":"open","dependencies":[{"issue_id":"bm-8","depends_on_id":"bm-4","type":"blocks"}]}`,
	}, "\n")

	issues := parseFixture(t, src)
	ready, blocked := ReadyBlockedFallback(issues)

	wantReady := []string{"bm-1", "bm-3", "bm-6", "bm-7"}
	wantBlocked := []string{"bm-2", "bm-8"}
	if !slices.Equal(ready, wantReady) {
		t.Errorf("ready = %v, want %v", ready, wantReady)
	}
	if !slices.Equal(blocked, wantBlocked) {
		t.Errorf("blocked = %v, want %v", blocked, wantBlocked)
	}
}
