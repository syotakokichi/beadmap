package collector

import (
	"strings"
	"testing"
	"time"
)

const fixtureBasic = `{"_type":"issue","id":"bm-1","title":"親エピック","status":"open","priority":1,"issue_type":"epic","created_at":"2026-07-01T00:00:00Z","updated_at":"2026-07-10T00:00:00Z"}
{"_type":"issue","id":"bm-2","title":"子タスク","status":"in_progress","priority":2,"issue_type":"task","labels":["area:demo"],"assignee":"someone","updated_at":"2026-07-05T00:00:00Z","dependencies":[{"issue_id":"bm-2","depends_on_id":"bm-1","type":"parent-child","created_at":"2026-07-01T00:00:00Z"}]}
{"_type":"issue","id":"bm-3","title":"ブロックされたタスク","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"bm-3","depends_on_id":"bm-2","type":"blocks"}]}
{"_type":"issue","id":"bm-4","title":"完了済み","status":"closed","priority":3,"issue_type":"task","closed_at":"2026-07-15T12:00:00Z","close_reason":"done"}`

func parseFixture(t *testing.T, s string) []Issue {
	t.Helper()
	issues, err := ParseJSONL(strings.NewReader(s))
	if err != nil {
		t.Fatalf("ParseJSONL: %v", err)
	}
	return issues
}

func findIssue(t *testing.T, issues []Issue, id string) Issue {
	t.Helper()
	for _, is := range issues {
		if is.ID == id {
			return is
		}
	}
	t.Fatalf("issue %s が見つかりません", id)
	return Issue{}
}

func TestParseJSONLBasic(t *testing.T) {
	issues := parseFixture(t, fixtureBasic)
	if len(issues) != 4 {
		t.Fatalf("件数 = %d, want 4", len(issues))
	}
	// ID 昇順で返る
	for i := 1; i < len(issues); i++ {
		if issues[i-1].ID >= issues[i].ID {
			t.Errorf("ID 昇順でない: %s >= %s", issues[i-1].ID, issues[i].ID)
		}
	}
	epic := findIssue(t, issues, "bm-1")
	if epic.IssueType != "epic" || epic.Priority != 1 {
		t.Errorf("bm-1 = %+v", epic)
	}
	want := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	if !epic.UpdatedAt.Equal(want) {
		t.Errorf("bm-1 UpdatedAt = %v, want %v", epic.UpdatedAt, want)
	}
}

func TestParseJSONLDerivesRelations(t *testing.T) {
	issues := parseFixture(t, fixtureBasic)
	child := findIssue(t, issues, "bm-2")
	if child.ParentID != "bm-1" {
		t.Errorf("bm-2 ParentID = %q, want bm-1", child.ParentID)
	}
	epic := findIssue(t, issues, "bm-1")
	if len(epic.Children) != 1 || epic.Children[0] != "bm-2" {
		t.Errorf("bm-1 Children = %v, want [bm-2]", epic.Children)
	}
	blocked := findIssue(t, issues, "bm-3")
	if len(blocked.BlockedBy) != 1 || blocked.BlockedBy[0] != "bm-2" {
		t.Errorf("bm-3 BlockedBy = %v, want [bm-2]", blocked.BlockedBy)
	}
	blocker := findIssue(t, issues, "bm-2")
	if len(blocker.Dependents) != 1 || blocker.Dependents[0] != "bm-3" {
		t.Errorf("bm-2 Dependents = %v, want [bm-3]", blocker.Dependents)
	}
}

func TestParseJSONLIgnoresUnknownFieldsAndDepTypes(t *testing.T) {
	src := `{"_type":"issue","id":"bm-1","title":"a","status":"open","brand_new_field":123,"nested":{"x":1},"dependencies":[{"issue_id":"bm-1","depends_on_id":"bm-9","type":"related"},{"issue_id":"bm-1","depends_on_id":"bm-9","type":"discovered-from"}]}`
	issues := parseFixture(t, src)
	if len(issues) != 1 {
		t.Fatalf("件数 = %d, want 1", len(issues))
	}
	is := issues[0]
	if is.ParentID != "" || len(is.BlockedBy) != 0 {
		t.Errorf("未知の依存 type が反映されている: %+v", is)
	}
}

func TestParseJSONLSkipsEmptyLinesAndNonIssueRecords(t *testing.T) {
	src := "\n" + `{"_type":"comment","id":"c-1","body":"not an issue"}` + "\n\n" +
		`{"_type":"issue","id":"bm-1","title":"a","status":"open"}` + "\n"
	issues := parseFixture(t, src)
	if len(issues) != 1 || issues[0].ID != "bm-1" {
		t.Fatalf("issues = %+v, want [bm-1]", issues)
	}
}

func TestParseJSONLMissingRequiredField(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"id なし", `{"_type":"issue","title":"a","status":"open"}`},
		{"title なし", `{"_type":"issue","id":"bm-1","status":"open"}`},
		{"status なし", `{"_type":"issue","id":"bm-1","title":"a"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseJSONL(strings.NewReader(tc.src))
			if err == nil {
				t.Fatal("エラーになるべき入力が成功した")
			}
			if !strings.Contains(err.Error(), "必須フィールド") {
				t.Errorf("エラー文言 = %v", err)
			}
		})
	}
}

func TestParseJSONLInvalidJSONReportsLineNumber(t *testing.T) {
	src := `{"_type":"issue","id":"bm-1","title":"a","status":"open"}` + "\n" + `{broken`
	_, err := ParseJSONL(strings.NewReader(src))
	if err == nil {
		t.Fatal("エラーになるべき入力が成功した")
	}
	if !strings.Contains(err.Error(), "2 行目") {
		t.Errorf("行番号がエラーに含まれない: %v", err)
	}
}

func TestParseJSONLLongLine(t *testing.T) {
	// bufio.Scanner の既定バッファ 64KB を超える行（長い design を持つ bead 相当）
	long := strings.Repeat("あ", 100*1024)
	src := `{"_type":"issue","id":"bm-1","title":"a","status":"open","design":"` + long + `"}`
	issues := parseFixture(t, src)
	if len(issues) != 1 {
		t.Fatalf("件数 = %d, want 1", len(issues))
	}
	if len(issues[0].Design) == 0 {
		t.Error("長い design が読めていない")
	}
}

func TestParseJSONLInvalidTimestampIsZero(t *testing.T) {
	src := `{"_type":"issue","id":"bm-1","title":"a","status":"open","updated_at":"not-a-time"}`
	issues := parseFixture(t, src)
	if !issues[0].UpdatedAt.IsZero() {
		t.Errorf("不正なタイムスタンプがゼロ値になっていない: %v", issues[0].UpdatedAt)
	}
}
