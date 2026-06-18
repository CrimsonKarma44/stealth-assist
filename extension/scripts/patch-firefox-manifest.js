import { readFileSync, writeFileSync } from 'fs';

const path = new URL('../dist/manifest.json', import.meta.url).pathname;
const manifest = JSON.parse(readFileSync(path, 'utf8'));

// Firefox MV3 uses background.scripts[] instead of background.service_worker.
// persistent: true keeps the background page alive — without it Firefox suspends
// it when idle, wiping conversation history and stopping the message listener.
const swPath = manifest.background?.service_worker;
if (swPath) {
  manifest.background = { scripts: [swPath], persistent: true };
}

// Firefox has limited/inconsistent support for world: "MAIN" in content scripts.
// Drop the field so the entry falls back to the isolated extension world.
// inject.ts spoof features won't work but the overlay (ui.js) will load cleanly.
if (Array.isArray(manifest.content_scripts)) {
  manifest.content_scripts = manifest.content_scripts.map(entry => {
    const { world: _world, ...rest } = entry;
    return rest;
  });
}

writeFileSync(path, JSON.stringify(manifest, null, 2));
console.log('Firefox manifest patched.');
console.log('  background:', manifest.background);
console.log('  content_scripts worlds removed:', manifest.content_scripts.map(e => e.js));
