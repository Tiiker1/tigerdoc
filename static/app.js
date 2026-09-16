"use strict";

const $ = (id) => document.getElementById(id);

const state = {
  containers: [],
  filter: "",
  stateFilter: "all",
  auto: true,
  timer: null,
  log: {
    id: null,
    name: null,
    ws: null,
    closedByUser: false,
    reconnectAttempts: 0,
    buf: "",
  },
};

const MAX_LOG_NODES = 3000;

/* ---------- helpers ---------- */

function esc(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  }[c]));
}

function timeAgo(unixSec) {
  if (!unixSec) return "—";
  const diff = Math.max(0, Date.now() / 1000 - unixSec);
  if (diff < 60) return "just now";
  if (diff < 3600) return Math.floor(diff / 60) + "m ago";
  if (diff < 86400) return Math.floor(diff / 3600) + "h ago";
  if (diff < 86400 * 30) return Math.floor(diff / 86400) + "d ago";
  return new Date(unixSec * 1000).toLocaleDateString();
}

function toast(msg, isErr) {
  const el = $("toast");
  el.textContent = msg;
  el.classList.toggle("err", !!isErr);
  el.classList.remove("hidden");
  clearTimeout(el._t);
  el._t = setTimeout(() => el.classList.add("hidden"), 2600);
}

function setConnected(ok) {
  const d = $("connDot");
  d.className = "dot " + (ok ? "ok" : "err");
}

function statusLine(msg, kind) {
  const el = $("statusLine");
  if (!msg) { el.classList.add("hidden"); el.textContent = ""; return; }
  el.classList.remove("hidden");
  el.className = "status " + (kind || "info");
  el.textContent = msg;
}

/* ---------- api ---------- */

async function api(url, opts) {
  const res = await fetch(url, opts);
  let body = null;
  try { body = await res.json(); } catch (_) { /* no json */ }
  if (!res.ok) {
    throw new Error((body && body.error) || ("HTTP " + res.status));
  }
  return body;
}

async function loadSystem() {
  try {
    const i = await api("/api/system");
    $("engineInfo").textContent =
      "docker " + i.serverVersion + " · api " + i.apiVersion +
      " · " + (i.hostname || i.osType) + " · " + (i.architecture || "") +
      " · " + i.running + "/" + i.containers + " running";
    setConnected(true);
    statusLine("");
  } catch (e) {
    setConnected(false);
    statusLine("cannot reach Docker engine: " + esc(e.message), "error");
  }
}

async function loadContainers() {
  try {
    state.containers = await api("/api/containers");
    setConnected(true);
    statusLine("");
  } catch (e) {
    setConnected(false);
    statusLine("failed to load containers: " + esc(e.message), "error");
    return;
  }
  render();
}

/* ---------- rendering ---------- */

function matching(rows) {
  const f = state.filter.toLowerCase();
  return rows.filter((c) => {
    if (state.stateFilter === "running" && c.state !== "running") return false;
    if (state.stateFilter === "paused" && c.state !== "paused") return false;
    if (state.stateFilter === "stopped" && ["exited", "created", "dead"].indexOf(c.state) < 0) return false;
    if (!f) return true;
    return (c.name + " " + c.image + " " + c.id).toLowerCase().indexOf(f) >= 0;
  });
}

function buttonsFor(c) {
  const b = [];
  const add = (a, extra) => b.push(
    '<button class="btn' + (extra || "") + '" data-action="' + a + '" data-id="' + esc(c.id) + '" data-name="' + esc(c.name) + '">' + a + "</button>"
  );
  add("logs", " primary");
  switch (c.state) {
    case "running":
      add("stop"); add("restart"); add("pause");
      add("remove", " danger");
      break;
    case "paused":
      add("unpause"); add("stop"); add("restart");
      break;
    case "exited":
    case "created":
    case "dead":
      add("start", " primary");
      add("remove", " danger");
      break;
    default: // restarting, removing
      break;
  }
  return b.join("");
}

function render() {
  const rows = matching(state.containers);
  $("count").textContent = rows.length + "/" + state.containers.length + " containers";
  const tbody = $("rows");
  tbody.innerHTML = "";

  for (const c of rows) {
    const tr = document.createElement("tr");
    const ports = (c.ports && c.ports.length)
      ? c.ports.map((p) => '<span class="port">' + esc(p) + "</span>").join("")
      : '<span class="muted" style="color:var(--muted)">—</span>';
    tr.innerHTML =
      '<td><span class="badge ' + esc(c.state) + '">' + esc(c.state) + "</span></td>" +
      '<td><span class="name">' + esc(c.name || "(untitled)") + '<span class="id">' + esc(c.id) + "</span></span></td>" +
      '<td><span class="img" title="' + esc(c.image) + '">' + esc(c.image) + "</span></td>" +
      '<td><span class="status-txt">' + esc(c.status) + "</span></td>" +
      '<td class="ports">' + ports + "</td>" +
      '<td class="created">' + timeAgo(c.created) + "</td>" +
      '<td class="actions">' + buttonsFor(c) + "</td>";
    tbody.appendChild(tr);
  }

  $("empty").classList.toggle("hidden", rows.length > 0);
  $("loading").classList.add("hidden");
}

/* ---------- actions ---------- */

async function runAction(id, name, action) {
  if (action === "remove") {
    if (!window.confirm("Remove container \"" + name + "\"? This deletes it.")) return;
  }
  try {
    await api("/api/containers/" + encodeURIComponent(id) + "/" + action, { method: "POST" });
    toast(action + " sent: " + name);
    loadContainers();
  } catch (e) {
    toast("failed: " + e.message, true);
    loadContainers();
  }
}

/* ---------- logs ---------- */

function openLogs(id, name) {
  state.log.id = id;
  state.log.name = name;
  state.log.closedByUser = false;
  state.log.reconnectAttempts = 0;
  $("logTitle").textContent = "logs · " + name;
  $("logModal").classList.remove("hidden");
  $("logOutput").innerHTML = "";
  connectLogs();
}

function closeLogs() {
  state.log.closedByUser = true;
  if (state.log.ws) { state.log.ws.close(); state.log.ws = null; }
  state.log.id = null;
  $("logModal").classList.add("hidden");
}

function logUrl() {
  const q = new URLSearchParams({ id: state.log.id, tail: $("logTail").value });
  if ($("logFollow").checked) q.set("follow", "1");
  if ($("logTs").checked) q.set("timestamps", "1");
  const proto = location.protocol === "https:" ? "wss" : "ws";
  return proto + "://" + location.host + "/ws/logs?" + q.toString();
}

function logOutput() {
  return $("logOutput");
}

function logWrap() {
  return document.querySelector(".log-wrap");
}

function atBottom() {
  const w = logWrap();
  return w.scrollHeight - w.scrollTop - w.clientHeight < 60;
}

function connectLogs() {
  if (!state.log.id) return;
  if (state.log.ws) { state.log.ws.onclose = null; state.log.ws.close(); state.log.ws = null; }

  $("liveDot").classList.toggle("hidden", !$("logFollow").checked);

  const ws = new WebSocket(logUrl());
  state.log.ws = ws;

  ws.onmessage = (ev) => {
    state.log.reconnectAttempts = 0;
    appendLog(ev.data);
    if (state.log.ws) {
      // data took: reset nothing
    }
  };
  ws.onerror = () => { /* onclose handles it */ };
  ws.onclose = (ev) => {
    if (state.log.closedByUser || !state.log.id) return;
    appendLog("— stream " + (ev.code === 1000 ? "ended" : "disconnected (" + ev.code + ")") + " —", "err");
    if ($("logFollow").checked && normError(ev)) {
      scheduleReconnect();
    }
  };
}

function normError(ev) {
  return ev.code !== 1000 && ev.code !== 1001;
}

function scheduleReconnect() {
  if (state.log.reconnectAttempts >= 5) return;
  state.log.reconnectAttempts++;
  const tries = state.log.reconnectAttempts;
  setTimeout(() => {
    if (!state.log.closedByUser && state.log.id && tries === state.log.reconnectAttempts) {
      appendLog("— reconnecting (" + tries + ") —", "err");
      connectLogs();
    }
  }, 2000 * tries);
}

function appendLine(text, cls) {
  const out = logOutput();
  const div = document.createElement("span");
  div.className = "line" + (cls ? " " + cls : "");
  div.textContent = text;
  out.appendChild(div);
}

function flushPartial() {
  if (state.log.buf) {
    appendLine(state.log.buf);
    state.log.buf = "";
  }
}

function appendLog(text, cls) {
  if (cls === "err") {
    flushPartial();
    appendLine(text, "err");
  } else {
    // Buffer across frames so a line split by a websocket chunk stays intact.
    state.log.buf += text;
    const parts = state.log.buf.split("\n");
    state.log.buf = parts.pop();
    for (const line of parts) appendLine(line);
  }
  const out = logOutput();
  while (out.childNodes.length > MAX_LOG_NODES) out.removeChild(out.firstChild);
  if (atBottom()) logWrap().scrollTop = logWrap().scrollHeight;
}

function clearLogs() {
  state.log.buf = "";
  logOutput().innerHTML = "";
}

async function copyLogs() {
  try {
    await navigator.clipboard.writeText(logOutput().textContent);
    toast("logs copied");
  } catch (_) {
    toast("copy failed", true);
  }
}

/* ---------- polling ---------- */

function setAuto(on) {
  state.auto = on;
  clearInterval(state.timer);
  if (on) {
    state.timer = setInterval(() => { loadContainers(); }, 3000);
  }
  $("autoToggle").classList.toggle("on", on);
  loadContainers();
}

/* ---------- wiring ---------- */

$("refreshBtn").addEventListener("click", () => { loadSystem(); loadContainers(); });
$("autoToggle").addEventListener("click", () => setAuto(!state.auto));

$("search").addEventListener("input", (e) => { state.filter = e.target.value.trim(); render(); });

document.querySelectorAll(".seg-btn").forEach((btn) => {
  btn.addEventListener("click", () => {
    document.querySelectorAll(".seg-btn").forEach((b) => b.classList.remove("active"));
    btn.classList.add("active");
    state.stateFilter = btn.dataset.state;
    render();
  });
});

$("rows").addEventListener("click", (e) => {
  const btn = e.target.closest("button[data-action]");
  if (!btn) return;
  if (btn.dataset.action === "logs") {
    openLogs(btn.dataset.id, btn.dataset.name);
  } else {
    runAction(btn.dataset.id, btn.dataset.name, btn.dataset.action);
  }
});

$("logClose").addEventListener("click", closeLogs);
$("logClear").addEventListener("click", clearLogs);
$("logCopy").addEventListener("click", copyLogs);
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && !$("logModal").classList.contains("hidden")) closeLogs();
});

$("logFollow").addEventListener("change", () => { if (state.log.id) connectLogs(); });
$("logTs").addEventListener("change", () => { if (state.log.id) connectLogs(); });
$("logTail").addEventListener("change", () => { if (state.log.id) connectLogs(); });

logWrap().addEventListener("scroll", () => {
  document.documentElement.dataset.atBottom = atBottom() ? "1" : "0";
});

/* ---------- boot ---------- */

loadSystem();
setAuto(true);