// Background service worker — relays messages to the local Go backend.
// Conversation history is persisted to chrome.storage.local so it survives
// service worker suspension between messages.

// ── First install: open settings page ─────────────────────────────────────
chrome.runtime.onInstalled.addListener((details) => {
  if (details.reason === 'install') {
    chrome.runtime.openOptionsPage();
  }
});

// ── Settings helpers ───────────────────────────────────────────────────────
type Settings = { provider: string; model: string; apiKey: string; serverUrl: string; configured: boolean };
type Turn = { role: string; content: string };

const DEFAULT_SERVER = 'https://stealth-assist.onrender.com';

function getSettings(): Promise<Settings> {
  return new Promise((resolve) => {
    chrome.storage.local.get(['provider', 'model', 'apiKey', 'serverUrl', 'configured'], (items) => {
      resolve({
        provider:   (items.provider   as string)  || '',
        model:      (items.model      as string)  || '',
        apiKey:     (items.apiKey     as string)  || '',
        serverUrl:  (items.serverUrl  as string)  || DEFAULT_SERVER,
        configured: (items.configured as boolean) || false,
      });
    });
  });
}

// ── History helpers (storage-backed) ──────────────────────────────────────
function loadHistory(): Promise<Turn[]> {
  return new Promise((resolve) => {
    chrome.storage.local.get('history', (items) => {
      resolve((items.history as Turn[]) || []);
    });
  });
}

function saveHistory(history: Turn[]): void {
  chrome.storage.local.set({ history });
}

// ── Message handler ────────────────────────────────────────────────────────
chrome.runtime.onMessage.addListener((request, _sender, sendResponse) => {

  // Open settings page
  if (request.type === 'OPEN_OPTIONS') {
    chrome.runtime.openOptionsPage();
    sendResponse({ ok: true });
    return true;
  }

  // Text chat
  if (request.type === 'ASK_LLM') {
    Promise.all([getSettings(), loadHistory()]).then(([settings, history]) => {
      if (!settings.configured) {
        sendResponse({ error: 'not_configured' });
        return;
      }

      history.push({ role: 'user', content: request.payload });

      fetch(`${settings.serverUrl}/api/ask`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          messages: history,
          provider: settings.provider,
          model:    settings.model,
          apiKey:   settings.apiKey,
        }),
      })
        .then(res => res.json())
        .then(data => {
          if (data.reply) history.push({ role: 'assistant', content: data.reply });
          saveHistory(history);
          sendResponse({ reply: data.reply });
        })
        .catch(err => {
          history.pop();
          saveHistory(history);
          sendResponse({ error: err.toString() });
        });
    });

    return true;
  }

  // Screenshot via Snap button
  if (request.type === 'SCREENSHOT_ASK') {
    getSettings().then((settings) => {
      if (!settings.configured) {
        sendResponse({ error: 'not_configured' });
        return;
      }

      chrome.tabs.captureVisibleTab({ format: 'png' })
        .then(dataUrl => {
          const base64 = dataUrl.replace(/^data:image\/png;base64,/, '');
          return fetch(`${settings.serverUrl}/api/screenshot`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              image:    base64,
              provider: settings.provider,
              model:    settings.model,
              apiKey:   settings.apiKey,
            }),
          });
        })
        .then(res => res.json())
        .then(data => {
          if (data.reply) {
            loadHistory().then(history => {
              history.push({ role: 'user',      content: '[Screenshot] Answer all questions visible on this screen.' });
              history.push({ role: 'assistant', content: data.reply });
              saveHistory(history);
            });
          }
          sendResponse({ reply: data.reply });
        })
        .catch(err => sendResponse({ error: err.toString() }));
    });

    return true;
  }

  // Test connection from settings page
  if (request.type === 'TEST_CONNECTION') {
    fetch(`${request.serverUrl || DEFAULT_SERVER}/api/ask`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        messages: [{ role: 'user', content: 'Reply with exactly the word "ok".' }],
        provider: request.provider,
        model:    request.model,
        apiKey:   request.apiKey,
      }),
    })
      .then(res => {
        if (!res.ok) throw new Error(`Server returned ${res.status}`);
        return res.json();
      })
      .then(data => sendResponse({ ok: true, reply: data.reply }))
      .catch(err => sendResponse({ ok: false, error: err.toString() }));

    return true;
  }

  // Clear conversation history
  if (request.type === 'CLEAR_HISTORY') {
    chrome.storage.local.remove('history', () => sendResponse({ ok: true }));
    return true;
  }
});
