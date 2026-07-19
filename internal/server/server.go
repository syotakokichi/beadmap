// Package server は collector のスナップショットを 127.0.0.1 でのみ配信する。
// read-only 契約（GET 以外 405・書き込み API を持たない・取得失敗時は前回
// スナップショットを stale と明示）はこのパッケージの契約テストで固定される。
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/syotakokichi/beadmap/internal/collector"
)

// Server は beads スナップショットの読み取り専用ビューを配信する。
type Server struct {
	dir    string
	ui     fs.FS
	loadFn func(context.Context, string) (*collector.Snapshot, error)

	mu       sync.Mutex
	lastGood *collector.Snapshot
}

// New は dir（.beads/ を含むリポジトリ）を対象とするサーバを作る。
// ui には配信する静的 UI（index.html 等）のファイルシステムを渡す。
func New(dir string, ui fs.FS) *Server {
	return &Server{dir: dir, ui: ui, loadFn: collector.Load}
}

// ListenLocal は 127.0.0.1 にのみバインドする。優先ポートが使用中なら
// エフェメラルポートに自動で回避する。0.0.0.0 等へのバインド手段は提供しない
// （read-only 契約: ローカル専用）。
func ListenLocal(preferredPort int) (net.Listener, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", preferredPort))
	if err == nil {
		return ln, nil
	}
	return net.Listen("tcp", "127.0.0.1:0")
}

// Handler は全エンドポイントを GET 限定ミドルウェアで包んで返す。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/snapshot", s.handleSnapshot)
	mux.HandleFunc("/api/closed", s.handleClosed)
	mux.Handle("/", http.FileServerFS(s.ui))
	return getOnly(mux)
}

// getOnly は GET 以外のメソッドをすべて 405 で拒否する（read-only 契約）。
func getOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "beadmap は読み取り専用です（GET のみ）", http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// response は API 応答の共通封筒。stale はデータの鮮度が保証できない状態
// （最新の読み込みに失敗し前回スナップショットを返している）を表す。
type response struct {
	Issues      []collector.Issue `json:"issues"`
	Ready       []string          `json:"ready"`
	Blocked     []string          `json:"blocked"`
	ReadySource string            `json:"ready_source"`
	GeneratedAt time.Time         `json:"generated_at"`
	FileModTime time.Time         `json:"file_mod_time"`
	SourcePath  string            `json:"source_path"`
	Warnings    []string          `json:"warnings,omitempty"`
	Stale       bool              `json:"stale"`
	StaleReason string            `json:"stale_reason,omitempty"`
	Counts      map[string]int    `json:"counts"`
}

// load は最新スナップショットを読み、失敗時は前回成功分を stale として返す。
func (s *Server) load(ctx context.Context) (snap *collector.Snapshot, stale bool, reason string, err error) {
	snap, err = s.loadFn(ctx, s.dir)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil {
		s.lastGood = snap
		return snap, false, "", nil
	}
	if s.lastGood != nil {
		return s.lastGood, true, "最新データの取得に失敗したため前回のスナップショットを表示中: " + err.Error(), nil
	}
	return nil, false, "", err
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	snap, stale, reason, err := s.load(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	resp := response{
		Ready:       snap.Ready,
		Blocked:     snap.Blocked,
		ReadySource: snap.ReadySource,
		GeneratedAt: snap.GeneratedAt,
		FileModTime: snap.FileModTime,
		SourcePath:  snap.SourcePath,
		Warnings:    snap.Warnings,
		Stale:       stale,
		StaleReason: reason,
		Counts:      countByStatus(snap.Issues),
	}
	// closed は件数が多いため既定では返さない（/api/closed で opt-in 取得）
	resp.Issues = make([]collector.Issue, 0, len(snap.Issues))
	for _, is := range snap.Issues {
		if is.Status != "closed" {
			resp.Issues = append(resp.Issues, is)
		}
	}
	writeJSON(w, resp)
}

func (s *Server) handleClosed(w http.ResponseWriter, r *http.Request) {
	snap, stale, reason, err := s.load(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	resp := response{
		GeneratedAt: snap.GeneratedAt,
		FileModTime: snap.FileModTime,
		SourcePath:  snap.SourcePath,
		Stale:       stale,
		StaleReason: reason,
		Counts:      countByStatus(snap.Issues),
	}
	resp.Issues = make([]collector.Issue, 0)
	for _, is := range snap.Issues {
		if is.Status == "closed" {
			resp.Issues = append(resp.Issues, is)
		}
	}
	writeJSON(w, resp)
}

func countByStatus(issues []collector.Issue) map[string]int {
	counts := map[string]int{"total": len(issues)}
	for _, is := range issues {
		counts[is.Status]++
	}
	return counts
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeJSONError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
