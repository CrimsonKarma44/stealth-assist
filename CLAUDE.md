# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Stealth Assist is a Chrome/Firefox (Manifest V3) browser extension paired with a local Go HTTP backend. The extension spoofs browser visibility/focus/fullscreen APIs to bypass tab-switch and focus-loss detection, and provides a draggable chat overlay powered by Claude (Anthropic) running through the local server. It also supports a screenshot/vision mode that captures the visible tab and sends it to Claude for analysis.

## Architecture

```
by-pass_plugin/
├── extension/   # TypeScript/Vite MV3 extension (Chrome + Firefox builds)
│   └── scripts/patch-firefox-manifest.js  # post-build manifest patcher
└── server/      # Go HTTP server (port 8080)
```

### Extension — three independently compiled entry points

- **`src/content/inject.ts`** — runs in `world: "MAIN"` at `document_start` (Chrome only — stripped in Firefox build). Spoofs `document.visibilityState` → `'visible'`, `document.hidden` → `false`, `document.hasFocus` → `() => true`, `document.fullscreenElement` → mock div. Intercepts `EventTarget.prototype.addEventListener` to silently drop `visibilitychange`, `blur`, and `focusout` events. All errors are silently caught — no `console.*` calls anywhere in this file (logging would reveal the extension's presence).

- **`src/content/ui.ts`** — runs in the isolated extension context at `document_idle`. Full chat overlay with:
  - `Ctrl+Shift+X` opens the overlay; if text is selected it pre-fills the input
  - Textarea input — `Enter` sends, `Shift+Enter` adds newline
  - Draggable via header (`mousedown` converts `bottom/right` anchor to `top/left` on first drag)
  - Minimize (`−`) collapses to title bar; `Ctrl+Shift+X` also un-minimizes
  - **Copy** button — copies last raw Claude reply to clipboard
  - **Clear** button — wipes chat and sends `CLEAR_HISTORY` to reset server-side history
  - **Snap** button — triggers screenshot/vision mode: hides overlay, double `requestAnimationFrame`, sends `SCREENSHOT_ASK`, restores overlay with response
  - Markdown rendered via `marked` (code blocks, bold, lists, etc.)
  - `SNAP_HIDE` message listener: hides overlay and responds after 2 rAF cycles so background can capture a clean frame
  - `SNAP_RESPONSE` message listener: restores overlay and appends Claude's vision response
  - DOM elements carry no obvious IDs; CSS classes use opaque names (`__md`, `__sc`, `__ta`) to reduce fingerprinting surface
  - `z-index: 2147483647` (max) keeps the overlay above everything
  - Null guards after every `await` in `submit()` — overlay may be closed while request is in-flight

- **`src/background/background.ts`** — Service Worker (Chrome) / persistent background script (Firefox). Maintains `history[]` (conversation turns) in memory — never written to `localStorage` or any page-accessible storage. Handles:
  - `chrome.commands.onCommand` for `"screenshot"` command — gesture-preserved path: sends `SNAP_HIDE`, awaits confirmation, calls `captureVisibleTab`, POSTs to `/api/screenshot`, sends `SNAP_RESPONSE`
  - `ASK_LLM` message — pushes user turn to history, POSTs `{ messages: history }` to `/api/ask`, pushes assistant reply; pops user turn on fetch failure
  - `SCREENSHOT_ASK` message — non-gesture path (Snap button): calls `captureVisibleTab` via `<all_urls>` permission, POSTs to `/api/screenshot`
  - `CLEAR_HISTORY` message — resets `history.length = 0`
  - Returns `true` from `onMessage` to signal async response

### Go Server

- **`server/main.go`** — `net/http` on `:8080`. Two routes: `POST /api/ask` and `POST /api/screenshot`. CORS open (`*`). Decodes respective request types and rejects bad input with 400.
- **`server/llm/client.go`** — Exports `Message` struct. `AskLLM(messages []Message)` for text chat. `AskVision(imageBase64 string)` for vision. Both call `doRequest(body []byte)` which POSTs to `https://api.anthropic.com/v1/messages` using raw `net/http` (no external Go deps). Model: `claude-opus-4-8`. Chat: max_tokens 1024, vision: max_tokens 2048. Vision sends a `[]contentBlock` with an image block (base64 PNG) and a text instruction.
- **`server/.env`** — holds `export ANTHROPIC_API_KEY=...`. Never committed. Must use `export` prefix so `source .env` exports to child processes.

## Commands

### Extension

```bash
cd extension
npm install               # first time only (installs vite, marked, @types/chrome)
npm run build             # Chrome build → dist/
npm run build:firefox     # Firefox build → dist/ (patches manifest for Firefox MV3)
npm run watch             # rebuild on file changes (Chrome)
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
2. `chrome://extensions` → enable Developer Mode → **Load unpacked** → select `extension/dist/` (the folder)
3. After any rebuild: click the **refresh icon** on the extension card
4. Check/reassign shortcuts at `chrome://extensions/shortcuts`

### Loading in Firefox / Firefox-based browsers

1. `npm run build:firefox` in `extension/`
2. `about:debugging` → **This Firefox** → **Load Temporary Add-on…** → select `extension/dist/manifest.json` (the file, not the folder)
3. Temporary add-ons are cleared on browser restart — reload each session
4. For permanent install without signing: set `xpinstall.signatures.required` = `false` in `about:config` (only works on Firefox Developer Edition, Nightly, or forks that allow it), then install the `.xpi`

## Firefox Build — What the Patch Does

`extension/scripts/patch-firefox-manifest.js` runs after `vite build` and modifies `dist/manifest.json`:

1. **`background.service_worker` → `background.scripts[]`** — Firefox MV3 uses scripts arrays, not service workers
2. **`background.persistent: true`** — prevents Firefox from suspending the idle background script, which would wipe conversation history and kill the message listener
3. **`world: "MAIN"` removed from all content_scripts entries** — Firefox has inconsistent support for this field across Firefox-based browsers; leaving it in can silently prevent content scripts from loading at all. This means `inject.ts` spoof features don't work on Firefox, but the overlay loads correctly.

## Key Constraints

- `inject.ts` **must** run in `world: "MAIN"` at `document_start` on Chrome — isolated world makes spoofing ineffective. This field is stripped for Firefox builds.
- The background script **must** return `true` from `onMessage` — without it, async replies are silently dropped.
- Vite builds each entry as a separate flat bundle (`entryFileNames: 'src/[name].js'`). Shared chunks across content script worlds break Chrome MV3 — do not change this.
- `host_permissions` must be `["<all_urls>"]` (the literal string) — `"*://*/*"` does not satisfy Chrome's internal `captureVisibleTab` permission check.
- The manifest `commands` entry for `"screenshot"` is required to preserve user-gesture context in the background service worker, enabling `captureVisibleTab` without `<all_urls>` as a fallback.
- The overlay is hidden with `visibility: hidden` (not `display: none`) before screenshot capture, and two `requestAnimationFrame` cycles are awaited to guarantee repaint before the tab is captured.
- Conversation history lives only in the background script's in-memory `history[]`. It is wiped on service worker restart / browser restart / extension reload, or when the user clicks Clear. Screenshot replies are NOT added to history.
- The server has no auth — it relies on localhost-only exposure.
- No `console.*` calls in `inject.ts` — any logging reveals the extension to the page.
- `.env` must use `export KEYWORD=value` format so `source .env` exports the variable to child processes.
