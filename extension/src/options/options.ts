const MODELS: Record<string, { label: string; value: string }[]> = {
  google: [
    { label: 'Gemini 2.0 Flash (recommended)', value: 'gemini-2.0-flash' },
    { label: 'Gemini 1.5 Flash',               value: 'gemini-1.5-flash' },
    { label: 'Gemini 1.5 Flash 8B (lightest)',  value: 'gemini-1.5-flash-8b' },
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

const HINTS: Record<string, string> = {
  google:    'Get a free key at <a href="https://aistudio.google.com/app/apikey" target="_blank">aistudio.google.com</a> — no credit card required.',
  anthropic: 'Get a key at <a href="https://console.anthropic.com/" target="_blank">console.anthropic.com</a>.',
  openai:    'Get a key at <a href="https://platform.openai.com/api-keys" target="_blank">platform.openai.com</a>.',
};

const providerEl = document.getElementById('provider') as HTMLSelectElement;
const modelEl    = document.getElementById('model')    as HTMLSelectElement;
const apiKeyEl   = document.getElementById('apiKey')   as HTMLInputElement;
const toggleBtn  = document.getElementById('toggleKey') as HTMLButtonElement;
const saveBtn    = document.getElementById('saveBtn')   as HTMLButtonElement;
const testBtn    = document.getElementById('testBtn')   as HTMLButtonElement;
const statusEl   = document.getElementById('status')   as HTMLParagraphElement;
const keyHint    = document.getElementById('keyHint')  as HTMLParagraphElement;

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

toggleBtn.addEventListener('click', () => {
  const show = apiKeyEl.type === 'password';
  apiKeyEl.type = show ? 'text' : 'password';
  toggleBtn.textContent = show ? 'Hide' : 'Show';
});

saveBtn.addEventListener('click', () => {
  const provider = providerEl.value;
  const model    = modelEl.value;
  const apiKey   = apiKeyEl.value.trim();

  if (!apiKey) {
    setStatus('err', 'Please enter an API key.');
    return;
  }

  chrome.storage.local.set({ provider, model, apiKey, configured: true }, () => {
    setStatus('ok', 'Saved.');
  });
});

testBtn.addEventListener('click', () => {
  const provider = providerEl.value;
  const model    = modelEl.value;
  const apiKey   = apiKeyEl.value.trim();

  if (!apiKey) {
    setStatus('err', 'Enter an API key first.');
    return;
  }

  setStatus('', 'Testing…');
  testBtn.disabled = true;

  chrome.runtime.sendMessage({ type: 'TEST_CONNECTION', provider, model, apiKey }, (res) => {
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
chrome.storage.local.get(['provider', 'model', 'apiKey'], (items) => {
  const provider = (items.provider as string) || 'google';
  const model    = (items.model    as string) || '';
  const apiKey   = (items.apiKey   as string) || '';

  providerEl.value = provider;
  populateModels(provider, model);
  updateHint(provider);
  apiKeyEl.value = apiKey;
});
