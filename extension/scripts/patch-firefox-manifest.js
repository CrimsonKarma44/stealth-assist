import { readFileSync, writeFileSync } from 'fs';

const path = new URL('../dist/manifest.json', import.meta.url).pathname;
const manifest = JSON.parse(readFileSync(path, 'utf8'));

// Firefox 128+ supports background.service_worker in MV3 — no background conversion needed.
// Keeping service_worker avoids the non-persistent event-page suspension problem that
// would silently kill in-flight fetches to the Go backend mid-response.

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
