// Background service worker — relays messages to the local Go backend.
// Conversation history is held in memory here; the page never sees it.

const history: Array<{ role: string; content: string }> = [];

// ── Manifest command: Ctrl+Shift+Z ─────────────────────────────────────────
// Fired directly by Chrome, so user-gesture context is preserved and
// captureVisibleTab works without needing <all_urls> host permission fallback.
chrome.commands.onCommand.addListener(async (command) => {
  if (command !== 'screenshot') return;

  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (!tab?.id) return;

  // Ask content script to hide overlay and wait until it confirms repaint
  try {
    await chrome.tabs.sendMessage(tab.id, { type: 'SNAP_HIDE' });
  } catch { /* content script may not be injected yet */ }

  // Capture after overlay is gone from the frame
  let dataUrl: string;
  try {
    dataUrl = await chrome.tabs.captureVisibleTab({ format: 'png' });
  } catch (err) {
    chrome.tabs.sendMessage(tab.id, { type: 'SNAP_RESPONSE', error: String(err) }).catch(() => {});
    return;
  }

  const base64 = dataUrl.replace(/^data:image\/png;base64,/, '');

  try {
    const res = await fetch('http://localhost:8080/api/screenshot', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ image: base64 }),
    });
    const data = await res.json();
    chrome.tabs.sendMessage(tab.id, { type: 'SNAP_RESPONSE', reply: data.reply }).catch(() => {});
  } catch (err) {
    chrome.tabs.sendMessage(tab.id, { type: 'SNAP_RESPONSE', error: String(err) }).catch(() => {});
  }
});

// ── Message handler ────────────────────────────────────────────────────────
chrome.runtime.onMessage.addListener((request, _sender, sendResponse) => {

  // Text chat
  if (request.type === 'ASK_LLM') {
    history.push({ role: 'user', content: request.payload });

    fetch('http://localhost:8080/api/ask', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ messages: history }),
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

    return true;
  }

  // Screenshot via Snap button (uses <all_urls> host permission path)
  if (request.type === 'SCREENSHOT_ASK') {
    chrome.tabs.captureVisibleTab({ format: 'png' })
      .then(dataUrl => {
        const base64 = dataUrl.replace(/^data:image\/png;base64,/, '');
        return fetch('http://localhost:8080/api/screenshot', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ image: base64 }),
        });
      })
      .then(res => res.json())
      .then(data => sendResponse({ reply: data.reply }))
      .catch(err => sendResponse({ error: err.toString() }));

    return true;
  }

  // Clear conversation history
  if (request.type === 'CLEAR_HISTORY') {
    history.length = 0;
    sendResponse({ ok: true });
    return true;
  }
});
