package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// bd への問い合わせは読み取り専用の固定引数のみ。任意コマンドの組み立ては
// 行わない（read-only 契約）。この固定リストはテストで固定されている。
var (
	bdReadyArgs   = []string{"ready", "--json"}
	bdBlockedArgs = []string{"blocked", "--json"}
)

const bdTimeout = 15 * time.Second

// DetectBD は PATH 上の bd CLI を探し、見つからなければ空文字を返す。
func DetectBD() string {
	p, err := exec.LookPath("bd")
	if err != nil {
		return ""
	}
	return p
}

// ReadyBlockedFromBD は bd のライブクエリで ready / blocked の issue ID を取得する。
// bd 導入時はこちらが正であり、fallback 計算（fallback.go）より優先する。
func ReadyBlockedFromBD(ctx context.Context, bdPath, dir string) (ready, blocked []string, err error) {
	ready, err = bdIDQuery(ctx, bdPath, dir, bdReadyArgs)
	if err != nil {
		return nil, nil, err
	}
	blocked, err = bdIDQuery(ctx, bdPath, dir, bdBlockedArgs)
	if err != nil {
		return nil, nil, err
	}
	return ready, blocked, nil
}

func bdIDQuery(ctx context.Context, bdPath, dir string, args []string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, bdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bdPath, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return nil, fmt.Errorf("bd %s: %w (%s)", strings.Join(args, " "), err, msg)
	}
	var rows []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		return nil, fmt.Errorf("bd %s の出力を解釈できません: %w", strings.Join(args, " "), err)
	}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.ID != "" {
			ids = append(ids, r.ID)
		}
	}
	return ids, nil
}
