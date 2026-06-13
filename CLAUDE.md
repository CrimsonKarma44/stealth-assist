# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Stealth Assist is a Chrome (Manifest V3) browser extension paired with a local Go HTTP backend. The extension spoofs browser visibility/focus/fullscreen APIs to bypass tab-switch and focus-loss detection, and provides a draggable chat overlay powered by Claude (Anthropic) running through the local server.

## Architecture

```
by-pass_plugin/
├── extension/   # TypeScript/Vite Chrome extension (MV3)
└── server/      # Go HTTP server (port 8080)
```

### Extension — three independently compiled entry points

- **`src/content/inject.ts`** — runs in `world: "MAIN"` at `document_start`. Spoofs `document.visibilityState` → `'visible'`, `document.hidden` → `false`, `document.hasFocus` → `() => true`, `document.fullscreenElement` → mock div. Intercepts `EventTarget.prototype.addEventListener` to silently drop `visibilitychange`, `blur`, and `focusout` events. All errors are silently caught — no `console.*` calls anywhere in this file (logging would reveal the extension's presence).

- **`src/content/ui.ts`** — runs in the isolated extension context at `document_idle`. Full chat overlay with:
  - `Ctrl+Shift+X` opens the overlay; if text is selected it pre-fills the input
  - Textarea input — `Enter` sends, `Shift+Enter` adds newline
  - Draggable via header (`mousedown` converts `bottom/right` anchor to `top/left` on first drag)
  - Minimize (`−`) collapses to title bar; `Ctrl+Shift+X` also un-minimizes
  - **Copy** button — copies last raw Claude reply to clipboard
  - **Clear** button — wipes chat and sends `CLEAR_HISTORY` to reset server-side history
  - Markdown rendered via `marked` (code blocks, bold, lists, etc.)
  - DOM elements carry no obvious IDs; CSS classes use opaque names (`__md`, `__sc`, `__ta`) to reduce fingerprinting surface
  - `z-index: 2147483647` (max) keeps the overlay above everything

- **`src/background/background.ts`** — Service Worker. Maintains `history[]` (conversation turns) in service-worker memory — never written to `localStorage` or any page-accessible storage. Handles two message types:
  - `ASK_LLM` — pushes user turn to history, POSTs `{ messages: history }` to `http://localhost:8080/api/ask`, pushes assistant reply; pops user turn on fetch failure
  - `CLEAR_HISTORY` — resets `history.length = 0`
  - Returns `true` from `onMessage` to signal async response (removing this breaks all replies)

### Go Server

- **`server/main.go`** — `net/http` on `:8080`. Single route: `POST /api/ask`. CORS open (`*`). Decodes `{ messages: []llm.Message }` and rejects empty arrays with 400.
- **`server/llm/client.go`** — `AskLLM(messages []Message)`. Calls `POST https://api.anthropic.com/v1/messages` using raw `net/http` (no external Go deps). Model: `claude-opus-4-8`, max tokens: 1024. Includes a hardcoded system prompt (concise, markdown-formatted answers). `Message` is exported so `main.go` can decode directly into `llm.Message`.
- **`server/.env`** — holds `ANTHROPIC_API_KEY`. Never committed. Must be sourced before running.

## Commands

### Extension

```bash
cd extension
npm install          # first time only (installs vite, marked, @types/chrome)
npm run build        # production build → dist/
npm run watch        # rebuild on file changes
```

### Go Server

```bash
cd server
source .env                            # load ANTHROPIC_API_KEY into shell
go run main.go                         # dev server on :8080
go build -o server_bin main.go         # compile binary
```

### Loading in Chrome

1. `npm run build` in `extension/`
2. `chrome://extensions` → enable Developer Mode → **Load unpacked** → select `extension/dist/`
3. After any rebuild: click the **refresh icon** on the extension card (no need to re-load unpacked)

## Key Constraints

- `inject.ts` **must** run in `world: "MAIN"` at `document_start` — isolated world or later run_at makes spoofing ineffective.
- The background service worker **must** return `true` from `onMessage` — without it, async replies are silently dropped.
- Vite builds each entry as a separate flat bundle (`entryFileNames: 'src/[name].js'`). Shared chunks across content script worlds break Chrome MV3 — do not change this.
- The server request body changed from `{ text: string }` to `{ messages: []llm.Message }` — old single-turn format no longer works.
- Conversation history lives only in the service worker's in-memory `history[]`. It is wiped on service worker restart (browser restart / extension reload) or when the user clicks Clear.
- The server has no auth on `/api/ask` — it relies on localhost-only exposure.
- No `console.*` calls in `inject.ts` — any logging reveals the extension to the page.

## Pending / Future Work

- **Screenshot / Vision mode** — `chrome.tabs.captureVisibleTab` to capture visible tab and send to Claude vision for canvas-based exam platforms. Requires adding `"tabs"` to manifest permissions (deferred).
