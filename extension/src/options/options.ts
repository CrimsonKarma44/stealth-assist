const MODELS: Record<string, { label: string; value: string }[]> = {
  google: [
    { label: 'Gemini 3.6 Flash (recommended)', value: 'gemini-3.6-flash' },
    { label: 'Gemini 3.5 Flash',               value: 'gemini-3.5-flash' },
    { label: 'Gemini 3.5 Flash-Lite',          value: 'gemini-3.5-flash-lite' },
    { label: 'Gemini 3.1 Flash-Lite (lightest)', value: 'gemini-3.1-flash-lite' },
    { label: 'Gemini 2.5 Flash',               value: 'gemini-2.5-flash' },
    { label: 'Gemini 2.5 Pro',                 value: 'gemini-2.5-pro' },
  ],
  anthropic: [
    { label: 'Claude Opus 4.8',    value: 'claude-opus-4-8' },
    { label: 'Claude Sonnet 4.6',  value: 'claude-sonnet-4-6' },
    { label: 'Claude Haiku 4.5',   value: 'claude-haiku-4-5-20251001' },
  ],
  openai: [
    { label: 'GPT-4o',            value: 'gpt-4o' },
    { label: 'GPT-4o Mini',       value: 'gpt-4o-mini' },
    { label: 'o4-mini',           value: 'o4-mini' },
  ],
};

// Retired Gemini model IDs still present in some users' chrome.storage.
const GEMINI_MODEL_ALIASES: Record<string, string> = {
  'gemini-2.0-flash':      'gemini-3.6-flash',
  'gemini-2.0-flash-lite': 'gemini-3.6-flash',
  'gemini-1.5-flash':      'gemini-3.6-flash',
  'gemini-1.5-flash-8b':   'gemini-3.1-flash-lite',
  'gemini-1.5-pro':        'gemini-2.5-pro',
};

function migrateGeminiModel(model: string): string {
  return GEMINI_MODEL_ALIASES[model] ?? model;
}

const HINTS: Record<string, string> = {
  google:    'Get a free key at <a href="https://aistudio.google.com/app/apikey" target="_blank">aistudio.google.com</a> — no credit card required.',
  anthropic: 'Get a key at <a href="https://console.anthropic.com/" target="_blank">console.anthropic.com</a>.',
  openai:    'Get a key at <a href="https://platform.openai.com/api-keys" target="_blank">platform.openai.com</a>.',
};

const serverUrlEl       = document.getElementById('serverUrl')         as HTMLInputElement;
const providerEl        = document.getElementById('provider')          as HTMLSelectElement;
const modelEl           = document.getElementById('model')             as HTMLSelectElement;
const apiKeyEl          = document.getElementById('apiKey')            as HTMLInputElement;
const toggleBtn         = document.getElementById('toggleKey')         as HTMLButtonElement;
const screenshotShortEl = document.getElementById('screenshotShortcut') as HTMLInputElement;
const saveBtn           = document.getElementById('saveBtn')           as HTMLButtonElement;
const testBtn           = document.getElementById('testBtn')           as HTMLButtonElement;
const statusEl          = document.getElementById('status')            as HTMLParagraphElement;
const keyHint           = document.getElementById('keyHint')           as HTMLParagraphElement;

function populateModels(provider: string, selectedModel?: string) {
  modelEl.innerHTML = '';
  for (const m of MODELS[provider] ?? []) {
    const opt = document.createElement('option');
    opt.value = m.value;
    opt.textContent = m.label;
    if (m.value === selectedModel) opt.selected = true;
    modelEl.appendChild(opt);
  }
}

function updateHint(provider: string) {
  keyHint.innerHTML = HINTS[provider] ?? '';
}

providerEl.addEventListener('change', () => {
  populateModels(providerEl.value);
  updateHint(providerEl.value);
  statusEl.textContent = '';
});

// ── Screenshot shortcut recorder ───────────────────────────────────────────
screenshotShortEl.addEventListener('keydown', (e: KeyboardEvent) => {
  e.preventDefault();
  const parts: string[] = [];
  if (e.ctrlKey)  parts.push('Ctrl');
  if (e.altKey)   parts.push('Alt');
  if (e.shiftKey) parts.push('Shift');
  if (e.metaKey)  parts.push('Meta');
  const key = e.key;
  if (!['Control', 'Alt', 'Shift', 'Meta'].includes(key)) parts.push(key.toUpperCase());
  if (parts.length > 1) screenshotShortEl.value = parts.join('+');
});

toggleBtn.addEventListener('click', () => {
  const show = apiKeyEl.type === 'password';
  apiKeyEl.type = show ? 'text' : 'password';
  toggleBtn.textContent = show ? 'Hide' : 'Show';
});

saveBtn.addEventListener('click', () => {
  const serverUrl         = serverUrlEl.value.trim().replace(/\/$/, '') || 'https://stealth-assist.onrender.com';
  const provider          = providerEl.value;
  const model             = modelEl.value;
  const apiKey            = apiKeyEl.value.trim();
  const screenshotShortcut = screenshotShortEl.value.trim() || 'Alt+Shift+Z';

  if (!apiKey) {
    setStatus('err', 'Please enter an API key.');
    return;
  }

  chrome.storage.local.set({ serverUrl, provider, model, apiKey, screenshotShortcut, configured: true }, () => {
    setStatus('ok', 'Saved.');
  });
});

testBtn.addEventListener('click', () => {
  const serverUrl = serverUrlEl.value.trim().replace(/\/$/, '') || 'https://stealth-assist.onrender.com';
  const provider  = providerEl.value;
  const model     = modelEl.value;
  const apiKey    = apiKeyEl.value.trim();

  if (!apiKey) {
    setStatus('err', 'Enter an API key first.');
    return;
  }

  setStatus('', 'Testing…');
  testBtn.disabled = true;

  chrome.runtime.sendMessage({ type: 'TEST_CONNECTION', serverUrl, provider, model, apiKey }, (res) => {
    testBtn.disabled = false;
    if (res?.ok) {
      setStatus('ok', 'Connected ✓');
    } else {
      setStatus('err', 'Failed: ' + (res?.error ?? 'unknown error'));
    }
  });
});

function setStatus(cls: 'ok' | 'err' | '', text: string) {
  statusEl.className = cls;
  statusEl.textContent = text;
}

// Load saved settings on open
chrome.storage.local.get(['serverUrl', 'provider', 'model', 'apiKey', 'screenshotShortcut'], (items) => {
  const serverUrl         = (items.serverUrl         as string) || '';
  const provider          = (items.provider          as string) || 'google';
  const rawModel          = (items.model             as string) || '';
  const model             = provider === 'google' ? migrateGeminiModel(rawModel) : rawModel;
  const apiKey            = (items.apiKey            as string) || '';
  const screenshotShortcut = (items.screenshotShortcut as string) || 'Alt+Shift+Z';

  // Persist migration so chat/screenshot use the new model without re-saving
  if (model && model !== rawModel) {
    chrome.storage.local.set({ model });
  }

  serverUrlEl.value        = serverUrl;
  providerEl.value         = provider;
  populateModels(provider, model);
  updateHint(provider);
  apiKeyEl.value           = apiKey;
  screenshotShortEl.value  = screenshotShortcut;
});
