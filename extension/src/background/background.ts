// Background service worker — relays messages to the local Go backend.
// Conversation history is held in memory here; the page never sees it.

const history: Array<{ role: string; content: string }> = [];

// ── First install: open settings page ─────────────────────────────────────
chrome.runtime.onInstalled.addListener((details) => {
  if (details.reason === 'install') {
    chrome.runtime.openOptionsPage();
  }
});

// ── Settings helpers ───────────────────────────────────────────────────────
type Settings = { provider: string; model: string; apiKey: string; configured: boolean };

function getSettings(): Promise<Settings> {
  return new Promise((resolve) => {
    chrome.storage.local.get(['provider', 'model', 'apiKey', 'configured'], (items) => {
      resolve({
        provider:   (items.provider   as string)  || '',
        model:      (items.model      as string)  || '',
        apiKey:     (items.apiKey     as string)  || '',
        configured: (items.configured as boolean) || false,
      });
    });
  });
}

// ── Manifest command: Ctrl+Shift+Z ─────────────────────────────────────────
chrome.commands.onCommand.addListener(async (command) => {
  if (command !== 'screenshot') return;

  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (!tab?.id) return;

  try {
    await chrome.tabs.sendMessage(tab.id, { type: 'SNAP_HIDE' });
  } catch { /* content script may not be injected yet */ }

  let dataUrl: string;
  try {
    dataUrl = await chrome.tabs.captureVisibleTab({ format: 'png' });
  } catch (err) {
    chrome.tabs.sendMessage(tab.id, { type: 'SNAP_RESPONSE', error: String(err) }).catch(() => {});
    return;
  }

  const settings = await getSettings();
  if (!settings.configured) {
    chrome.tabs.sendMessage(tab.id, { type: 'SNAP_RESPONSE', error: 'not_configured' }).catch(() => {});
    return;
  }

  const base64 = dataUrl.replace(/^data:image\/png;base64,/, '');

  try {
    const res = await fetch('http://localhost:8080/api/screenshot', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        image:    base64,
        provider: settings.provider,
        model:    settings.model,
        apiKey:   settings.apiKey,
      }),
    });
    const data = await res.json();
    chrome.tabs.sendMessage(tab.id, { type: 'SNAP_RESPONSE', reply: data.reply }).catch(() => {});
  } catch (err) {
    chrome.tabs.sendMessage(tab.id, { type: 'SNAP_RESPONSE', error: String(err) }).catch(() => {});
  }
});

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
    getSettings().then((settings) => {
      if (!settings.configured) {
        sendResponse({ error: 'not_configured' });
        return;
      }

      history.push({ role: 'user', content: request.payload });

      fetch('http://localhost:8080/api/ask', {
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
          sendResponse({ reply: data.reply });
        })
        .catch(err => {
          history.pop();
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
          return fetch('http://localhost:8080/api/screenshot', {
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
        .then(data => sendResponse({ reply: data.reply }))
        .catch(err => sendResponse({ error: err.toString() }));
    });

    return true;
  }

  // Test connection from settings page
  if (request.type === 'TEST_CONNECTION') {
    fetch('http://localhost:8080/api/ask', {
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
    history.length = 0;
    sendResponse({ ok: true });
    return true;
  }
});
