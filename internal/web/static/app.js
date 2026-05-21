// anthrogo web UI — vanilla JS, no framework
'use strict';

// ---- state ----
let currentSessionId = null;
let streaming = false;

// ---- settings (persisted in localStorage) ----
const cfg = {
  get token() { return localStorage.getItem('ag_token') || ''; },
  set token(v) { localStorage.setItem('ag_token', v); },
  get baseURL() { return localStorage.getItem('ag_baseURL') || location.origin; },
  set baseURL(v) { localStorage.setItem('ag_baseURL', v); },
  get stream() { return localStorage.getItem('ag_stream') !== 'false'; },
  set stream(v) { localStorage.setItem('ag_stream', v ? 'true' : 'false'); },
};

// ---- DOM refs ----
const $sessionList = document.getElementById('session-list');
const $messages    = document.getElementById('messages');
const $input       = document.getElementById('msg-input');
const $btnSend     = document.getElementById('btn-send');
const $btnNew      = document.getElementById('btn-new');
const $topTitle    = document.getElementById('topbar-title');
const $settings    = document.getElementById('settings-popover');
const $fToken      = document.getElementById('f-token');
const $fBaseURL    = document.getElementById('f-baseurl');
const $fStream     = document.getElementById('f-stream');
const $sidebar     = document.getElementById('sidebar');

// ---- API helpers ----
function apiURL(path) {
  return cfg.baseURL.replace(/\/$/, '') + path;
}

function authHeaders() {
  const h = { 'Content-Type': 'application/json' };
  if (cfg.token) h['Authorization'] = 'Bearer ' + cfg.token;
  return h;
}

async function apiFetch(path, opts) {
  const res = await fetch(apiURL(path), {
    headers: authHeaders(),
    ...opts,
  });
  return res;
}

// ---- minimal markdown renderer (~40 lines) ----
function renderMd(text) {
  let html = escapeHTML(text);
  // code fences
  html = html.replace(/```([^\n]*)\n([\s\S]*?)```/g, (_, lang, code) =>
    `<pre><code>${code}</code></pre>`);
  // inline code
  html = html.replace(/`([^`\n]+)`/g, '<code>$1</code>');
  // bold
  html = html.replace(/\*\*([^*\n]+)\*\*/g, '<strong>$1</strong>');
  // italic
  html = html.replace(/\*([^*\n]+)\*/g, '<em>$1</em>');
  // links
  html = html.replace(/\[([^\]]+)\]\((https?:\/\/[^)]+)\)/g,
    '<a href="$2" target="_blank" rel="noopener">$1</a>');
  // headings
  html = html.replace(/^### (.+)$/gm, '<h3>$1</h3>');
  html = html.replace(/^## (.+)$/gm, '<h2>$1</h2>');
  html = html.replace(/^# (.+)$/gm, '<h1>$1</h1>');
  // unordered list items (basic)
  html = html.replace(/^[*\-] (.+)$/gm, '<li>$1</li>');
  html = html.replace(/(<li>.*<\/li>\n?)+/g, m => `<ul>${m}</ul>`);
  // paragraphs: wrap non-tag lines
  html = html.replace(/^(?!<[hupoli]).+$/gm, m => `<p>${m}</p>`);
  return html;
}

function escapeHTML(s) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

// ---- session list ----
async function fetchSessions() {
  try {
    const res = await apiFetch('/v1/sessions');
    if (!res.ok) return;
    const data = await res.json();
    const sessions = data.sessions || [];
    $sessionList.innerHTML = '';
    if (sessions.length === 0) {
      $sessionList.innerHTML = '<div style="padding:12px 14px;font-size:12px;color:var(--text2)">No sessions yet</div>';
      return;
    }
    for (const s of sessions) {
      const el = document.createElement('div');
      el.className = 'session-item' + (s.id === currentSessionId ? ' active' : '');
      el.textContent = (s.label || s.id).slice(0, 36);
      el.title = s.id;
      el.dataset.id = s.id;
      el.addEventListener('click', () => loadSession(s.id));
      $sessionList.appendChild(el);
    }
  } catch (_) {}
}

// ---- load session ----
async function loadSession(id) {
  currentSessionId = id;
  $messages.innerHTML = '';
  updateActiveSession(id);
  $topTitle.textContent = id.slice(0, 24) + '…';
  try {
    const res = await apiFetch('/v1/sessions/' + encodeURIComponent(id));
    if (!res.ok) { appendError('Failed to load session: ' + res.status); return; }
    const data = await res.json();
    const records = data.records || [];
    for (const rec of records) {
      if (rec.role && rec.content) {
        appendMessage(rec.role, rec.content);
      }
    }
    scrollBottom();
  } catch (e) {
    appendError('Load error: ' + e.message);
  }
}

function updateActiveSession(id) {
  for (const el of $sessionList.querySelectorAll('.session-item')) {
    el.classList.toggle('active', el.dataset.id === id);
  }
}

// ---- new session ----
function newSession() {
  currentSessionId = null;
  $messages.innerHTML = '';
  $topTitle.textContent = 'New conversation';
  updateActiveSession(null);
  $input.focus();
  // Show empty state briefly then clear
  const es = document.getElementById('empty-state');
  if (es) es.remove();
}

// ---- message rendering ----
function appendMessage(role, content) {
  const wrap = document.createElement('div');
  wrap.className = 'msg ' + (role === 'user' ? 'user' : 'assistant');
  const roleEl = document.createElement('div');
  roleEl.className = 'msg-role';
  roleEl.textContent = role;
  wrap.appendChild(roleEl);
  const body = document.createElement('div');
  body.innerHTML = renderMd(typeof content === 'string' ? content : JSON.stringify(content));
  wrap.appendChild(body);
  $messages.appendChild(wrap);
  return body; // return body for streaming appends
}

function appendToolBubble(kind, text) {
  const wrap = document.createElement('div');
  wrap.className = 'msg tool';
  const roleEl = document.createElement('div');
  roleEl.className = 'msg-role';
  roleEl.textContent = kind;
  wrap.appendChild(roleEl);
  const body = document.createElement('div');
  body.textContent = text;
  wrap.appendChild(body);
  $messages.appendChild(wrap);
  return body;
}

function appendError(text) {
  const wrap = document.createElement('div');
  wrap.className = 'msg error';
  wrap.textContent = text;
  $messages.appendChild(wrap);
  scrollBottom();
}

function scrollBottom() {
  $messages.scrollTop = $messages.scrollHeight;
}

// ---- send message ----
async function sendMessage(text) {
  if (streaming || !text.trim()) return;
  streaming = true;
  $btnSend.disabled = true;

  // Remove empty-state if present
  const es = document.getElementById('empty-state');
  if (es) es.remove();

  appendMessage('user', text);
  scrollBottom();

  const payload = {
    session_id: currentSessionId || undefined,
    messages: [{ role: 'user', content: text }],
    stream: cfg.stream,
  };

  // Optimistically create assistant bubble
  let assistantBody = null;
  let assistantText = '';

  const spinnerMsg = document.createElement('div');
  spinnerMsg.className = 'msg assistant';
  spinnerMsg.innerHTML = '<span class="spinner"></span>';
  $messages.appendChild(spinnerMsg);
  scrollBottom();

  try {
    const res = await apiFetch('/v1/chat', {
      method: 'POST',
      body: JSON.stringify(payload),
    });

    if (!res.ok) {
      spinnerMsg.remove();
      appendError('Server error: ' + res.status);
      return;
    }

    if (cfg.stream) {
      // SSE streaming
      spinnerMsg.remove();
      const wrap = document.createElement('div');
      wrap.className = 'msg assistant';
      const roleEl = document.createElement('div');
      roleEl.className = 'msg-role';
      roleEl.textContent = 'assistant';
      wrap.appendChild(roleEl);
      assistantBody = document.createElement('div');
      wrap.appendChild(assistantBody);
      $messages.appendChild(wrap);

      const reader = res.body.getReader();
      const dec = new TextDecoder();
      let buf = '';
      let done = false;
      while (!done) {
        const { value, done: rd } = await reader.read();
        done = rd;
        if (value) buf += dec.decode(value, { stream: true });
        // parse SSE frames from buf
        let idx;
        while ((idx = buf.indexOf('\n\n')) !== -1) {
          const frame = buf.slice(0, idx);
          buf = buf.slice(idx + 2);
          const lines = frame.split('\n');
          let event = 'message', data = '';
          for (const line of lines) {
            if (line.startsWith('event: ')) event = line.slice(7).trim();
            else if (line.startsWith('data: ')) data = line.slice(6);
          }
          if (!data) continue;
          let obj;
          try { obj = JSON.parse(data); } catch (_) { obj = { text: data }; }

          switch (event) {
            case 'delta':
              assistantText += (obj.text || '');
              assistantBody.innerHTML = renderMd(assistantText);
              scrollBottom();
              break;
            case 'tool_use':
              appendToolBubble('tool_use', '[tool: ' + (obj.name || obj.tool_name || '?') +
                '(' + JSON.stringify(obj.input || obj.args || {}) + ')]');
              scrollBottom();
              break;
            case 'tool_result':
              appendToolBubble('tool_result', '[result: ' + JSON.stringify(obj.content || obj.result || obj) + ']');
              scrollBottom();
              break;
            case 'done':
              if (obj.session_id) currentSessionId = obj.session_id;
              done = true;
              break;
            case 'error':
              appendError('Stream error: ' + (obj.error || obj.message || JSON.stringify(obj)));
              done = true;
              break;
          }
        }
      }
    } else {
      // Sync response
      spinnerMsg.remove();
      const data = await res.json();
      if (data.session_id) currentSessionId = data.session_id;
      const content = data.content || data.text || JSON.stringify(data);
      appendMessage('assistant', content);
      scrollBottom();
    }
  } catch (e) {
    spinnerMsg.remove();
    appendError('Request failed: ' + e.message);
  } finally {
    streaming = false;
    $btnSend.disabled = false;
    await fetchSessions();
    updateActiveSession(currentSessionId);
  }
}

// ---- event wiring ----
$btnSend.addEventListener('click', () => {
  const text = $input.value.trim();
  if (!text) return;
  $input.value = '';
  autoResize();
  sendMessage(text);
});

$input.addEventListener('keydown', e => {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault();
    $btnSend.click();
  }
});

$input.addEventListener('input', autoResize);
function autoResize() {
  $input.style.height = '38px';
  $input.style.height = Math.min($input.scrollHeight, 160) + 'px';
}

$btnNew.addEventListener('click', newSession);

// settings
document.getElementById('btn-settings').addEventListener('click', () => {
  $fToken.value = cfg.token;
  $fBaseURL.value = cfg.baseURL;
  $fStream.checked = cfg.stream;
  $settings.classList.toggle('open');
});
document.getElementById('btn-settings-close').addEventListener('click', () => {
  cfg.token  = $fToken.value.trim();
  cfg.baseURL = $fBaseURL.value.trim() || location.origin;
  cfg.stream  = $fStream.checked;
  $settings.classList.remove('open');
});
// close popover on outside click
document.addEventListener('click', e => {
  if (!$settings.contains(e.target) && e.target.id !== 'btn-settings') {
    $settings.classList.remove('open');
  }
});
// light/dark toggle
document.getElementById('btn-theme').addEventListener('click', () => {
  document.body.classList.toggle('light');
  const light = document.body.classList.contains('light');
  localStorage.setItem('ag_theme', light ? 'light' : 'dark');
  document.getElementById('btn-theme').textContent = light ? '🌙' : '☀';
});
if (localStorage.getItem('ag_theme') === 'light') {
  document.body.classList.add('light');
  document.getElementById('btn-theme').textContent = '🌙';
}

// mobile sidebar toggle
document.getElementById('sidebar-toggle').addEventListener('click', () => {
  $sidebar.classList.toggle('open');
});
// close sidebar on session click on mobile
$sessionList.addEventListener('click', () => {
  if (window.innerWidth <= 768) $sidebar.classList.remove('open');
});

// ---- init ----
(async function init() {
  await fetchSessions();
  $input.focus();
})();
