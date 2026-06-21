import { describe, it, expect } from 'vitest';
import { patchManifest } from '../patch-firefox-manifest.js';

describe('patchManifest', () => {
  it('converts service_worker to scripts array', () => {
    const input = {
      background: { service_worker: 'src/background.js' },
      content_scripts: [],
    };
    const result = patchManifest(input);
    expect(result.background).toEqual({ scripts: ['src/background.js'] });
    expect(result.background.service_worker).toBeUndefined();
  });

  it('removes world field from all content_scripts entries', () => {
    const input = {
      content_scripts: [
        { js: ['src/inject.js'], world: 'MAIN', matches: ['<all_urls>'] },
        { js: ['src/ui.js'], matches: ['<all_urls>'] },
      ],
    };
    const result = patchManifest(input);
    expect(result.content_scripts[0].world).toBeUndefined();
    expect(result.content_scripts[0].js).toEqual(['src/inject.js']);
    expect(result.content_scripts[0].matches).toEqual(['<all_urls>']);
    expect(result.content_scripts[1].world).toBeUndefined();
  });

  it('preserves all other content_script fields', () => {
    const input = {
      content_scripts: [
        { js: ['a.js'], css: ['style.css'], world: 'MAIN', run_at: 'document_start', matches: ['<all_urls>'] },
      ],
    };
    const result = patchManifest(input);
    expect(result.content_scripts[0]).toEqual({
      js: ['a.js'],
      css: ['style.css'],
      run_at: 'document_start',
      matches: ['<all_urls>'],
    });
  });

  it('is a no-op when background already uses scripts array', () => {
    const input = {
      background: { scripts: ['src/background.js'] },
    };
    const result = patchManifest(input);
    expect(result.background).toEqual({ scripts: ['src/background.js'] });
  });

  it('handles missing content_scripts gracefully', () => {
    const input = { background: { service_worker: 'bg.js' } };
    const result = patchManifest(input);
    expect(result.background.scripts).toEqual(['bg.js']);
    expect(result.content_scripts).toBeUndefined();
  });

  it('handles empty content_scripts array', () => {
    const input = { content_scripts: [] };
    const result = patchManifest(input);
    expect(result.content_scripts).toEqual([]);
  });

  it('mutates and returns the same object', () => {
    const input = { background: { service_worker: 'bg.js' }, content_scripts: [] };
    const result = patchManifest(input);
    expect(result).toBe(input);
  });
});
