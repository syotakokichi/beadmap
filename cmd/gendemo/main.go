// gendemo は静的デモページ用のスナップショット JSON を生成する開発ツール。
// サーバの /api/snapshot・/api/closed と同じ形の JSON をファイルに書き出す
// （デモはブラウザ内描画のみで、このツールも正本には一切書き込まない）。
//
// 使い方:
//
//	go run ./cmd/gendemo -dir . -name beadmap
//	go run ./cmd/gendemo -dir ../settlebase -name settlebase
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/syotakokichi/beadmap/internal/collector"
)

// envelope は internal/server の API 応答と同じ封筒。UI が読むフィールドを揃える。
type envelope struct {
	Issues      []collector.Issue `json:"issues"`
	Ready       []string          `json:"ready"`
	Blocked     []string          `json:"blocked"`
	ReadySource string            `json:"ready_source"`
	GeneratedAt time.Time         `json:"generated_at"`
	FileModTime time.Time         `json:"file_mod_time"`
	SourcePath  string            `json:"source_path"`
	Warnings    []string          `json:"warnings,omitempty"`
	Stale       bool              `json:"stale"`
	Counts      map[string]int    `json:"counts"`
}

func main() {
	dir := flag.String("dir", ".", ".beads/ を含むリポジトリのパス")
	name := flag.String("name", "", "データセット名（出力ファイル名の接頭辞）")
	out := flag.String("out", "demo/data", "出力ディレクトリ")
	flag.Parse()
	if *name == "" {
		fmt.Fprintln(os.Stderr, "-name は必須です（例: -name settlebase）")
		os.Exit(2)
	}

	snap, err := collector.Load(context.Background(), *dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// ローカルの絶対パスを公開 JSON に含めない（データセット名で表す）
	snap.SourcePath = *name + "/.beads/issues.jsonl（スナップショット）"

	counts := map[string]int{"total": len(snap.Issues)}
	var open, closed []collector.Issue
	for _, is := range snap.Issues {
		counts[is.Status]++
		if is.Status == "closed" {
			closed = append(closed, is)
		} else {
			open = append(open, is)
		}
	}

	write := func(suffix string, issues []collector.Issue, full bool) {
		env := envelope{
			Issues:      issues,
			GeneratedAt: snap.GeneratedAt,
			FileModTime: snap.FileModTime,
			SourcePath:  snap.SourcePath,
			Counts:      counts,
		}
		if full {
			env.Ready = snap.Ready
			env.Blocked = snap.Blocked
			env.ReadySource = snap.ReadySource
			env.Warnings = snap.Warnings
		}
		if env.Issues == nil {
			env.Issues = []collector.Issue{}
		}
		path := filepath.Join(*out, *name+"-"+suffix+".json")
		f, err := os.Create(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		enc := json.NewEncoder(f)
		enc.SetEscapeHTML(false)
		// 行単位の diff 判定（自動更新でタイムスタンプ行だけの変化を無視する）のため整形出力
		enc.SetIndent("", "  ")
		if err := enc.Encode(env); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		f.Close()
		fmt.Println("wrote", path)
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	write("snapshot", open, true)
	write("closed", closed, false)
}
