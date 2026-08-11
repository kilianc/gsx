// The playground's wasm instance, kept off the main thread.
//
// gsxRun is synchronous and interprets the reader's code, so `for {}` blocks
// whatever thread it is on. Under wasm the Go scheduler is cooperative, so a
// tight loop never yields and a context deadline inside Go will not fire.
// Running here means the page can call terminate() and lose only the worker.

importScripts("./wasm_exec.js");

// Installed by the Go side once the exports exist. Reporting readiness any
// earlier would invite a call that lands before gsxRun is defined.
self.gsxReady = () => postMessage({ type: "ready" });

const go = new Go();

// Deliberately not awaited: main() ends in select{}, so this promise only
// settles if the instance dies.
WebAssembly.instantiateStreaming(fetch("./gsx.wasm"), go.importObject)
  .then((res) => go.run(res.instance))
  .catch((err) => postMessage({ type: "fatal", error: String(err) }));

self.onmessage = (e) => {
  const { id, action, src } = e.data;

  try {
    let result;
    switch (action) {
      case "run":
        result = gsxRun(src);
        break;
      case "compile":
        result = gsxCompile(src);
        break;
      case "highlight":
        result = { html: gsxHighlight(src) };
        break;
      default:
        result = { error: `unknown action ${action}`, stage: "compile" };
    }
    postMessage({ type: "result", id, result });
  } catch (err) {
    // A trap in the wasm instance leaves it unusable, so the page has to
    // replace the worker rather than retry into a corpse.
    postMessage({ type: "fatal", id, error: String(err) });
  }
};
