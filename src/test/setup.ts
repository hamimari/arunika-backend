import '@testing-library/jest-dom';

// jsdom doesn't implement matchMedia — antd's responsive grid/breakpoint
// hooks call it on every mount, so any page test using antd components
// needs this polyfill.
// jsdom also doesn't implement ResizeObserver, which antd's Select/dropdown
// positioning relies on.
if (!window.ResizeObserver) {
  window.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}

if (!window.matchMedia) {
  window.matchMedia = (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  }) as unknown as MediaQueryList;
}
