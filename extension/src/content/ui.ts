import { marked } from 'marked';

// Module-level references — no IDs on DOM elements to avoid page-script detection
let overlay: HTMLDivElement | null = null;
let bodyEl: HTMLDivElement | null = null;
let chatEl: HTMLDivElement | null = null;
let inputEl: HTMLTextAreaElement | null = null;
let sendBtn: HTMLButtonElement | null = null;
let minBtn: HTMLButtonElement | null = null;
let copyBtn: HTMLButtonElement | null = null;
let minimized = false;
let lastRawReply = '';

// Drag state
let isDragging = false;
let dragOffsetX = 0;
let dragOffsetY = 0;

function onDragMove(e: MouseEvent) {
  if (!isDragging || !overlay) return;
  overlay.style.left = (e.clientX - dragOffsetX) + 'px';
  overlay.style.top  = (e.clientY - dragOffsetY) + 'px';
}

function onDragEnd() {
  isDragging = false;
  if (overlay) overlay.style.userSelect = '';
  document.removeEventListener('mousemove', onDragMove);
  document.removeEventListener('mouseup', onDragEnd);
}

function appendMessage(role: 'user' | 'assistant', text: string) {
  const msg = document.createElement('div');
  msg.style.cssText = `margin-bottom: 16px; ${role === 'user' ? 'opacity:.72;' : ''}`;

  const label = document.createElement('div');
  label.style.cssText = `
    font-size: 10px; font-weight: 700; letter-spacing: .8px;
    text-transform: uppercase; margin-bottom: 4px;
    color: ${role === 'user' ? '#6b7280' : '#60a5fa'};
    pointer-events: none;
  `;
  label.textContent = role === 'user' ? 'You' : 'Claude';

  const content = document.createElement('div');
  content.style.cssText = 'font-size: 13.5px; line-height: 1.65;';

  if (role === 'assistant') {
    content.className = '__md';
    content.innerHTML = marked.parse(text) as string;
    lastRawReply = text;
  } else {
    content.textContent = text;
    content.style.color = '#9ca3af';
  }

  msg.appendChild(label);
  msg.appendChild(content);
  chatEl!.appendChild(msg);
  chatEl!.scrollTop = chatEl!.scrollHeight;
}

function setMinimized(val: boolean) {
  minimized = val;
  bodyEl!.style.display = val ? 'none' : 'flex';
  minBtn!.textContent = val ? '+' : '−';
  overlay!.style.maxHeight = val ? '' : '72vh';
}

function buildOverlay() {
  overlay = document.createElement('div');
  overlay.style.cssText = `
    position: fixed; bottom: 20px; right: 20px;
    width: 440px; max-height: 72vh;
    display: flex; flex-direction: column;
    background: rgba(13,13,18,.97); color: #e2e8f0;
    border: 1px solid #2d2d3a; border-radius: 12px;
    z-index: 2147483647;
    font-family: -apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;
    box-shadow: 0 16px 48px rgba(0,0,0,.65), 0 0 0 1px rgba(255,255,255,.04);
    overflow: hidden;
  `;

  // Scoped styles — class name is opaque to avoid obvious fingerprinting
  const style = document.createElement('style');
  style.textContent = `
    .__sc::-webkit-scrollbar{width:3px}
    .__sc::-webkit-scrollbar-track{background:transparent}
    .__sc::-webkit-scrollbar-thumb{background:#3a3a4a;border-radius:2px}
    .__md h1,.__md h2,.__md h3{color:#f1f5f9;margin:10px 0 5px;font-size:14px}
    .__md p{margin:5px 0}
    .__md code{background:#1e1e2e;color:#c4b5fd;padding:2px 5px;border-radius:4px;font-size:12px;font-family:'Fira Code',monospace}
    .__md pre{background:#1e1e2e;border-radius:6px;padding:10px 12px;overflow-x:auto;margin:8px 0}
    .__md pre code{background:none;padding:0;color:#e2e8f0;font-size:12px}
    .__md ul,.__md ol{padding-left:18px;margin:5px 0}
    .__md li{margin:3px 0}
    .__md strong{color:#f1f5f9}
    .__md em{color:#a5b4fc}
    .__md blockquote{border-left:2px solid #374151;margin:8px 0;padding-left:10px;color:#6b7280}
    .__md a{color:#60a5fa;text-decoration:none}
    .__md hr{border:none;border-top:1px solid #2d2d3a;margin:10px 0}
    .__ta::placeholder{color:#4a4a5a}
    .__ta{scrollbar-width:none}
    .__ta::-webkit-scrollbar{display:none}
  `;

  // ── Header / drag handle ─────────────────────────────────────────────────
  const header = document.createElement('div');
  header.style.cssText = `
    display:flex; align-items:center; justify-content:space-between;
    padding: 10px 12px; border-bottom: 1px solid #2d2d3a;
    flex-shrink: 0; cursor: grab; user-select: none;
  `;

  header.addEventListener('mousedown', (e: MouseEvent) => {
    if ((e.target as HTMLElement).tagName === 'BUTTON') return;
    isDragging = true;
    const rect = overlay!.getBoundingClientRect();
    overlay!.style.bottom = 'auto';
    overlay!.style.right  = 'auto';
    overlay!.style.top    = rect.top + 'px';
    overlay!.style.left   = rect.left + 'px';
    dragOffsetX = e.clientX - rect.left;
    dragOffsetY = e.clientY - rect.top;
    overlay!.style.userSelect = 'none';
    header.style.cursor = 'grabbing';
    document.addEventListener('mousemove', onDragMove);
    document.addEventListener('mouseup', onDragEnd);
  });

  document.addEventListener('mouseup', () => {
    if (isDragging) header.style.cursor = 'grab';
  });

  const titleEl = document.createElement('span');
  titleEl.textContent = 'Stealth Assist';
  titleEl.style.cssText = 'font-size:11px;font-weight:600;color:#6b7280;letter-spacing:.6px;pointer-events:none;';

  const headerBtns = document.createElement('div');
  headerBtns.style.cssText = 'display:flex;gap:6px;align-items:center;';

  const settingsBtn = document.createElement('button');
  settingsBtn.textContent = '⚙';
  settingsBtn.title = 'Settings';
  settingsBtn.style.cssText = 'background:none;border:none;color:#555;cursor:pointer;font-size:13px;padding:0 3px;line-height:1;';

  minBtn = document.createElement('button');
  minBtn.textContent = '−';
  minBtn.title = 'Minimize';
  minBtn.style.cssText = 'background:none;border:none;color:#555;cursor:pointer;font-size:16px;padding:0 3px;line-height:1;';

  const closeBtn = document.createElement('button');
  closeBtn.textContent = '✕';
  closeBtn.style.cssText = 'background:none;border:none;color:#555;cursor:pointer;font-size:12px;padding:0 3px;line-height:1;';

  headerBtns.appendChild(settingsBtn);
  headerBtns.appendChild(minBtn);
  headerBtns.appendChild(closeBtn);
  header.appendChild(titleEl);
  header.appendChild(headerBtns);

  // ── Body (hidden when minimized) ─────────────────────────────────────────
  bodyEl = document.createElement('div');
  bodyEl.style.cssText = 'display:flex;flex-direction:column;flex:1;min-height:0;overflow:hidden;';

  // Chat thread
  chatEl = document.createElement('div');
  chatEl.className = '__sc';
  chatEl.style.cssText = 'flex:1;overflow-y:auto;padding:14px 16px 6px;min-height:80px;';

  // Action bar
  const actionBar = document.createElement('div');
  actionBar.style.cssText = 'display:flex;gap:8px;padding:4px 16px 8px;flex-shrink:0;';

  copyBtn = document.createElement('button');
  copyBtn.textContent = 'Copy';
  copyBtn.style.cssText = `
    background:none;border:1px solid #2d2d3a;border-radius:5px;
    color:#6b7280;cursor:pointer;font-size:11px;padding:3px 10px;
  `;

  const clearBtn = document.createElement('button');
  clearBtn.textContent = 'Clear';
  clearBtn.style.cssText = copyBtn.style.cssText;

  const snapBtn = document.createElement('button');
  snapBtn.textContent = 'Snap';
  snapBtn.title = 'Capture screen and ask AI';
  snapBtn.style.cssText = `
    background:none;border:1px solid #2d2d3a;border-radius:5px;
    color:#60a5fa;cursor:pointer;font-size:11px;padding:3px 10px;
  `;

  actionBar.appendChild(copyBtn);
  actionBar.appendChild(clearBtn);
  actionBar.appendChild(snapBtn);

  // Input row
  const inputRow = document.createElement('div');
  inputRow.style.cssText = `
    display:flex;gap:8px;align-items:flex-end;
    padding:8px 12px 12px;border-top:1px solid #2d2d3a;flex-shrink:0;
  `;

  inputEl = document.createElement('textarea') as HTMLTextAreaElement;
  inputEl.className = '__ta';
  inputEl.rows = 1;
  inputEl.placeholder = 'Ask anything… (Enter sends, Shift+Enter = newline)';
  inputEl.style.cssText = `
    background:#1a1a24;border:1px solid #2d2d3a;border-radius:7px;
    color:#e2e8f0;font-family:inherit;font-size:13px;
    resize:none;flex:1;line-height:1.5;max-height:100px;
    padding:7px 10px;outline:none;overflow-y:auto;
  `;

  inputEl.addEventListener('input', () => {
    inputEl!.style.height = 'auto';
    inputEl!.style.height = Math.min(inputEl!.scrollHeight, 100) + 'px';
  });

  sendBtn = document.createElement('button');
  sendBtn.textContent = 'Send';
  sendBtn.style.cssText = `
    background:#2563eb;border:none;border-radius:7px;color:#fff;
    cursor:pointer;font-size:12px;font-weight:600;
    padding:7px 14px;align-self:flex-end;flex-shrink:0;transition:opacity .15s;
  `;

  inputRow.appendChild(inputEl);
  inputRow.appendChild(sendBtn);

  bodyEl.appendChild(chatEl);
  bodyEl.appendChild(actionBar);
  bodyEl.appendChild(inputRow);

  overlay.appendChild(style);
  overlay.appendChild(header);
  overlay.appendChild(bodyEl);
  document.body.appendChild(overlay);

  // ── Handlers ──────────────────────────────────────────────────────────────

  settingsBtn.onclick = () => {
    chrome.runtime.sendMessage({ type: 'OPEN_OPTIONS' }).catch(() => {});
  };

  closeBtn.onclick = () => {
    overlay?.remove();
    overlay = bodyEl = chatEl = inputEl = sendBtn = minBtn = copyBtn = null;
    minimized = false;
    lastRawReply = '';
  };

  minBtn.onclick = () => setMinimized(!minimized);

  copyBtn.onclick = () => {
    if (!lastRawReply) return;
    navigator.clipboard.writeText(lastRawReply).then(() => {
      copyBtn!.textContent = 'Copied!';
      copyBtn!.style.color = '#4ade80';
      setTimeout(() => {
        if (copyBtn) { copyBtn.textContent = 'Copy'; copyBtn.style.color = '#6b7280'; }
      }, 1500);
    });
  };

  clearBtn.onclick = () => {
    if (!chatEl) return;
    chatEl.innerHTML = '';
    lastRawReply = '';
    chrome.runtime.sendMessage({ type: 'CLEAR_HISTORY' }).catch(() => {});
  };

  const submit = async () => {
    const prompt = inputEl!.value.trim();
    if (!prompt || sendBtn!.disabled) return;

    appendMessage('user', prompt);
    inputEl!.value = '';
    inputEl!.style.height = 'auto';
    sendBtn!.disabled = true;
    sendBtn!.style.opacity = '.45';

    const thinking = document.createElement('div');
    thinking.style.cssText = 'color:#4a4a5a;font-style:italic;font-size:13px;margin-bottom:10px;';
    thinking.textContent = 'Claude is thinking…';
    chatEl!.appendChild(thinking);
    chatEl!.scrollTop = chatEl!.scrollHeight;

    try {
      const res = await chrome.runtime.sendMessage({ type: 'ASK_LLM', payload: prompt });
      thinking.remove();
      if (!chatEl) return; // overlay closed while waiting
      if (res?.reply) {
        appendMessage('assistant', res.reply);
      } else {
        const err = document.createElement('div');
        err.style.cssText = 'color:#f87171;font-size:13px;margin-bottom:10px;';
        err.textContent = res?.error === 'not_configured'
          ? 'No API key configured — click ⚙ to set one up.'
          : 'Error: ' + (res?.error ?? 'unknown');
        chatEl.appendChild(err);
        chatEl.scrollTop = chatEl.scrollHeight;
      }
    } catch (err: any) {
      thinking.remove();
      if (!chatEl) return; // overlay closed while waiting
      const errEl = document.createElement('div');
      errEl.style.cssText = 'color:#f87171;font-size:13px;margin-bottom:10px;';
      errEl.textContent = 'Connection error: ' + err.message;
      chatEl.appendChild(errEl);
      chatEl.scrollTop = chatEl.scrollHeight;
    }

    if (sendBtn) { sendBtn.disabled = false; sendBtn.style.opacity = '1'; }
    inputEl?.focus();
  };

  sendBtn.onclick = submit;
  inputEl.addEventListener('keydown', (e: KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); submit(); }
  });

  snapBtn.onclick = () => takeScreenshot();
}

// ── Screenshot capture ─────────────────────────────────────────────────────
async function takeScreenshot() {
  // Hide overlay so it doesn't appear in the capture
  if (overlay) overlay.style.visibility = 'hidden';

  // Two rAF cycles guarantee the browser has repainted before we capture
  await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));

  try {
    const res = await chrome.runtime.sendMessage({ type: 'SCREENSHOT_ASK' });

    // Ensure overlay is built and visible
    if (!overlay) buildOverlay();
    overlay!.style.visibility = 'visible';
    if (minimized) setMinimized(false);

    if (res?.reply) {
      appendMessage('assistant', res.reply);
    } else {
      const errEl = document.createElement('div');
      errEl.style.cssText = 'color:#f87171;font-size:13px;margin-bottom:10px;';
      errEl.textContent = res?.error === 'not_configured'
        ? 'No API key configured — click ⚙ to set one up.'
        : 'Snap error: ' + (res?.error ?? 'unknown');
      chatEl!.appendChild(errEl);
      chatEl!.scrollTop = chatEl!.scrollHeight;
    }
  } catch (err: any) {
    if (!overlay) buildOverlay();
    overlay!.style.visibility = 'visible';
    if (minimized) setMinimized(false);
    const errEl = document.createElement('div');
    errEl.style.cssText = 'color:#f87171;font-size:13px;margin-bottom:10px;';
    errEl.textContent = 'Snap error: ' + err.message;
    if (chatEl) { chatEl.appendChild(errEl); chatEl.scrollTop = chatEl.scrollHeight; }
  }
}

// ── Shortcut helpers ───────────────────────────────────────────────────────
const DEFAULT_SCREENSHOT_SHORTCUT = 'Alt+Shift+Z';

function matchesShortcut(e: KeyboardEvent, shortcut: string): boolean {
  const parts = shortcut.toLowerCase().split('+');
  const key   = parts[parts.length - 1];
  return e.ctrlKey  === parts.includes('ctrl')  &&
         e.shiftKey === parts.includes('shift') &&
         e.altKey   === parts.includes('alt')   &&
         e.metaKey  === parts.includes('meta')  &&
         e.key.toLowerCase() === key;
}

// ── Global shortcuts ───────────────────────────────────────────────────────
let screenshotShortcut = DEFAULT_SCREENSHOT_SHORTCUT;
chrome.storage.local.get('screenshotShortcut', (items) => {
  screenshotShortcut = (items.screenshotShortcut as string) || DEFAULT_SCREENSHOT_SHORTCUT;
});

document.addEventListener('keydown', (e: KeyboardEvent) => {
  // Overlay toggle — Ctrl+Shift+X (fixed)
  if (e.ctrlKey && e.shiftKey && e.key.toLowerCase() === 'x') {
    e.preventDefault();
    const selected = window.getSelection()?.toString().trim() ?? '';
    if (!overlay) {
      buildOverlay();
    } else if (minimized) {
      setMinimized(false);
    }
    if (selected && inputEl) {
      inputEl.value = selected;
      inputEl.style.height = 'auto';
      inputEl.style.height = Math.min(inputEl.scrollHeight, 100) + 'px';
    }
    inputEl?.focus();
    if (inputEl) inputEl.setSelectionRange(inputEl.value.length, inputEl.value.length);
    return;
  }

  // Screenshot — user-configured shortcut
  if (matchesShortcut(e, screenshotShortcut)) {
    e.preventDefault();
    takeScreenshot();
  }
});
