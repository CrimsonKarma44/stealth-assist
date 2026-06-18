import { readFileSync, writeFileSync } from 'fs';

const path = new URL('../dist/manifest.json', import.meta.url).pathname;
const manifest = JSON.parse(readFileSync(path, 'utf8'));

// Firefox MV3 uses background.scripts[] instead of background.service_worker
const swPath = manifest.background?.service_worker;
if (swPath) {
  manifest.background = { scripts: [swPath] };
}

writeFileSync(path, JSON.stringify(manifest, null, 2));
console.log('Firefox manifest patched:', manifest.background);
