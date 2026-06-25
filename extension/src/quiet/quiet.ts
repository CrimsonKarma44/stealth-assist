type Turn = { role: string; content: string };

// Lightweight markdown renderer — avoids a shared Rollup chunk with ui.ts
// (content scripts can't load ES module imports, so marked must stay in ui.js only)
function renderMarkdown(raw: string): string {
  const esc = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

  // Fenced code blocks
  raw = raw.replace(/```[^\n]*\n([\s\S]*?)```/g, (_, code) =>
    `<pre><code>${esc(code.trimEnd())}</code></pre>`);

  // Inline code
  raw = raw.replace(/`([^`\n]+)`/g, (_, code) => `<code>${esc(code)}</code>`);

  // Headers
  raw = raw.replace(/^### (.+)$/gm, '<h3>$1</h3>');
  raw = raw.replace(/^## (.+)$/gm, '<h2>$1</h2>');
  raw = raw.replace(/^# (.+)$/gm, '<h1>$1</h1>');

  // Horizontal rule
  raw = raw.replace(/^---+$/gm, '<hr>');

  // Bold / italic
  raw = raw.replace(/\*\*([^*\n]+)\*\*/g, '<strong>$1</strong>');
  raw = raw.replace(/\*([^*\n]+)\*/g, '<em>$1</em>');

  // Unordered lists
  raw = raw.replace(/((?:^- .+\n?)+)/gm, (block) => {
    const items = block.trim().split('\n')
      .map(l => `<li>${l.replace(/^- /, '')}</li>`).join('');
    return `<ul>${items}</ul>`;
  });

  // Ordered lists
  raw = raw.replace(/((?:^\d+\. .+\n?)+)/gm, (block) => {
    const items = block.trim().split('\n')
      .map(l => `<li>${l.replace(/^\d+\. /, '')}</li>`).join('');
    return `<ol>${items}</ol>`;
  });

  // Paragraphs
  raw = raw.split(/\n\n+/).map(para => {
    para = para.trim();
    if (!para) return '';
    if (/^<(h[1-6]|pre|ul|ol|hr)/.test(para)) return para;
    return `<p>${para.replace(/\n/g, '<br>')}</p>`;
  }).join('\n');

  return raw;
}

const chatEl    = document.getElementById('chat')!;
const inputEl   = document.getElementById('input') as HTMLTextAreaElement;
const sendBtn   = document.getElementById('send-btn') as HTMLButtonElement;
const copyBtn   = document.getElementById('copy-btn') as HTMLButtonElement;
const clearBtn  = document.getElementById('clear-btn') as HTMLButtonElement;
const snapBtn   = document.getElementById('snap-btn') as HTMLButtonElement;
const statusBar = document.getElementById('status-bar')!;
const settingsBtn = document.getElementById('settings-btn') as HTMLButtonElement;

let lastRawReply = '';

function appendMessage(role: 'user' | 'assistant', text: string) {
  const msg = document.createElement('div');
  msg.className = 'msg';

  const label = document.createElement('div');
  label.className = `msg-label ${role === 'user' ? 'user' : 'asst'}`;
  label.textContent = role === 'user' ? 'You' : 'Claude';

  const body = document.createElement('div');
  body.className = `msg-body ${role === 'user' ? 'user' : ''}`;

  if (role === 'assistant') {
    body.className += ' md';
    body.innerHTML = renderMarkdown(text);
    lastRawReply = text;
  } else {
    body.textContent = text;
  }

  msg.appendChild(label);
  msg.appendChild(body);
  chatEl.appendChild(msg);
  chatEl.scrollTop = chatEl.scrollHeight;
}

function setStatus(text: string, color = '#6b7280') {
  statusBar.textContent = text;
  statusBar.style.color = color;
}

async function submit() {
  const prompt = inputEl.value.trim();
  if (!prompt || sendBtn.disabled) return;

  appendMessage('user', prompt);
  inputEl.value = '';
  inputEl.style.height = 'auto';
  sendBtn.disabled = true;
  setStatus('Claude is thinking…');

  try {
    const res = await chrome.runtime.sendMessage({ type: 'ASK_LLM', payload: prompt });
    setStatus('');
    if (res?.reply) {
      appendMessage('assistant', res.reply);
    } else {
      const errText = res?.error === 'not_configured'
        ? 'No API key — click ⚙ to configure.'
        : 'Error: ' + (res?.error ?? 'unknown');
      setStatus(errText, '#f87171');
    }
  } catch (err: any) {
    setStatus('Connection error: ' + err.message, '#f87171');
  }

  sendBtn.disabled = false;
  inputEl.focus();
}

async function takeScreenshot() {
  setStatus('Capturing screen…');
  sendBtn.disabled = true;

  try {
    const res = await chrome.runtime.sendMessage({ type: 'SCREENSHOT_ASK' });
    setStatus('');
    if (res?.reply) {
      appendMessage('assistant', res.reply);
    } else {
      const errText = res?.error === 'not_configured'
        ? 'No API key — click ⚙ to configure.'
        : 'Snap error: ' + (res?.error ?? 'unknown');
      setStatus(errText, '#f87171');
    }
  } catch (err: any) {
    setStatus('Snap error: ' + err.message, '#f87171');
  }

  sendBtn.disabled = false;
}

// ── Load existing history on open ─────────────────────────────────────────
chrome.storage.local.get('history', (items) => {
  const history = (items.history as Turn[]) || [];
  for (const turn of history) {
    if (turn.role === 'user' || turn.role === 'assistant') {
      appendMessage(turn.role as 'user' | 'assistant', turn.content);
    }
  }
});

// ── Wire up handlers ──────────────────────────────────────────────────────
inputEl.addEventListener('input', () => {
  inputEl.style.height = 'auto';
  inputEl.style.height = Math.min(inputEl.scrollHeight, 100) + 'px';
});

inputEl.addEventListener('keydown', (e: KeyboardEvent) => {
  if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); submit(); }
});

sendBtn.onclick = submit;

copyBtn.onclick = () => {
  if (!lastRawReply) return;
  navigator.clipboard.writeText(lastRawReply).then(() => {
    copyBtn.textContent = 'Copied!';
    copyBtn.style.color = '#4ade80';
    setTimeout(() => { copyBtn.textContent = 'Copy'; copyBtn.style.color = ''; }, 1500);
  });
};

clearBtn.onclick = () => {
  chatEl.innerHTML = '';
  lastRawReply = '';
  setStatus('');
  chrome.runtime.sendMessage({ type: 'CLEAR_HISTORY' }).catch(() => {});
};

snapBtn.onclick = () => takeScreenshot();

settingsBtn.onclick = () => {
  chrome.runtime.sendMessage({ type: 'OPEN_OPTIONS' }).catch(() => {});
};
