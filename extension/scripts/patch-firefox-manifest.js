import { readFileSync, writeFileSync } from 'fs';
import { fileURLToPath } from 'url';

/**
 * Transforms a Chrome MV3 manifest into a Firefox-compatible form.
 * Pure function — no file I/O.
 */
export function patchManifest(manifest) {
  // Firefox < 128 uses background.scripts[], not background.service_worker.
  if (manifest.background?.service_worker) {
    const sw = manifest.background.service_worker;
    manifest.background = { scripts: [sw] };
  }

  // Firefox has inconsistent support for world: "MAIN" across forks — drop it.
  if (Array.isArray(manifest.content_scripts)) {
    manifest.content_scripts = manifest.content_scripts.map(entry => {
      const { world: _world, ...rest } = entry;
      return rest;
    });
  }

  return manifest;
}

// Run file I/O only when executed directly (not when imported by tests).
const __filename = fileURLToPath(import.meta.url);
if (process.argv[1] === __filename) {
  const path = new URL('../dist/manifest.json', import.meta.url).pathname;
  const manifest = JSON.parse(readFileSync(path, 'utf8'));
  const patched = patchManifest(manifest);
  writeFileSync(path, JSON.stringify(patched, null, 2));
  console.log('Firefox manifest patched.');
  console.log('  background:', patched.background);
  console.log('  content_scripts worlds removed:', patched.content_scripts?.map(e => e.js));
}
