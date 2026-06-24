# Implementation Plan: Stealth Assistant Extension

## 1. Goal
Build a browser extension with a local Golang backend that intercepts browser visibility and fullscreen APIs to bypass tracking, while extracting webpage content to be processed by an LLM via the local server.

## 2. Approach
We will use a two-part architecture:
1.  **TypeScript Browser Extension:** Built with Vite for fast bundling. It will utilize a highly privileged Content Script injected into the `MAIN` world (the webpage's actual context) to spoof APIs like `document.visibilityState`, `window.onblur`, and the Fullscreen API. A standard isolated Content Script will handle reading the DOM and displaying a discreet UI, forwarding requests to the Background Service Worker.
2.  **Golang Local Backend:** A lightweight HTTP server that securely holds LLM API keys. It will receive questions/context from the extension, call the external LLM provider (like OpenAI or Claude), and stream the response back. This keeps CORS issues out of the picture and secures credentials.

## 3. File Changes

### Extension (TypeScript)
*   **Create** `extension/package.json` & `extension/vite.config.ts`: Project configuration and build tooling.
*   **Create** `extension/manifest.json`: Chrome extension Manifest V3 configuration with appropriate host permissions.
*   **Create** `extension/src/content/inject.ts`: Runs in the page's context to redefine getters for `visibilityState`, `hidden`, and intercept event listeners.
*   **Create** `extension/src/content/ui.ts`: Runs in the isolated extension context. Injects the discreet UI overlay and reads page content.
*   **Create** `extension/src/background/background.ts`: Service worker to relay messages between the UI and the Go server.

### Backend (Golang)
*   **Create** `server/go.mod` & `server/go.sum`: Go module definitions.
*   **Create** `server/main.go`: The HTTP server, CORS configuration, and endpoint routing.
*   **Create** `server/llm/client.go`: Handles formatting the prompt and communicating with the LLM API.
*   **Create** `server/.env`: Stores the `OPENAI_API_KEY` or equivalent.

## 4. Implementation Steps

*   **Task 1: Project Scaffolding**
    *   Set up the `extension` folder with Vite and Manifest V3.
    *   Set up the `server` folder and initialize the Go module.
*   **Task 2: Anti-Detection & Spoofing (Extension)**
    *   Implement `extension/src/content/inject.ts`. We will use `Object.defineProperty` to permanently force `document.hidden` to return `false` and `document.visibilityState` to return `'visible'`.
    *   Override `document.fullscreenElement` to return a mock element and block `blur` events from propagating.
*   **Task 3: Page Interaction & UI (Extension)**
    *   Implement `extension/src/content/ui.ts` to listen for a keyboard shortcut (e.g., `Ctrl+Shift+X`). When triggered, it grabs the currently selected text or specific DOM elements and opens a discreet overlay.
*   **Task 4: Communication Bridge**
    *   Implement `extension/src/background/background.ts` to listen for messages from `ui.ts` and perform `fetch` requests to `http://localhost:8080/api/ask`.
*   **Task 5: Golang Backend Server**
    *   Implement `server/main.go` using standard library `net/http` (or a lightweight router like `chi`).
    *   Add CORS middleware to allow requests from `chrome-extension://*`.
    *   Implement a basic LLM connector in `server/llm/client.go` to mock or perform real AI queries.

## 5. Acceptance Criteria
*   **Anti-Cheat Bypass:** A webpage monitoring `visibilitychange`, `blur`, or `document.hidden` does not trigger when the user switches tabs or clicks outside the browser.
*   **Fullscreen Spoofing:** A webpage that enforces fullscreen cannot detect when the user exits native fullscreen.
*   **End-to-End Flow:** Triggering the extension shortcut grabs text, sends it through the Background Script to the Go server, queries an LLM, and displays the text back in the injected extension UI.
*   **Security:** API keys are read from `server/.env` and are never sent to or visible within the browser.

## 6. Verification Steps
1.  Run the backend: `cd server && go run main.go`. Ensure it logs "Listening on :8080".
2.  Build the extension: `cd extension && npm install && npm run build`.
3.  Load the extension into Chrome (`chrome://extensions`, Developer Mode -> Load Unpacked).
4.  Navigate to a test site (e.g., a simple JS Fiddle logging visibility state).
5.  Alt-tab away from the browser. Verify the site still logs that the page is visible.
6.  Highlight text on the page, press the shortcut, and verify the Go server receives the text and the UI displays the response.

## 7. Risks & Mitigations
*   **Risk:** Advanced scripts might detect that native DOM objects have been overridden if not done perfectly (e.g., checking `.toString()` on native functions).
    *   **Mitigation:** Carefully proxy original functions using `Proxy` objects instead of direct reassignment, masking the `toString` behavior to look like `[native code]`.
*   **Risk:** The target website is rendered via `<canvas>` or highly obfuscated React DOM, making text extraction impossible.
    *   **Mitigation:** Future scope: Utilize `chrome.tabs.captureVisibleTab` to send a screenshot to the Go server for OCR or Vision Model processing.