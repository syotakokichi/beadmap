package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// read-only 契約 4「外部サービスへの自動アクセスなし」を構造で固定するテスト。
// 依存追加・HTTP クライアント・外向きダイヤルの混入は契約変更であり、
// README の契約表と同時に更新して人間の承認を得る（.claude/rules/read-only-contract.md）。

func TestGoModHasNoDependencies(t *testing.T) {
	b, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("go.mod を読めません: %v", err)
	}
	if strings.Contains(string(b), "require") {
		t.Fatalf("go.mod に依存が追加されています（依存ゼロ契約）:\n%s", b)
	}
}

// 外向き通信に使われる代表 API。net.Listen（loopback バインド）は含めない。
var outboundPattern = regexp.MustCompile(`http\.(Get|Post|PostForm|Head|NewRequest|Client)\b|net\.Dial`)

func TestNoOutboundNetworkCalls(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".beads", "ui", "docs":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if m := outboundPattern.Find(src); m != nil {
			t.Errorf("%s: 外向き通信 API の使用を検出: %s（read-only 契約 4）", path, m)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("走査に失敗: %v", err)
	}
}
