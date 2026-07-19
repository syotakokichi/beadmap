/* beadmap UI — ビルドなしの素のJS。
   データ取得は dataSource に集約する（静的デモではここだけ差し替える）。 */
"use strict";

const dataSource = {
  snapshot: () => fetchJSON("/api/snapshot"),
  closed: () => fetchJSON("/api/closed"),
};

async function fetchJSON(url) {
  const res = await fetch(url, { cache: "no-store" });
  if (!res.ok) throw new Error(`${url}: HTTP ${res.status}`);
  return res.json();
}

const state = {
  snap: null,        // /api/snapshot の応答
  closed: null,      // /api/closed の応答（opt-in で取得するまで null）
  view: "in_progress", // in_progress | ready | blocked | open | closed
  expandAll: false,
  expanded: new Set(), // 個別に開いた epic の ID
  selected: null,
  filters: { priority: "", label: "", search: "" },
};

const $ = (sel) => document.querySelector(sel);

/* ---------- データアクセス ---------- */

function allIssues() {
  const open = state.snap ? state.snap.issues : [];
  const closed = state.closed ? state.closed.issues : [];
  return open.concat(closed);
}

function byId(id) {
  return allIssues().find((i) => i.id === id) || null;
}

function readySet() { return new Set(state.snap ? state.snap.ready : []); }
function blockedSet() { return new Set(state.snap ? state.snap.blocked : []); }

/* ---------- 時刻表示 ---------- */

function relTime(iso) {
  if (!iso) return "";
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return "";
  const sec = Math.floor((Date.now() - t) / 1000);
  if (sec < 60) return "たった今";
  if (sec < 3600) return `${Math.floor(sec / 60)}分前`;
  if (sec < 86400) return `${Math.floor(sec / 3600)}時間前`;
  return `${Math.floor(sec / 86400)}日前`;
}

function stallDays(iso) {
  if (!iso) return null;
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return null;
  return Math.floor((Date.now() - t) / 86400000);
}

function hhmmss(iso) {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "" : d.toTimeString().slice(0, 8);
}

/* ---------- ヘッダ（staleness 常時表示） ---------- */

function renderHeader() {
  const s = state.snap;
  $("#source").textContent = s.source_path || "";
  const src = s.ready_source === "bd" ? "bd" : "fallback（近似）";
  $("#freshness").innerHTML =
    `jsonl更新 ${relTime(s.file_mod_time) || "不明"} · 取得 ${hhmmss(s.generated_at)} · ready算出: ` +
    (s.ready_source === "bd" ? "bd" : `<span class="warn">${src}</span>`);

  const banner = $("#stale-banner");
  const messages = [];
  if (s.stale) messages.push(s.stale_reason || "最新データの取得に失敗したため前回のスナップショットを表示中");
  for (const w of s.warnings || []) messages.push(w);
  banner.textContent = messages.join(" / ");
  banner.classList.toggle("hidden", messages.length === 0);
}

/* ---------- サマリータイル ---------- */

const TILES = [
  { key: "in_progress", label: "進行中" },
  { key: "ready", label: "着手可能" },
  { key: "blocked", label: "ブロック中" },
  { key: "open", label: "open全体" },
  { key: "closed", label: "closed" },
];

function tileCount(key) {
  const c = state.snap.counts || {};
  switch (key) {
    case "in_progress": return c.in_progress || 0;
    case "ready": return (state.snap.ready || []).length;
    case "blocked": return (state.snap.blocked || []).length;
    case "open": return (c.open || 0) + (c.in_progress || 0) + (c.deferred || 0);
    case "closed": return c.closed || 0;
  }
  return 0;
}

function renderTiles() {
  const el = $("#tiles");
  el.innerHTML = "";
  for (const t of TILES) {
    const div = document.createElement("div");
    div.className = "tile" + (state.view === t.key ? " active" : "") +
      (t.key === "closed" && !state.closed ? " optin" : "");
    div.innerHTML = `<span class="n">${tileCount(t.key)}</span><span class="t">${t.label}${
      t.key === "closed" && !state.closed ? " ⊕" : ""}</span>`;
    div.onclick = () => selectView(t.key);
    el.appendChild(div);
  }
}

async function selectView(key) {
  state.view = key;
  if (key === "closed" && !state.closed) {
    state.closed = await dataSource.closed().catch((e) => ({ issues: [], error: String(e) }));
  }
  render();
}

/* ---------- フィルタ ---------- */

function applyFilters(issues) {
  const f = state.filters;
  return issues.filter((i) => {
    if (f.priority !== "" && String(i.priority) !== f.priority) return false;
    if (f.label && !(i.labels || []).includes(f.label)) return false;
    if (f.search) {
      const q = f.search.toLowerCase();
      if (!i.id.toLowerCase().includes(q) && !(i.title || "").toLowerCase().includes(q)) return false;
    }
    return true;
  });
}

function renderFilterOptions() {
  const issues = allIssues();
  const priorities = [...new Set(issues.map((i) => i.priority))].sort();
  const labels = [...new Set(issues.flatMap((i) => i.labels || []))].sort();

  const fp = $("#f-priority");
  const fl = $("#f-label");
  const keep = (sel, v) => { if ([...sel.options].some((o) => o.value === v)) sel.value = v; };
  fp.innerHTML = '<option value="">priority: 全て</option>' +
    priorities.map((p) => `<option value="${p}">P${p}</option>`).join("");
  fl.innerHTML = '<option value="">ラベル: 全て</option>' +
    labels.map((l) => `<option>${l}</option>`).join("");
  keep(fp, state.filters.priority);
  keep(fl, state.filters.label);
}

/* ---------- 一覧 / 親子ツリー ---------- */

function badge(text, cls) { return `<span class="b ${cls || ""}">${text}</span>`; }

function priorityBadge(i) {
  return badge(`P${i.priority}`, i.priority === 1 ? "p1" : i.priority === 2 ? "p2" : "");
}

function rowHTML(i, opts = {}) {
  const parts = [];
  if (opts.twist !== undefined) parts.push(`<span class="twist">${opts.twist}</span>`);
  parts.push(`<span class="id">${i.id}</span>`);
  parts.push(`<span class="title">${escapeHTML(i.title)}</span>`);
  if (i.issue_type === "epic") parts.push(badge("epic", "epic"));
  parts.push(priorityBadge(i));
  if (opts.badges) parts.push(opts.badges);
  return parts.join("");
}

function escapeHTML(s) {
  return String(s ?? "").replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

function addRow(parent, i, opts = {}) {
  const div = document.createElement("div");
  div.className = "row" + (opts.kid ? " kid" : "") + (state.selected === i.id ? " sel" : "") +
    (i.status === "closed" ? " closed" : "");
  div.innerHTML = rowHTML(i, opts);
  div.onclick = (e) => {
    if (opts.onTwist && e.target.classList.contains("twist")) { opts.onTwist(); return; }
    selectIssue(i.id);
  };
  parent.appendChild(div);
}

function statusBadge(i) {
  if (i.status === "deferred") return badge("❄ deferred", "st-deferred");
  return badge(i.status, `st-${i.status}`);
}

function stallBadge(i) {
  const d = stallDays(i.updated_at);
  if (d === null) return "";
  const label = d >= 1 ? `${d}日 更新なし` : "今日更新";
  return badge(label, d >= 3 ? "stall" : d >= 1 ? "stall mild" : "");
}

function renderList() {
  const el = $("#list");
  el.innerHTML = "";
  const title = document.createElement("div");
  title.className = "pane-title";
  el.appendChild(title);

  const ready = readySet();
  const blocked = blockedSet();
  const open = state.snap.issues; // closed を含まない
  let rows = [];

  switch (state.view) {
    case "in_progress":
      title.textContent = "進行中 — 更新が古い順（停滞から目に入る）";
      rows = applyFilters(open.filter((i) => i.status === "in_progress"))
        .sort((a, b) => new Date(a.updated_at || 0) - new Date(b.updated_at || 0));
      for (const i of rows) addRow(el, i, { badges: stallBadge(i) });
      break;

    case "ready":
      title.textContent = "着手可能 — ブロックなしの open" +
        (state.snap.ready_source === "fallback" ? "（fallback 近似）" : "");
      rows = applyFilters(open.filter((i) => ready.has(i.id)))
        .sort((a, b) => a.priority - b.priority || a.id.localeCompare(b.id));
      for (const i of rows) addRow(el, i, { badges: badge("ready", "ready") });
      break;

    case "blocked":
      title.textContent = "ブロック中 — 依存先が閉じるまで待ち";
      rows = applyFilters(open.filter((i) => blocked.has(i.id)))
        .sort((a, b) => a.priority - b.priority || a.id.localeCompare(b.id));
      for (const i of rows) {
        const waiting = (i.blocked_by || []).join(", ");
        addRow(el, i, { badges: badge(`待ち: ${waiting}`, "blocked") });
      }
      break;

    case "open":
      title.textContent = "open 全体 — 親子ツリー（▸ で展開）";
      renderTree(el, applyFilters(open));
      return;

    case "closed": {
      title.textContent = "closed — 完了順（opt-in 取得）";
      const cl = state.closed ? state.closed.issues : [];
      rows = applyFilters(cl)
        .sort((a, b) => new Date(b.closed_at || 0) - new Date(a.closed_at || 0));
      for (const i of rows) addRow(el, i, { badges: badge(`close ${relTime(i.closed_at)}`, "st-closed") });
      break;
    }
  }
  if (!rows.length && state.view !== "open") {
    el.insertAdjacentHTML("beforeend", '<div class="empty">該当なし</div>');
  }
}

function renderTree(el, issues) {
  const ids = new Set(issues.map((i) => i.id));
  const roots = issues.filter((i) => !i.parent_id || !ids.has(i.parent_id))
    .sort((a, b) => (b.issue_type === "epic") - (a.issue_type === "epic") ||
      a.priority - b.priority || a.id.localeCompare(b.id));
  const kids = (id) => issues.filter((i) => i.parent_id === id)
    .sort((a, b) => a.priority - b.priority || a.id.localeCompare(b.id));

  let count = 0;
  for (const r of roots) {
    const children = kids(r.id);
    const isOpen = state.expandAll || state.expanded.has(r.id);
    addRow(el, r, {
      twist: children.length ? (isOpen ? "▾" : "▸") : "",
      badges: statusBadge(r),
      onTwist: children.length ? () => { toggleEpic(r.id); } : undefined,
    });
    count++;
    if (isOpen) {
      for (const c of children) {
        addRow(el, c, { kid: true, badges: statusBadge(c) + (c.status === "in_progress" ? stallBadge(c) : "") });
        count++;
      }
    }
  }
  if (!count) el.insertAdjacentHTML("beforeend", '<div class="empty">該当なし</div>');
}

function toggleEpic(id) {
  if (state.expanded.has(id)) state.expanded.delete(id);
  else state.expanded.add(id);
  render();
}

/* ---------- 詳細ペイン（一段先のみ） ---------- */

function issueLink(id) {
  const i = byId(id);
  const label = i ? `${id} ${escapeHTML(i.title)}` : id;
  const st = i ? ` ${statusBadge(i)}` : "";
  return `<span class="link" data-id="${id}">${label}</span>${st}`;
}

function linkList(ids) {
  if (!ids || !ids.length) return '<span class="none">（なし）</span>';
  return ids.map(issueLink).join("<br>");
}

function renderDetail() {
  const el = $("#detail");
  const i = state.selected ? byId(state.selected) : null;
  if (!i) {
    el.innerHTML = '<div class="pane-title">詳細</div><div class="empty">左の一覧から選択してください</div>';
    return;
  }
  const ready = readySet();
  const blocked = blockedSet();
  const stateBadges = [
    statusBadge(i), priorityBadge(i),
    i.issue_type === "epic" ? badge("epic", "epic") : "",
    ready.has(i.id) ? badge("ready", "ready") : "",
    blocked.has(i.id) ? badge("blocked", "blocked") : "",
    ...(i.labels || []).map((l) => badge(escapeHTML(l))),
  ].filter(Boolean).join("");

  const meta = [
    i.updated_at ? `更新 ${relTime(i.updated_at)}` : "",
    i.created_at ? `作成 ${relTime(i.created_at)}` : "",
    i.status === "closed" && i.closed_at ? `close ${relTime(i.closed_at)}` : "",
    i.assignee ? `担当 ${escapeHTML(i.assignee)}` : "",
  ].filter(Boolean).join(" · ");

  const fold = (label, text) => text
    ? `<details><summary>${label}</summary><pre>${escapeHTML(text)}</pre></details>` : "";

  el.innerHTML = `
    <div class="pane-title">詳細（親・子・依存は一段先のみ）</div>
    <h2>${escapeHTML(i.title)}</h2>
    <div class="did">${i.id} · ${meta}</div>
    <div class="badges">${stateBadges}</div>
    <dl>
      <dt>親</dt><dd>${i.parent_id ? issueLink(i.parent_id) : '<span class="none">（なし）</span>'}</dd>
      <dt>子</dt><dd>${linkList(i.children)}</dd>
      <dt>blocker（これが閉じるのを待っている）</dt><dd>${linkList(i.blocked_by)}</dd>
      <dt>依存元（この issue を待っている）</dt><dd>${linkList(i.dependents)}</dd>
    </dl>
    ${i.status === "closed" && i.close_reason ? fold("close 理由", i.close_reason) : ""}
    ${fold("description", i.description)}
    ${fold("design", i.design)}
    ${fold("acceptance", i.acceptance_criteria)}
    ${fold("notes", i.notes)}
  `;
  el.querySelectorAll(".link").forEach((a) => {
    a.onclick = () => selectIssue(a.dataset.id);
  });
}

async function selectIssue(id) {
  // closed 未取得の issue（close 済みの親など）を参照した場合は opt-in 取得してから表示
  if (!byId(id) && !state.closed) {
    state.closed = await dataSource.closed().catch(() => null);
  }
  state.selected = id;
  render();
}

/* ---------- 全体 ---------- */

function render() {
  renderHeader();
  renderTiles();
  renderFilterOptions();
  renderList();
  renderDetail();
  const t = $("#toggle-expand");
  t.classList.toggle("on", state.expandAll);
  t.disabled = state.view !== "open";
  t.textContent = state.expandAll ? "epicのみに戻す" : "子を展開";
}

async function refresh() {
  try {
    state.snap = await dataSource.snapshot();
    if (state.closed) state.closed = await dataSource.closed().catch(() => state.closed);
  } catch (e) {
    if (!state.snap) {
      document.body.innerHTML = `<div class="empty" style="padding:40px">データを取得できません: ${escapeHTML(String(e))}</div>`;
      return;
    }
    // 取得失敗。手元の前回スナップショットを保持し、次の refresh で復帰を試みる
  }
  render();
}

$("#f-priority").addEventListener("change", (e) => { state.filters.priority = e.target.value; render(); });
$("#f-label").addEventListener("change", (e) => { state.filters.label = e.target.value; render(); });
$("#f-search").addEventListener("input", (e) => { state.filters.search = e.target.value.trim(); render(); });
$("#toggle-expand").addEventListener("click", () => { state.expandAll = !state.expandAll; render(); });

refresh();
setInterval(refresh, 30000); // 常時開いておける用の自動更新
