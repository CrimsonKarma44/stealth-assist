# Stealth Assist

A Chrome extension + local Go backend that bypasses tab-visibility and focus-detection used by proctoring software, and gives you a private AI assistant (Claude) accessible via a keyboard shortcut on any page.

## How it works

```
Chrome Extension (MV3)
  ├── inject.ts    → spoofs visibility/focus APIs in the page's own JS context
  ├── ui.ts        → draggable chat overlay (Ctrl+Shift+X)
  └── background.ts → holds conversation history, proxies requests to Go server

Go Server (localhost:8080)
  └── proxies messages to Anthropic Claude API (API key never touches the browser)
```

### Stealth layer

`inject.ts` runs at `document_start` inside the page's MAIN JavaScript world and permanently overrides:

- `document.visibilityState` → always `"visible"`
- `document.hidden` → always `false`
- `document.hasFocus` → always `true`
- `document.fullscreenElement` → mock element
- `EventTarget.prototype.addEventListener` → silently drops `visibilitychange`, `blur`, and `focusout` event registrations

### Chat overlay

Press **Ctrl+Shift+X** on any page to open the assistant:

- **With text selected** — selection is pre-filled into the input
- **Without selection** — opens with an empty input, type freely
- **Enter** sends · **Shift+Enter** adds a newline
- Drag the header to reposition the overlay anywhere on screen
- **−** minimizes to a title bar; **Ctrl+Shift+X** un-minimizes
- **Copy** — copies Claude's last reply to clipboard
- **Clear** — wipes the chat and resets conversation memory

Responses are rendered as markdown (code blocks, bold, lists, etc.).

### Conversation memory

The background service worker maintains a rolling message history. Every turn is sent to the server so Claude can answer follow-up questions with full context. Memory is held in service-worker RAM — never written to `localStorage` or any page-accessible storage. Clicking **Clear** resets it.

---

## Setup

### 1. Go server

```bash
cd server
```

Create `server/.env` (already present if cloned; never commit this file):
```
export ANTHROPIC_API_KEY=sk-ant-...
```

Run:
```bash
source .env && go run main.go
```

Server listens on `http://localhost:8080`. Keep it running while using the extension.

### 2. Extension

```bash
cd extension
npm install       # first time only
npm run build     # outputs to extension/dist/
```

Load in Chrome:
1. Go to `chrome://extensions`
2. Enable **Developer mode** (top-right toggle)
3. Click **Load unpacked** → select `extension/dist/`

After any code change, run `npm run build` then click the **refresh icon** on the extension card. Refresh the target tab too so the new content scripts are injected.

---

## Project structure

```
by-pass_plugin/
├── extension/
│   ├── src/
│   │   ├── content/
│   │   │   ├── inject.ts      # MAIN world spoof script
│   │   │   └── ui.ts          # Chat overlay UI
│   │   └── background/
│   │       └── background.ts  # Service worker + conversation history
│   ├── public/
│   │   └── manifest.json
│   └── vite.config.ts
└── server/
    ├── main.go                # HTTP server, CORS, request routing
    ├── llm/
    │   └── client.go          # Anthropic API client, system prompt
    └── .env                   # ANTHROPIC_API_KEY (not committed)
```

---

## Development

| Task | Command |
|---|---|
| Build extension | `cd extension && npm run build` |
| Watch mode (auto-rebuild) | `cd extension && npm run watch` |
| Run server | `cd server && source .env && go run main.go` |
| Compile server binary | `cd server && go build -o server_bin main.go` |

---

## Roadmap

- [ ] Screenshot / vision mode — `chrome.tabs.captureVisibleTab` for canvas-based exam platforms (requires `tabs` permission)
