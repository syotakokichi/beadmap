package collector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Snapshot は collector の出力。UI が必要とする情報一式に、データの
// 鮮度（staleness 判断材料）を必ず添える。jsonl は正本（Dolt DB）の
// export であり正本そのものではないため、取得時刻とファイル更新時刻を
// UI に明示する。
type Snapshot struct {
	Issues      []Issue   `json:"issues"`
	Ready       []string  `json:"ready"`
	Blocked     []string  `json:"blocked"`
	ReadySource string    `json:"ready_source"` // "bd"（ライブクエリ・正） | "fallback"（自前計算・近似）
	GeneratedAt time.Time `json:"generated_at"` // collector がデータを読んだ時刻
	FileModTime time.Time `json:"file_mod_time"` // issues.jsonl の最終更新時刻
	SourcePath  string    `json:"source_path"`
	Warnings    []string  `json:"warnings,omitempty"`
}

// Load は dir（.beads/ を含むリポジトリ）からスナップショットを構築する。
// bd CLI が検出できれば ready / blocked はライブクエリで取得し、
// 失敗・未検出時は fallback 計算に切り替えて Warnings に残す。
func Load(ctx context.Context, dir string) (*Snapshot, error) {
	path := filepath.Join(dir, ".beads", "issues.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("issues.jsonl を開けません: %w", err)
	}
	defer f.Close()

	issues, err := ParseJSONL(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	snap := &Snapshot{
		Issues:      issues,
		GeneratedAt: time.Now(),
		SourcePath:  path,
	}
	if fi, err := os.Stat(path); err == nil {
		snap.FileModTime = fi.ModTime()
	}

	if bdPath := DetectBD(); bdPath != "" {
		ready, blocked, err := ReadyBlockedFromBD(ctx, bdPath, dir)
		if err == nil {
			snap.Ready, snap.Blocked, snap.ReadySource = ready, blocked, "bd"
			return snap, nil
		}
		snap.Warnings = append(snap.Warnings,
			"bd ライブクエリに失敗したため fallback 計算を使用: "+err.Error())
	}
	snap.Ready, snap.Blocked = ReadyBlockedFallback(issues)
	snap.ReadySource = "fallback"
	return snap, nil
}
