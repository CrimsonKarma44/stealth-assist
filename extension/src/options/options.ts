const CUSTOM_MODEL_VALUE = '__custom__';

const MODELS: Record<string, { label: string; value: string }[]> = {
  openrouter: [
    { label: 'Auto free router (recommended)',          value: 'openrouter/free' },
    { label: 'NVIDIA Nemotron 3.5 Lightning (free)',     value: 'nvidia/nemotron-3.5-lightning:free' },
    { label: 'Poolside Laguna S 2.1 (free)',             value: 'poolside/laguna-s-2.1:free' },
    { label: 'Liquid LFM 2.5 (free)',                    value: 'liquid/lfm-2.5-2.6b:free' },
    { label: 'Google Gemini 3.6 Flash (paid, vision)',   value: 'google/gemini-3.6-flash' },
    { label: 'xAI Grok 4.6 (paid, vision)',              value: 'x-ai/grok-4.6' },
    { label: 'Custom…',                                 value: CUSTOM_MODEL_VALUE },
  ],
  google: [
    { label: 'Gemini 3.6 Flash (recommended)', value: 'gemini-3.6-flash' },
    { label: 'Gemini 3.5 Flash',               value: 'gemini-3.5-flash' },
    { label: 'Gemini 3.5 Flash-Lite',          value: 'gemini-3.5-flash-lite' },
    { label: 'Gemini 3.1 Flash-Lite (lightest)', value: 'gemini-3.1-flash-lite' },
    { label: 'Gemini 2.5 Flash',               value: 'gemini-2.5-flash' },
    { label: 'Gemini 2.5 Pro',                 value: 'gemini-2.5-pro' },
  ],
  xai: [
    { label: 'Grok 4.5 (recommended)',              value: 'grok-4.5' },
    { label: 'Grok 4.3',                             value: 'grok-4.3' },
    { label: 'Grok 4.20 (non-reasoning)',            value: 'grok-4.20-0309-non-reasoning' },
    { label: 'Grok 4.20 (reasoning)',                value: 'grok-4.20-0309-reasoning' },
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
  openrouter: 'Get a free key at <a href="https://openrouter.ai/keys" target="_blank">openrouter.ai/keys</a> — free models available; paid credits unlock stronger models.',
  google:     'Get a free key at <a href="https://aistudio.google.com/app/apikey" target="_blank">aistudio.google.com</a> — no credit card required.',
  xai:        'Get a key at <a href="https://console.x.ai/" target="_blank">console.x.ai</a>.',
  anthropic:  'Get a key at <a href="https://console.anthropic.com/" target="_blank">console.anthropic.com</a>.',
  openai:     'Get a key at <a href="https://platform.openai.com/api-keys" target="_blank">platform.openai.com</a>.',
};

const serverUrlEl       = document.getElementById('serverUrl')         as HTMLInputElement;
const providerEl        = document.getElementById('provider')          as HTMLSelectElement;
const modelEl           = document.getElementById('model')             as HTMLSelectElement;
const customModelField  = document.getElementById('customModelField')  as HTMLDivElement;
const customModelEl     = document.getElementById('customModel')       as HTMLInputElement;
const apiKeyEl          = document.getElementById('apiKey')            as HTMLInputElement;
const toggleBtn         = document.getElementById('toggleKey')         as HTMLButtonElement;
const screenshotShortEl = document.getElementById('screenshotShortcut') as HTMLInputElement;
const saveBtn           = document.getElementById('saveBtn')           as HTMLButtonElement;
const testBtn           = document.getElementById('testBtn')           as HTMLButtonElement;
const statusEl          = document.getElementById('status')            as HTMLParagraphElement;
const keyHint           = document.getElementById('keyHint')           as HTMLParagraphElement;

function curatedValues(provider: string): Set<string> {
  return new Set((MODELS[provider] ?? []).map(m => m.value).filter(v => v !== CUSTOM_MODEL_VALUE));
}

function syncCustomModelVisibility() {
  const show = providerEl.value === 'openrouter' && modelEl.value === CUSTOM_MODEL_VALUE;
  customModelField.style.display = show ? '' : 'none';
}

function resolveModelForRequest(): string {
  if (providerEl.value === 'openrouter' && modelEl.value === CUSTOM_MODEL_VALUE) {
    return customModelEl.value.trim();
  }
  return modelEl.value;
}

function populateModels(provider: string, selectedModel?: string) {
  modelEl.innerHTML = '';
  const list = MODELS[provider] ?? [];
  const known = curatedValues(provider);
  const useCustom = provider === 'openrouter'
    && !!selectedModel
    && !known.has(selectedModel);

  for (const m of list) {
    const opt = document.createElement('option');
    opt.value = m.value;
    opt.textContent = m.label;
    if (useCustom) {
      if (m.value === CUSTOM_MODEL_VALUE) opt.selected = true;
    } else if (m.value === selectedModel) {
      opt.selected = true;
    }
    modelEl.appendChild(opt);
  }

  if (useCustom && selectedModel) {
    customModelEl.value = selectedModel;
  } else if (provider !== 'openrouter') {
    customModelEl.value = '';
  }

  syncCustomModelVisibility();
}

function updateHint(provider: string) {
  keyHint.innerHTML = HINTS[provider] ?? '';
}

providerEl.addEventListener('change', () => {
  populateModels(providerEl.value);
  updateHint(providerEl.value);
  statusEl.textContent = '';
});

modelEl.addEventListener('change', () => {
  syncCustomModelVisibility();
  if (modelEl.value === CUSTOM_MODEL_VALUE) customModelEl.focus();
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
  const model             = resolveModelForRequest();
  const apiKey            = apiKeyEl.value.trim();
  const screenshotShortcut = screenshotShortEl.value.trim() || 'Alt+Shift+Z';

  if (!apiKey) {
    setStatus('err', 'Please enter an API key.');
    return;
  }
  if (!model) {
    setStatus('err', provider === 'openrouter'
      ? 'Enter a custom OpenRouter model ID, or pick one from the list.'
      : 'Please select a model.');
    return;
  }

  chrome.storage.local.set({ serverUrl, provider, model, apiKey, screenshotShortcut, configured: true }, () => {
    setStatus('ok', 'Saved.');
  });
});

testBtn.addEventListener('click', () => {
  const serverUrl = serverUrlEl.value.trim().replace(/\/$/, '') || 'https://stealth-assist.onrender.com';
  const provider  = providerEl.value;
  const model     = resolveModelForRequest();
  const apiKey    = apiKeyEl.value.trim();

  if (!apiKey) {
    setStatus('err', 'Enter an API key first.');
    return;
  }
  if (!model) {
    setStatus('err', 'Enter or select a model first.');
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
