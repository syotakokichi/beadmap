// Package collector は beads のデータ（.beads/issues.jsonl と bd CLI の
// 読み取り専用クエリ）を読み、正規化したスナップショットを返す。
// 正本（beads の Dolt DB）への書き込みは一切行わない。read-only 契約の全文は
// README を参照。
package collector

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"
)

// Issue は issues.jsonl の 1 レコードを UI 表示用に正規化したもの。
type Issue struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Status      string    `json:"status"`
	Priority    int       `json:"priority"`
	IssueType   string    `json:"issue_type"`
	Labels      []string  `json:"labels,omitempty"`
	Assignee    string    `json:"assignee,omitempty"`
	Description string    `json:"description,omitempty"`
	Design      string    `json:"design,omitempty"`
	Acceptance  string    `json:"acceptance_criteria,omitempty"`
	Notes       string    `json:"notes,omitempty"`
	CloseReason string    `json:"close_reason,omitempty"`
	ExternalRef string    `json:"external_ref,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitzero"`
	UpdatedAt   time.Time `json:"updated_at,omitzero"`
	ClosedAt    time.Time `json:"closed_at,omitzero"`

	// 依存関係（jsonl の dependencies から導出、詳細ペインは一段先のみ表示）
	ParentID   string   `json:"parent_id,omitempty"`   // parent-child の親
	Children   []string `json:"children,omitempty"`    // parent-child の子（導出）
	BlockedBy  []string `json:"blocked_by,omitempty"`  // blocks の依存先（これらが閉じるまで待ち）
	Dependents []string `json:"dependents,omitempty"`  // この issue を待っている issue（導出）
}

// rawIssue は jsonl のスキーマに合わせた入力用の型。
// 未知フィールドは encoding/json の既定動作で無視される（parser 耐性方針）。
type rawIssue struct {
	Type         string   `json:"_type"`
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Status       string   `json:"status"`
	Priority     int      `json:"priority"`
	IssueType    string   `json:"issue_type"`
	Labels       []string `json:"labels"`
	Assignee     string   `json:"assignee"`
	Description  string   `json:"description"`
	Design       string   `json:"design"`
	Acceptance   string   `json:"acceptance_criteria"`
	Notes        string   `json:"notes"`
	CloseReason  string   `json:"close_reason"`
	ExternalRef  string   `json:"external_ref"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
	ClosedAt     string   `json:"closed_at"`
	Dependencies []rawDep `json:"dependencies"`
}

type rawDep struct {
	IssueID     string `json:"issue_id"`
	DependsOnID string `json:"depends_on_id"`
	Type        string `json:"type"`
}

// maxLineBytes は 1 レコードの上限。design や notes が長い bead があるため
// bufio.Scanner の既定 64KB では足りない。
const maxLineBytes = 16 * 1024 * 1024

// ParseJSONL は issues.jsonl を読み、正規化した Issue 一覧を ID 昇順で返す。
// 未知フィールド・未知の依存 type は無視する。必須フィールド
// （id / title / status）が欠けた行は行番号付きのエラーを返す。
func ParseJSONL(r io.Reader) ([]Issue, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	var issues []Issue
	deps := make(map[string][]rawDep) // issue ID → 依存エントリ
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var raw rawIssue
		if err := json.Unmarshal(line, &raw); err != nil {
			return nil, fmt.Errorf("%d 行目: JSON を解釈できません: %w", lineNo, err)
		}
		if raw.Type != "" && raw.Type != "issue" {
			continue // issue 以外のレコードは対象外
		}
		if raw.ID == "" || raw.Title == "" || raw.Status == "" {
			return nil, fmt.Errorf("%d 行目: 必須フィールド（id / title / status）が欠けています", lineNo)
		}
		issue := Issue{
			ID:          raw.ID,
			Title:       raw.Title,
			Status:      raw.Status,
			Priority:    raw.Priority,
			IssueType:   raw.IssueType,
			Labels:      raw.Labels,
			Assignee:    raw.Assignee,
			Description: raw.Description,
			Design:      raw.Design,
			Acceptance:  raw.Acceptance,
			Notes:       raw.Notes,
			CloseReason: raw.CloseReason,
			ExternalRef: raw.ExternalRef,
			CreatedAt:   parseTime(raw.CreatedAt),
			UpdatedAt:   parseTime(raw.UpdatedAt),
			ClosedAt:    parseTime(raw.ClosedAt),
		}
		issues = append(issues, issue)
		if len(raw.Dependencies) > 0 {
			deps[raw.ID] = raw.Dependencies
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("issues.jsonl の読み込みに失敗: %w", err)
	}

	linkDependencies(issues, deps)
	sort.Slice(issues, func(i, j int) bool { return issues[i].ID < issues[j].ID })
	return issues, nil
}

// linkDependencies は dependencies から親子・ブロック関係を双方向に導出する。
// parent-child / blocks 以外の type（related 等）は v1 では扱わない。
func linkDependencies(issues []Issue, deps map[string][]rawDep) {
	byID := make(map[string]*Issue, len(issues))
	for i := range issues {
		byID[issues[i].ID] = &issues[i]
	}
	for id, ds := range deps {
		issue := byID[id]
		if issue == nil {
			continue
		}
		for _, d := range ds {
			target := byID[d.DependsOnID]
			switch d.Type {
			case "parent-child":
				issue.ParentID = d.DependsOnID
				if target != nil {
					target.Children = append(target.Children, id)
				}
			case "blocks":
				issue.BlockedBy = append(issue.BlockedBy, d.DependsOnID)
				if target != nil {
					target.Dependents = append(target.Dependents, id)
				}
			}
		}
	}
	for i := range issues {
		sort.Strings(issues[i].Children)
		sort.Strings(issues[i].BlockedBy)
		sort.Strings(issues[i].Dependents)
	}
}

// parseTime は RFC3339 のタイムスタンプを解釈する。空文字や不正な値は
// ゼロ値として扱う（タイムスタンプは必須フィールドではない）。
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
