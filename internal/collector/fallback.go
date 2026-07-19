package collector

import "sort"

// ReadyBlockedFallback は bd 未導入時に dependencies から ready / blocked を
// 自前計算する。bd の実装と完全一致する保証はなく、bd 導入時はライブクエリを
// 正とする（README 参照）。
//
// セマンティクス（bd 1.x の実出力との突き合わせに基づく）:
//   - 対象は status == "open" のみ（in_progress / deferred / closed は含めない）
//   - type "blocks" の依存先が closed でないものが 1 つでもあれば blocked、
//     1 つもなければ ready
//   - 依存先 ID が jsonl 内に存在しない場合、その依存は無視する
func ReadyBlockedFallback(issues []Issue) (ready, blocked []string) {
	byID := make(map[string]*Issue, len(issues))
	for i := range issues {
		byID[issues[i].ID] = &issues[i]
	}
	for i := range issues {
		issue := &issues[i]
		if issue.Status != "open" {
			continue
		}
		unmet := 0
		for _, b := range issue.BlockedBy {
			if target, ok := byID[b]; ok && target.Status != "closed" {
				unmet++
			}
		}
		if unmet > 0 {
			blocked = append(blocked, issue.ID)
		} else {
			ready = append(ready, issue.ID)
		}
	}
	sort.Strings(ready)
	sort.Strings(blocked)
	return ready, blocked
}
