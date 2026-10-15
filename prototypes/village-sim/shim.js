// Runs the real batnet app (internal/web/app) inside the simulator's phone.
// Instead of the network, the app's requests go to the simulated house's real
// server, which runs in WebAssembly in the parent page (window.batnetSim).
(() => {
  'use strict';
  const q = new URLSearchParams(location.search);
  const house = q.get('house') || '';
  let token = q.get('token') || '';
  const sim = () => parent.batnetSim;

  // Each phone in the simulator gets its own outbox: without IndexedDB the app
  // keeps it in memory, so one person's waiting posts never reach another's phone.
  try { Object.defineProperty(window, 'indexedDB', { value: undefined, configurable: true }); } catch (e) {}

  // The simulated village keeps UTC, like the simulator's clock, so the phone does too.
  for (const f of ['toLocaleTimeString', 'toLocaleDateString', 'toLocaleString']) {
    const orig = Date.prototype[f];
    Date.prototype[f] = function (locales, opts) { return orig.call(this, locales, Object.assign({ timeZone: 'UTC' }, opts)); };
  }

  const realFetch = window.fetch.bind(window);
  window.fetch = async (input, opts = {}) => {
    const url = new URL(typeof input === 'string' ? input : input.url, location.href);
    if (url.origin !== location.origin || !url.pathname.startsWith('/api/')) return realFetch(input, opts);
    const raw = sim().http(house, (opts.method || 'GET').toUpperCase(), url.pathname + url.search, opts.body || '', token);
    let out;
    try { out = JSON.parse(raw); } catch (e) { throw new TypeError(raw); } // the bridge refused the call
    token = out.token;
    return new Response(out.body, { status: out.status, headers: { 'Content-Type': 'application/json' } });
  };

  // The live stream: poll the house server's subscription every 300 ms.
  class SimEventSource extends EventTarget {
    constructor(url) {
      super();
      this.url = url;
      this.readyState = 0;
      this.timer = setTimeout(() => {
        sim().events(house, token); // the first call subscribes
        this.readyState = 1;
        this.dispatchEvent(new Event('open'));
        this.timer = setInterval(() => this.poll(), 300);
      }, 0);
    }
    poll() {
      let evs = [];
      try { evs = JSON.parse(sim().events(house, token)); } catch (e) { return; }
      for (const ev of evs) this.dispatchEvent(new MessageEvent(ev.name || 'message', { data: ev.data }));
    }
    close() { clearTimeout(this.timer); clearInterval(this.timer); this.readyState = 2; }
  }
  SimEventSource.CONNECTING = 0; SimEventSource.OPEN = 1; SimEventSource.CLOSED = 2;
  window.EventSource = SimEventSource;
})();
