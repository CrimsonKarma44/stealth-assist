# Stealth Assist

A Chrome/Firefox extension + local Go backend that bypasses tab-visibility and focus-detection used by proctoring software, and gives you a private AI assistant accessible via keyboard shortcuts on any page. Supports Anthropic Claude, OpenAI, and Google Gemini.

## How it works

```
Browser Extension (MV3 — Chrome + Firefox)
  ├── inject.ts      → spoofs visibility/focus APIs in the page's own JS context
  ├── ui.ts          → draggable chat overlay (Ctrl+Shift+X) + Snap button
  ├── background.ts  → holds conversation history, proxies requests to Go server
  └── options.ts     → settings page (provider, model, API key)

Go Server (local or Render)
  ├── /api/ask        → text chat with conversation memory
  └── /api/screenshot → vision mode (screenshot analysis)
```

### Stealth layer

`inject.ts` runs at `document_start` inside the page's MAIN JavaScript world and permanently overrides:

- `document.visibilityState` → always `"visible"`
- `document.hidden` → always `false`
- `document.hasFocus` → always `true`
- `document.fullscreenElement` → mock element
- `EventTarget.prototype.addEventListener` → silently drops `visibilitychange`, `blur`, and `focusout` event registrations

> **Firefox note:** the MAIN world injection doesn't work on Firefox (stripped at build time), so the spoofing layer is Chrome/Chromium only. The chat overlay and screenshot features work on both.

### Chat overlay

Press **Ctrl+Shift+X** on any page to open the assistant:

- **With text selected** — selection is pre-filled into the input
- **Without selection** — opens with an empty input, type freely
- **Enter** sends · **Shift+Enter** adds a newline
- Drag the header to reposition the overlay anywhere on screen
- **−** minimizes to a title bar; **Ctrl+Shift+X** un-minimizes
- **⚙** opens the settings page to switch provider or update your key
- **Copy** — copies the last reply to clipboard
- **Clear** — wipes the chat and resets conversation memory

Responses are rendered as markdown (code blocks, bold, lists, etc.).

### Screenshot / vision mode

Two ways to capture the screen and ask the AI what's on it:

| Method | Trigger | How it works |
|---|---|---|
| **Keyboard** | **Ctrl+Shift+Z** (Mac: Cmd+Shift+Z) | Manifest command fires directly in the background service worker, preserving the user-gesture context required for `captureVisibleTab`. |
| **Snap button** | Click **Snap** in the overlay | Content script sends a `SCREENSHOT_ASK` message; the `<all_urls>` host permission grants `captureVisibleTab` access without needing a user gesture. |

In both cases the overlay is hidden before capture, then restored with the AI's answer. The model reads every question visible on screen and numbers its answers to match.

### Conversation memory

The background service worker maintains a rolling message history for all interactions. Every text turn and every screenshot exchange is added to history and persisted to `chrome.storage.local`, so follow-up text questions after a Snap have full context. Memory is never written to `localStorage` or any page-accessible storage. Clicking **Clear** resets it.

---

## Setup

### 1. Go server

**Option A — run locally:**
```bash
cd server
go run main.go
```
Server listens on `http://localhost:8080`. Keep it running while using the extension. No `.env` file needed — API keys are configured in the extension and sent per request.

**Option B — deploy to Render:**
1. Push the repo to GitHub
2. Render dashboard → **New → Web Service** → connect the repo
3. Render auto-detects `server/render.yaml` — root dir `server`, build `go build -o server_bin main.go`, no env vars to set
4. Deploy — the extension already defaults to `https://stealth-assist.onrender.com`

> **Render free tier note:** web services spin down after 15 minutes of inactivity, causing a ~30s cold start on the next request. The $7/month paid tier keeps the service always-on.

### 2. Extension

```bash
cd extension
npm install       # first time only
```

**Chrome / Chromium:**
```bash
npm run build     # outputs to extension/dist/
```
Load:
1. Go to `chrome://extensions`
2. Enable **Developer mode** (top-right toggle)
3. Click **Load unpacked** → select `extension/dist/` (the folder)

**Firefox / Firefox-based browsers:**
```bash
npm run build:firefox   # patches manifest for Firefox MV3 compatibility
```
Load:
1. Go to `about:debugging` → **This Firefox**
2. Click **Load Temporary Add-on…** → navigate to `extension/dist/` and select **`manifest.json`** (the file, not the folder)
3. Reload each session — temporary add-ons are cleared on browser restart

After any code change, re-run the appropriate build command and click **Refresh** on the extension card.

### 3. Configure settings

On first install the settings page opens automatically. You can also reach it any time via:
- The **⚙** button in the chat overlay
- Right-clicking the extension icon → **Options**

The extension defaults to `https://stealth-assist.onrender.com`. If running the server locally, change **Server URL** to `http://localhost:8080`. Then pick a provider and paste your API key.

| Provider | Free tier | Where to get a key |
|---|---|---|
| **Google Gemini** | ✓ No credit card required | [aistudio.google.com](https://aistudio.google.com/app/apikey) |
| **Anthropic Claude** | Paid | [console.anthropic.com](https://console.anthropic.com/) |
| **OpenAI** | Paid | [platform.openai.com](https://platform.openai.com/api-keys) |

Select your provider, pick a model, paste the key, click **Save**. Use **Test connection** to verify before closing the page.

### 4. Screenshot shortcut

After loading the extension, Chrome may assign the `Ctrl+Shift+Z` shortcut automatically. If it conflicts with another extension, reassign it at:

```
chrome://extensions/shortcuts
```

Find **Stealth Assist → Capture screen and ask Claude**.

---

## Project structure

```
by-pass_plugin/
├── extension/
│   ├── src/
│   │   ├── content/
│   │   │   ├── inject.ts      # MAIN world spoof script (Chrome only)
│   │   │   └── ui.ts          # Chat overlay UI + Snap button + gear icon
│   │   ├── background/
│   │   │   └── background.ts  # Service worker, history, screenshot, settings relay
│   │   └── options/
│   │       └── options.ts     # Settings page logic
│   ├── public/
│   │   ├── manifest.json      # MV3 manifest
│   │   ├── src/
│   │   │   └── options.html   # Settings page HTML
│   │   └── icons/
│   ├── scripts/
│   │   └── patch-firefox-manifest.js
│   └── vite.config.ts
└── server/
    ├── main.go                # HTTP server, CORS, /api/ask + /api/screenshot
    ├── render.yaml            # Render deployment blueprint
    └── llm/
        └── client.go          # Multi-provider LLM client (Anthropic, OpenAI, Gemini)
```

---

## Development

| Task | Command |
|---|---|
| Build extension (Chrome) | `cd extension && npm run build` |
| Build extension (Firefox) | `cd extension && npm run build:firefox` |
| Watch mode (auto-rebuild) | `cd extension && npm run watch` |
| Run server | `cd server && go run main.go` |
| Compile server binary | `cd server && go build -o server_bin main.go` |

---

## Permissions

| Permission | Why |
|---|---|
| `activeTab` | Access the active tab's metadata |
| `tabs` | Query active tab for screenshot capture |
| `scripting` | Inject content scripts programmatically |
| `storage` | Store API key and provider settings locally |
| `<all_urls>` host permission | Required for `captureVisibleTab` on the Snap button path (no user gesture in message channel) |
