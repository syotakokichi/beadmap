package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/syotakokichi/beadmap/internal/collector"
)

var testUI = fstest.MapFS{
	"index.html": &fstest.MapFile{Data: []byte("<!doctype html><title>beadmap</title>")},
}

func fixtureSnapshot() *collector.Snapshot {
	return &collector.Snapshot{
		Issues: []collector.Issue{
			{ID: "bm-1", Title: "開いている", Status: "open", Priority: 1},
			{ID: "bm-2", Title: "進行中", Status: "in_progress", Priority: 2},
			{ID: "bm-3", Title: "完了", Status: "closed", Priority: 2},
		},
		Ready:       []string{"bm-1"},
		Blocked:     []string{},
		ReadySource: "fallback",
		GeneratedAt: time.Now(),
		SourcePath:  "/tmp/fixture/.beads/issues.jsonl",
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	s := New("/tmp/fixture", testUI)
	s.loadFn = func(context.Context, string) (*collector.Snapshot, error) {
		return fixtureSnapshot(), nil
	}
	return s
}

// read-only 契約: GET 以外のメソッドはパスに関わらずすべて 405。
func TestWriteMethodsRejected(t *testing.T) {
	s := newTestServer(t)
	h := s.Handler()
	methods := []string{
		http.MethodPost, http.MethodPut, http.MethodDelete,
		http.MethodPatch, http.MethodOptions, http.MethodHead,
	}
	paths := []string{"/", "/api/snapshot", "/api/closed", "/anything"}
	for _, m := range methods {
		for _, p := range paths {
			req := httptest.NewRequest(m, p, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s = %d, want 405", m, p, rec.Code)
			}
			if allow := rec.Header().Get("Allow"); allow != http.MethodGet {
				t.Errorf("%s %s の Allow ヘッダ = %q, want GET", m, p, allow)
			}
		}
	}
}

// read-only 契約: バインド先は 127.0.0.1 のみ。優先ポート使用中は自動回避する。
func TestListenLocalBindsLoopbackOnly(t *testing.T) {
	ln, err := ListenLocal(0)
	if err != nil {
		t.Fatalf("ListenLocal: %v", err)
	}
	defer ln.Close()
	addr := ln.Addr().String()
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Errorf("バインド先 = %s, want 127.0.0.1:*", addr)
	}
}

// ポート自動回避: 使用中の優先ポートを渡しても 127.0.0.1 の別ポートで listen できる。
func TestListenLocalAvoidsBusyPort(t *testing.T) {
	first, err := ListenLocal(0)
	if err != nil {
		t.Fatalf("ListenLocal: %v", err)
	}
	defer first.Close()
	busyPort := first.Addr().(*net.TCPAddr).Port

	second, err := ListenLocal(busyPort)
	if err != nil {
		t.Fatalf("使用中ポートからの自動回避に失敗: %v", err)
	}
	defer second.Close()
	addr := second.Addr().String()
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Errorf("回避後のバインド先 = %s, want 127.0.0.1:*", addr)
	}
	if second.Addr().(*net.TCPAddr).Port == busyPort {
		t.Error("使用中ポートと同じポートで listen している")
	}
}

func decodeResponse(t *testing.T, rec *httptest.ResponseRecorder) response {
	t.Helper()
	var resp response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("JSON decode: %v", err)
	}
	return resp
}

// /api/snapshot は closed を含まない（closed は /api/closed で opt-in 取得）。
func TestSnapshotExcludesClosed(t *testing.T) {
	s := newTestServer(t)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/snapshot", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	resp := decodeResponse(t, rec)
	if len(resp.Issues) != 2 {
		t.Errorf("issues 件数 = %d, want 2（closed 除外）", len(resp.Issues))
	}
	for _, is := range resp.Issues {
		if is.Status == "closed" {
			t.Errorf("closed な issue %s が snapshot に含まれている", is.ID)
		}
	}
	// サマリータイル用の counts は closed も含む全件を数える
	if resp.Counts["closed"] != 1 || resp.Counts["total"] != 3 {
		t.Errorf("counts = %v", resp.Counts)
	}
	if resp.Stale {
		t.Error("正常時に stale が立っている")
	}
}

func TestClosedEndpointReturnsOnlyClosed(t *testing.T) {
	s := newTestServer(t)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/closed", nil))
	resp := decodeResponse(t, rec)
	if len(resp.Issues) != 1 || resp.Issues[0].ID != "bm-3" {
		t.Errorf("closed = %+v, want [bm-3]", resp.Issues)
	}
}

// read-only 契約: 取得失敗時は前回スナップショットを stale と明示して返す。
func TestStaleServesLastGoodSnapshot(t *testing.T) {
	s := New("/tmp/fixture", testUI)
	calls := 0
	s.loadFn = func(context.Context, string) (*collector.Snapshot, error) {
		calls++
		if calls == 1 {
			return fixtureSnapshot(), nil
		}
		return nil, errors.New("jsonl が読めない")
	}
	h := s.Handler()

	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/api/snapshot", nil))
	if resp := decodeResponse(t, rec1); resp.Stale {
		t.Fatal("1回目から stale になっている")
	}

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/snapshot", nil))
	if rec2.Code != http.StatusOK {
		t.Fatalf("stale 時の status = %d, want 200", rec2.Code)
	}
	resp := decodeResponse(t, rec2)
	if !resp.Stale || resp.StaleReason == "" {
		t.Errorf("stale フラグ/理由が返っていない: stale=%v reason=%q", resp.Stale, resp.StaleReason)
	}
	if len(resp.Issues) != 2 {
		t.Errorf("stale 時に前回スナップショットが返っていない: %d 件", len(resp.Issues))
	}
}

// 一度も成功していない状態での取得失敗はエラー応答（500）を返す。
func TestLoadErrorWithoutLastGood(t *testing.T) {
	s := New("/tmp/fixture", testUI)
	s.loadFn = func(context.Context, string) (*collector.Snapshot, error) {
		return nil, errors.New("jsonl が読めない")
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/snapshot", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestServesStaticUI(t *testing.T) {
	s := newTestServer(t)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "beadmap") {
		t.Error("index.html が配信されていない")
	}
}
