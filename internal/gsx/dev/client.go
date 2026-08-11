package dev

import (
	"bytes"
	"fmt"
)

// clientScript is injected into every HTML response the proxy passes through.
//
// It is deliberately dependency-free and defensive: it runs inside the user's
// own page, so it must not introduce globals, must survive the server being
// down between a restart, and must remove its own overlay cleanly.
const clientScript = `<script data-gsx-live>
(() => {
  const PATH = %q;
  let es, wasConnected = false;

  const overlayID = "__gsx_overlay";
  const removeOverlay = () => document.getElementById(overlayID)?.remove();

  const showError = (msg) => {
    removeOverlay();
    const el = document.createElement("div");
    el.id = overlayID;
    el.setAttribute("style", [
      "position:fixed", "inset:0", "z-index:2147483647",
      "background:rgba(12,12,14,.92)", "color:#f4f4f5",
      "font:13px/1.55 ui-monospace,SFMono-Regular,Menlo,monospace",
      "padding:32px", "overflow:auto", "white-space:pre-wrap",
      "-webkit-font-smoothing:antialiased",
    ].join(";"));
    const h = document.createElement("div");
    h.textContent = "GSX build failed";
    h.setAttribute("style", "color:#ff6b6b;font-weight:600;margin-bottom:16px;font-size:14px");
    const body = document.createElement("div");
    body.textContent = msg;
    el.append(h, body);
    document.body ? document.body.appendChild(el) : addEventListener("DOMContentLoaded", () => document.body.appendChild(el));
  };

  const connect = () => {
    es = new EventSource(PATH);

    es.addEventListener("connected", () => {
      // A reconnect after the stream dropped means the server restarted while
      // this tab was open, so the page it is showing is stale.
      if (wasConnected) location.reload();
      wasConnected = true;
      removeOverlay();
    });

    es.addEventListener("reload", () => location.reload());
    es.addEventListener("error", (e) => {
      try { showError(JSON.parse(e.data)); } catch { showError("build failed"); }
    });

    // EventSource retries on its own, but not after an explicit close, and it
    // gives up if the server is unreachable at connect time. Retry manually so
    // a tab opened before the app is up eventually attaches.
    es.onerror = () => {
      es.close();
      setTimeout(connect, 500);
    };
  };

  connect();
})();
</script>`

// Script returns the reload client, wired to the event endpoint.
func Script() []byte {
	return []byte(fmt.Sprintf(clientScript, EventPath))
}

var (
	bodyClose = []byte("</body>")
	htmlClose = []byte("</html>")
)

// inject places the reload client at the end of an HTML document.
//
// It appends before </body> so the script runs after the page content, and
// falls back to </html> and then to the end of the document, because plenty of
// real handlers emit fragments or omit the closing tags entirely.
func inject(body, script []byte) []byte {
	for _, marker := range [][]byte{bodyClose, htmlClose} {
		if i := bytes.LastIndex(body, marker); i >= 0 {
			out := make([]byte, 0, len(body)+len(script))
			out = append(out, body[:i]...)
			out = append(out, script...)
			out = append(out, body[i:]...)
			return out
		}
	}
	return append(body, script...)
}
