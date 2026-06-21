import { describe, it, expect, vi } from 'vitest';

// Import inject.ts as a side-effect — the IIFE runs immediately, spoofing DOM APIs.
import '../content/inject';

describe('inject.ts — visibility spoofing', () => {
  it('spoofs document.visibilityState to "visible"', () => {
    expect(document.visibilityState).toBe('visible');
  });

  it('spoofs document.hidden to false', () => {
    expect(document.hidden).toBe(false);
  });

  it('spoofs document.hasFocus() to always return true', () => {
    expect(document.hasFocus()).toBe(true);
  });
});

describe('inject.ts — event interception', () => {
  it('silently drops visibilitychange listeners', () => {
    const handler = vi.fn();
    document.addEventListener('visibilitychange', handler);
    document.dispatchEvent(new Event('visibilitychange'));
    expect(handler).not.toHaveBeenCalled();
  });

  it('silently drops blur listeners on DOM elements', () => {
    const handler = vi.fn();
    document.body.addEventListener('blur', handler);
    document.body.dispatchEvent(new Event('blur'));
    expect(handler).not.toHaveBeenCalled();
  });

  it('silently drops focusout listeners', () => {
    const handler = vi.fn();
    document.addEventListener('focusout', handler);
    document.dispatchEvent(new Event('focusout'));
    expect(handler).not.toHaveBeenCalled();
  });

  it('passes through unrelated event listeners', () => {
    const handler = vi.fn();
    document.addEventListener('click', handler);
    document.dispatchEvent(new Event('click'));
    expect(handler).toHaveBeenCalledOnce();
    document.removeEventListener('click', handler);
  });

  it('spoofs addEventListener.toString() to look native', () => {
    expect(EventTarget.prototype.addEventListener.toString()).toContain('[native code]');
  });
});
