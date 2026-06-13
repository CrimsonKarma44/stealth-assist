// Background service worker — relays messages to the local Go backend.
// Conversation history is held in memory here; the page never sees it.

const history: Array<{ role: string; content: string }> = [];

chrome.runtime.onMessage.addListener((request, _sender, sendResponse) => {
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
        history.pop(); // remove the user turn that failed
        sendResponse({ error: err.toString() });
      });

    return true; // async response
  }

  if (request.type === 'CLEAR_HISTORY') {
    history.length = 0;
    sendResponse({ ok: true });
    return true;
  }
});
