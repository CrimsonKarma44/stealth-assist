# Stealth Assist

A Chrome extension + local Go backend that bypasses tab-visibility and focus-detection used by proctoring software, and gives you a private Claude AI assistant accessible via keyboard shortcuts on any page.

## How it works

```
Chrome Extension (MV3)
  ├── inject.ts    → spoofs visibility/focus APIs in the page's own JS context
  ├── ui.ts        → draggable chat overlay (Ctrl+Shift+X) + Snap button
  └── background.ts → holds conversation history, proxies requests to Go server

Go Server (localhost:8080)
  ├── /api/ask        → text chat with conversation memory
  └── /api/screenshot → vision mode via Claude multimodal API
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

### Screenshot / vision mode

Two ways to capture the screen and ask Claude what's on it:

| Method | Trigger | How it works |
|---|---|---|
| **Keyboard** | **Ctrl+Shift+Z** (Mac: Cmd+Shift+Z) | Manifest command fires directly in the background service worker, preserving the user-gesture context Chrome requires for `captureVisibleTab`. |
| **Snap button** | Click **Snap** in the overlay | Content script sends a `SCREENSHOT_ASK` message; the `<all_urls>` host permission grants `captureVisibleTab` access without needing a user gesture. |

In both cases the overlay is hidden before capture (so it doesn't appear in the screenshot), then restored with Claude's answer once the server responds. Claude reads every question visible on screen and numbers its answers to match.

### Conversation memory

The background service worker maintains a rolling message history for text chat. Every turn is sent to the server so Claude can answer follow-up questions with full context. Memory is held in service-worker RAM — never written to `localStorage` or any page-accessible storage. Clicking **Clear** resets it. Screenshot responses are standalone (not added to the chat history).

---

## Setup

### 1. Go server

```bash
cd server
```

Create `server/.env` (never commit this file):
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

### 3. Screenshot shortcut

After loading the extension, Chrome may assign the `Ctrl+Shift+Z` shortcut automatically. If it conflicts with another extension, go to:

```
chrome://extensions/shortcuts
```

Find **Stealth Assist → Capture screen and ask Claude** and reassign if needed.

---

## Project structure

```
by-pass_plugin/
├── extension/
│   ├── src/
│   │   ├── content/
│   │   │   ├── inject.ts      # MAIN world spoof script
│   │   │   └── ui.ts          # Chat overlay UI + Snap button
│   │   └── background/
│   │       └── background.ts  # Service worker, history, screenshot capture
│   ├── public/
│   │   ├── manifest.json      # MV3 manifest with commands + <all_urls>
│   │   └── icons/             # icon16/32/48/128.png
│   └── vite.config.ts
└── server/
    ├── main.go                # HTTP server, CORS, /api/ask + /api/screenshot
    ├── llm/
    │   └── client.go          # Anthropic API client (text + vision)
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

## Permissions

| Permission | Why |
|---|---|
| `activeTab` | Access the active tab's metadata |
| `tabs` | Query active tab for screenshot capture |
| `scripting` | Inject content scripts programmatically |
| `storage` | Extension settings (future use) |
| `<all_urls>` host permission | Required for `captureVisibleTab` on the Snap button path (no user gesture in message channel) |
