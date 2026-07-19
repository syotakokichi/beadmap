// beadmap は beads（bd CLI）のタスクデータを読むだけのローカル進捗ビューア。
// 正本（beads の Dolt DB）にも viewer 向け export（.beads/issues.jsonl）にも一切書き込まない。
// read-only 契約の全文は README を参照。
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/syotakokichi/beadmap/internal/server"
)

//go:embed ui
var uiFiles embed.FS

// defaultPort 23237 は電話キーパッドで "beads"（2=b, 3=e, 2=a, 3=d, 7=s）。
// 使用中なら自動でエフェメラルポートに回避する。
const defaultPort = 23237

func main() {
	dir := flag.String("dir", ".", ".beads/ を含むリポジトリのパス")
	port := flag.Int("port", defaultPort, "優先ポート（使用中なら自動で別ポートに回避）")
	noOpen := flag.Bool("no-open", false, "ブラウザを自動で開かない")
	flag.Parse()

	abs, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatalf("beadmap: パスを解決できません: %v", err)
	}
	jsonl := filepath.Join(abs, ".beads", "issues.jsonl")
	if _, err := os.Stat(jsonl); err != nil {
		log.Fatalf("beadmap: %s が見つかりません。-dir に .beads/ を含むリポジトリを指定してください", jsonl)
	}

	ui, err := fs.Sub(uiFiles, "ui")
	if err != nil {
		log.Fatalf("beadmap: %v", err)
	}

	ln, err := server.ListenLocal(*port)
	if err != nil {
		log.Fatalf("beadmap: listen に失敗: %v", err)
	}
	url := fmt.Sprintf("http://%s/", ln.Addr())
	fmt.Printf("beadmap: %s を表示中 → %s\n", jsonl, url)
	fmt.Println("beadmap: 読み取り専用です。終了は Ctrl+C。")

	if !*noOpen {
		openBrowser(url)
	}
	log.Fatal(http.Serve(ln, server.New(abs, ui).Handler()))
}

// openBrowser は既定ブラウザで url を開く。開けなくても致命ではない
// （URL は標準出力に表示済み）。
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "beadmap: ブラウザを開けませんでした（%v）。上記 URL を手動で開いてください\n", err)
	}
}
