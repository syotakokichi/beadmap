/* beadmap UI — ビルドなしの素のJS。
   データ取得は dataSource に集約する（静的デモではここだけ差し替える）。 */
"use strict";

/* 静的デモモード: demo-config.js（デモビルドのみ同梱）が window.BEADMAP_DEMO を
   定義していれば、API の代わりに同梱スナップショット JSON を読む。 */
const DEMO = window.BEADMAP_DEMO || null;
let demoDataset = DEMO
  ? DEMO.datasets.find((d) => d.id === DEMO.initial) || DEMO.datasets[0]
  : null;
let demoNeedsViewPick = !!DEMO; // データセット読み込み直後に中身のあるビューを初期選択する

const dataSource = {
  snapshot: () => fetchJSON(demoDataset ? demoDataset.snapshot : "/api/snapshot"),
  closed: () => fetchJSON(demoDataset ? demoDataset.closed : "/api/closed"),
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

function dateYMD(iso) {
  const d = new Date(iso);
  if (!iso || Number.isNaN(d.getTime())) return "";
  // 閲覧環境のタイムゾーンで YYYY-MM-DD（UTC の ISO 文字列を slice すると日付がずれる）
  return d.toLocaleDateString("sv-SE");
}

/* ---------- ヘッダ（staleness 常時表示） ---------- */

function renderHeader() {
  const s = state.snap;
  if (DEMO) {
    $("#source").textContent = s.source_path || "";
    $("#freshness").innerHTML =
      `スナップショット取得 ${dateYMD(s.generated_at)}（${relTime(s.generated_at) || "不明"}） · ready算出: ` +
      (s.ready_source === "bd" ? "bd" : '<span class="warn">fallback（近似）</span>');
    const db = $("#demo-banner");
    db.innerHTML =
      `静的デモ — ${dateYMD(s.generated_at)} 時点のスナップショットを表示中` +
      `（データは毎日自動更新・変化があった日のみ反映。ページ内の自動リロードなし）。` +
      `起票・更新は bd CLI の役割。手元の実データを見るにはローカルで beadmap を起動 ` +
      `（<a href="https://github.com/syotakokichi/beadmap">README</a>）`;
    db.classList.remove("hidden");
  } else {
    $("#source").textContent = s.source_path || "";
    const src = s.ready_source === "bd" ? "bd" : "fallback（近似）";
    $("#freshness").innerHTML =
      `jsonl更新 ${relTime(s.file_mod_time) || "不明"} · 取得 ${hhmmss(s.generated_at)} · ready算出: ` +
      (s.ready_source === "bd" ? "bd" : `<span class="warn">${src}</span>`);
  }

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
    priorities.map((p) => `<option value="${escapeHTML(p)}">P${escapeHTML(p)}</option>`).join("");
  fl.innerHTML = '<option value="">ラベル: 全て</option>' +
    labels.map((l) => `<option>${escapeHTML(l)}</option>`).join("");
  keep(fp, state.filters.priority);
  keep(fl, state.filters.label);
}

/* ---------- 一覧 / 親子ツリー ---------- */

function badge(text, cls) { return `<span class="b ${cls || ""}">${text}</span>`; }

function priorityBadge(i) {
  return badge(`P${escapeHTML(i.priority)}`, i.priority === 1 ? "p1" : i.priority === 2 ? "p2" : "");
}

function rowHTML(i, opts = {}) {
  const parts = [];
  if (opts.twist !== undefined) parts.push(`<span class="twist">${opts.twist}</span>`);
  parts.push(`<span class="id">${escapeHTML(i.id)}</span>`);
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

/* ---------- Markdown レンダラー（自前・ゼロ依存）
   bead の description / design / notes を読みやすく表示するための最小実装。
   対応: 見出し・箇条書き/番号リスト（ネスト・チェックボックス）・表・
   コードフェンス・引用・水平線・太字・インラインコード・リンク。
   全テキストは先にエスケープしてから自前タグに変換する（XSS 防止）。 ---------- */

function mdInline(s) {
  return String(s).split("`").map((part, idx) => {
    if (idx % 2 === 1) return `<code>${escapeHTML(part)}</code>`;
    let t = escapeHTML(part);
    t = t.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
    t = t.replace(/\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)/g,
      '<a href="$2" target="_blank" rel="noopener">$1</a>');
    return t;
  }).join("");
}

function mdTable(rows) {
  const cells = (r) => r.replace(/^\s*\|/, "").replace(/\|\s*$/, "")
    .split("|").map((c) => mdInline(c.trim()));
  let header = null;
  let body = rows;
  if (rows.length >= 2 && /^[\s|:\-]+$/.test(rows[1])) {
    header = cells(rows[0]);
    body = rows.slice(2);
  }
  let out = "<table>";
  if (header) out += "<thead><tr>" + header.map((c) => `<th>${c}</th>`).join("") + "</tr></thead>";
  out += "<tbody>" + body.map((r) =>
    "<tr>" + cells(r).map((c) => `<td>${c}</td>`).join("") + "</tr>").join("") + "</tbody>";
  return out + "</table>";
}

function mdRender(src) {
  const lines = String(src ?? "").replace(/\r\n/g, "\n").split("\n");
  const html = [];
  const stack = []; // 開いているリストの種別（ul / ol）
  const closeTo = (depth) => { while (stack.length > depth) html.push(`</${stack.pop()}>`); };
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    if (/^```/.test(line)) { // コードフェンス
      closeTo(0);
      const buf = [];
      i++;
      while (i < lines.length && !/^```/.test(lines[i])) { buf.push(lines[i]); i++; }
      i++;
      html.push(`<pre class="code"><code>${escapeHTML(buf.join("\n"))}</code></pre>`);
      continue;
    }
    if (/^\s*\|/.test(line)) { // 表
      closeTo(0);
      const buf = [];
      while (i < lines.length && /^\s*\|/.test(lines[i])) { buf.push(lines[i]); i++; }
      html.push(mdTable(buf));
      continue;
    }
    const h = line.match(/^(#{1,6})\s+(.*)$/);
    if (h) { // 見出し（詳細ペイン内なので h3 以下に落とす）
      closeTo(0);
      const lv = Math.min(h[1].length + 2, 6);
      html.push(`<h${lv}>${mdInline(h[2])}</h${lv}>`);
      i++;
      continue;
    }
    if (/^\s*(---+|\*\*\*+)\s*$/.test(line)) { closeTo(0); html.push("<hr>"); i++; continue; }
    if (/^\s*>\s?/.test(line)) { // 引用
      closeTo(0);
      const buf = [];
      while (i < lines.length && /^\s*>\s?/.test(lines[i])) {
        buf.push(lines[i].replace(/^\s*>\s?/, ""));
        i++;
      }
      html.push(`<blockquote>${buf.map(mdInline).join("<br>")}</blockquote>`);
      continue;
    }
    const li = line.match(/^(\s*)([-*+]|\d+[.)])\s+(.*)$/);
    if (li) { // リスト（インデント 2 スペース = 1 段）
      const depth = Math.floor(li[1].replace(/\t/g, "  ").length / 2) + 1;
      const type = /^[-*+]$/.test(li[2]) ? "ul" : "ol";
      closeTo(depth);
      if (stack.length === depth && stack[depth - 1] !== type) closeTo(depth - 1);
      while (stack.length < depth) { html.push(`<${type}>`); stack.push(type); }
      const cb = li[3].match(/^\[( |x|X)\]\s+(.*)$/);
      html.push(cb
        ? `<li><input type="checkbox" disabled${cb[1] === " " ? "" : " checked"}>${mdInline(cb[2])}</li>`
        : `<li>${mdInline(li[3])}</li>`);
      i++;
      continue;
    }
    if (/^\s*$/.test(line)) { closeTo(0); i++; continue; }
    // 段落（特別な行が来るまで連結）
    closeTo(0);
    const buf = [line];
    i++;
    while (i < lines.length &&
      !/^\s*$/.test(lines[i]) &&
      !/^(```|\s*\||\s*>|#{1,6}\s|\s*([-*+]|\d+[.)])\s+|\s*(---+|\*\*\*+)\s*$)/.test(lines[i])) {
      buf.push(lines[i]);
      i++;
    }
    html.push(`<p>${buf.map(mdInline).join("<br>")}</p>`);
  }
  closeTo(0);
  return `<div class="md">${html.join("")}</div>`;
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
  return badge(escapeHTML(i.status), `st-${escapeHTML(i.status)}`);
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
        addRow(el, i, { badges: badge(`待ち: ${escapeHTML(waiting)}`, "blocked") });
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
  const label = i ? `${escapeHTML(id)} ${escapeHTML(i.title)}` : escapeHTML(id);
  const st = i ? ` ${statusBadge(i)}` : "";
  return `<span class="link" data-id="${escapeHTML(id)}">${label}</span>${st}`;
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

  const fold = (label, text, open) => text
    ? `<details${open ? " open" : ""}><summary>${label}</summary>${mdRender(text)}</details>` : "";

  el.innerHTML = `
    <div class="pane-title">詳細（親・子・依存は一段先のみ）</div>
    <h2>${escapeHTML(i.title)}</h2>
    <div class="did">${escapeHTML(i.id)} · ${meta}</div>
    <div class="badges">${stateBadges}</div>
    <dl>
      <dt>親</dt><dd>${i.parent_id ? issueLink(i.parent_id) : '<span class="none">（なし）</span>'}</dd>
      <dt>子</dt><dd>${linkList(i.children)}</dd>
      <dt>blocker（これが閉じるのを待っている）</dt><dd>${linkList(i.blocked_by)}</dd>
      <dt>依存元（この issue を待っている）</dt><dd>${linkList(i.dependents)}</dd>
    </dl>
    ${i.status === "closed" && i.close_reason ? fold("close 理由", i.close_reason, true) : ""}
    ${fold("description", i.description, true)}
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

/* 静的デモ: 初期ビューが空だと第一印象が「該当なし」になるため、
   件数のあるビューへフォールバックする（表示内容は実データのまま）。 */
async function demoPickView() {
  for (const k of ["in_progress", "ready", "blocked", "open"]) {
    if (tileCount(k) > 0) { state.view = k; return; }
  }
  if ((state.snap.counts || {}).closed) {
    state.closed = await dataSource.closed().catch(() => null);
    if (state.closed) state.view = "closed";
  }
}

async function refresh() {
  try {
    state.snap = await dataSource.snapshot();
    if (demoNeedsViewPick) { demoNeedsViewPick = false; await demoPickView(); }
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

/* ---------- ペイン幅の調整（ドラッグ・localStorage 保存） ---------- */

(function initSplitter() {
  const panes = document.querySelector(".panes");
  const splitter = $("#splitter");
  const saved = Number(localStorage.getItem("beadmap-list-w"));
  if (saved >= 20 && saved <= 70) panes.style.setProperty("--list-w", saved + "%");

  splitter.addEventListener("mousedown", (e) => {
    e.preventDefault();
    splitter.classList.add("drag");
    const move = (ev) => {
      const rect = panes.getBoundingClientRect();
      const pct = Math.max(20, Math.min(70, ((ev.clientX - rect.left) / rect.width) * 100));
      panes.style.setProperty("--list-w", pct + "%");
      localStorage.setItem("beadmap-list-w", String(Math.round(pct)));
    };
    const up = () => {
      splitter.classList.remove("drag");
      window.removeEventListener("mousemove", move);
      window.removeEventListener("mouseup", up);
    };
    window.addEventListener("mousemove", move);
    window.addEventListener("mouseup", up);
  });
})();

$("#f-priority").addEventListener("change", (e) => { state.filters.priority = e.target.value; render(); });
$("#f-label").addEventListener("change", (e) => { state.filters.label = e.target.value; render(); });
$("#f-search").addEventListener("input", (e) => { state.filters.search = e.target.value.trim(); render(); });
$("#toggle-expand").addEventListener("click", () => { state.expandAll = !state.expandAll; render(); });

/* ---------- 静的デモ: データセット切替 ---------- */

(function initDemoSelect() {
  if (!DEMO) return;
  const sel = $("#demo-dataset");
  sel.innerHTML = DEMO.datasets.map((d) =>
    `<option value="${escapeHTML(d.id)}"${d.id === demoDataset.id ? " selected" : ""}>${escapeHTML(d.label)}</option>`
  ).join("");
  sel.classList.remove("hidden");
  sel.addEventListener("change", () => {
    demoDataset = DEMO.datasets.find((d) => d.id === sel.value) || DEMO.datasets[0];
    state.snap = null;
    state.closed = null;
    state.selected = null;
    state.view = "in_progress";
    state.expandAll = false;
    state.expanded.clear();
    state.filters = { priority: "", label: "", search: "" };
    demoNeedsViewPick = true;
    refresh();
  });
})();

refresh();
if (!DEMO) setInterval(refresh, 30000); // 常時開いておける用の自動更新（静的デモでは不要）
