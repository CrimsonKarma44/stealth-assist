// This script is injected into the MAIN world (the page's actual JS context).
// It spoofs visibility, focus, and fullscreen APIs.

(function() {
  const spoofVisibility = () => {
    Object.defineProperty(document, 'visibilityState', {
      get: () => 'visible',
      configurable: true,
    });
    Object.defineProperty(document, 'hidden', {
      get: () => false,
      configurable: true,
    });
    Object.defineProperty(document, 'hasFocus', {
      value: () => true,
      configurable: true,
    });
  };

  const spoofFullscreen = () => {
    // Basic mock element to return when queried for fullscreen element.
    const mockFsElement = document.createElement('div');
    Object.defineProperty(document, 'fullscreenElement', {
      get: () => mockFsElement,
      configurable: true,
    });
  };

  const interceptEvents = () => {
    const originalAddEventListener = EventTarget.prototype.addEventListener;
    EventTarget.prototype.addEventListener = function(type, listener, options) {
      if (type === 'visibilitychange' || type === 'blur' || type === 'focusout') {
        // Wrap the listener to never be called or just ignore adding it.
        // For stealth, maybe we add a dummy listener, but let's just ignore to prevent the page from seeing it trigger.
        return;
      }
      return originalAddEventListener.call(this, type, listener, options);
    };

    // Mask the proxy
    Object.defineProperty(EventTarget.prototype.addEventListener, 'toString', {
      value: () => 'function addEventListener() { [native code] }',
      configurable: true,
      writable: true,
    });
  };

  try {
    spoofVisibility();
    spoofFullscreen();
    interceptEvents();
  } catch (e) {
    // silent fail — logging would reveal the extension's presence
  }
})();
