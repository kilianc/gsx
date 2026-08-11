// Playground page controller.
//
// Owns the worker running the compiler, and the watchdog that replaces it when
// interpreted code will not come back.
//
// Work is split in two because the halves have different risk. Compiling is a
// source transform and always terminates, so its result can be shown
// immediately. Running interprets the reader's code and may never return, so it
// is what the watchdog guards — and why the generated Go stays on screen even
// for a snippet that hangs.

(() => {
  const TIMEOUT_MS = 4000;
  const DEBOUNCE_MS = 300;

  const editor = document.getElementById("pg-editor");
  const goPane = document.getElementById("pg-go");
  const htmlPane = document.getElementById("pg-html");
  const preview = document.getElementById("pg-preview");
  const errorBox = document.getElementById("pg-error");
  const status = document.getElementById("pg-status");
  const tabs = document.querySelectorAll("[data-pane]");

  if (!editor) return;

  let worker = null;
  let ready = false;
  let seq = 0;
  let pending = null; // {id, action, timer}
  let queued = false;

  // Source that hung. Re-running it after the restart would hang again, so the
  // watchdog would fire forever; it stays parked until the reader edits.
  let poisoned = null;

  function setStatus(text, kind) {
    status.textContent = text;
    status.className = "pg-status" + (kind ? " is-" + kind : "");
  }

  function spawn() {
    if (worker) worker.terminate();
    ready = false;
    pending = null;
    queued = false;
    setStatus("Loading compiler…", "busy");

    worker = new Worker("./playground.worker.js");
    worker.onmessage = (e) => {
      const msg = e.data;

      if (msg.type === "ready") {
        ready = true;
        setStatus("Ready", "ok");
        send();
        return;
      }

      if (msg.type === "fatal") {
        showError("The compiler crashed: " + msg.error);
        poisoned = editor.value;
        spawn();
        return;
      }

      if (msg.type === "result" && pending && msg.id === pending.id) {
        clearTimeout(pending.timer);
        const action = pending.action;
        pending = null;
        apply(action, msg.result);
        next();
      }
    };
    worker.onerror = (e) => showError("The worker failed: " + e.message);
  }

  // send starts the compile half; run follows once it lands.
  function send() {
    if (!ready || pending) {
      queued = ready;
      return;
    }
    post("compile");
  }

  function next() {
    if (queued) {
      queued = false;
      send();
      return;
    }
    // Nothing newer to do; the run half is scheduled by apply().
  }

  function post(action) {
    const id = ++seq;
    pending = {
      id,
      action,
      timer: setTimeout(() => onTimeout(action), TIMEOUT_MS),
    };
    if (action === "run") setStatus("Running…", "busy");
    worker.postMessage({ id, action, src: editor.value });
  }

  function onTimeout(action) {
    pending = null;
    poisoned = editor.value;
    showError(
      "Timed out after " +
        TIMEOUT_MS / 1000 +
        "s and the compiler was restarted.\n\n" +
        "An endless loop in Page() will do this. The generated Go above is still " +
        "current — only running it timed out."
    );
    spawn();
  }

  function apply(action, result) {
    if (result.go || result.go_html) {
      goPane.innerHTML = result.go_html || escapeHTML(result.go);
    }

    if (result.error) {
      showError(result.error, result.stage);
      return;
    }

    if (action === "compile") {
      // Compiled cleanly, so it is worth trying to run — unless this exact
      // source already hung once.
      errorBox.hidden = true;
      if (editor.value === poisoned) {
        setStatus("Paused — edit to retry", "bad");
        return;
      }
      post("run");
      return;
    }

    errorBox.hidden = true;
    setStatus("Ready", "ok");
    htmlPane.textContent = result.html;

    // sandbox="" keeps scripts, forms and navigation out of the preview. The
    // markup is the reader's own, but it is still untrusted here.
    preview.srcdoc =
      '<!doctype html><meta charset="utf-8">' +
      "<style>body{font:14px/1.5 system-ui,sans-serif;margin:12px;color:#111}" +
      "table{border-collapse:collapse}td,th{border:1px solid #ccc;padding:4px 8px}</style>" +
      result.html;
  }

  function showError(message, stage) {
    errorBox.hidden = false;
    errorBox.textContent = message;
    setStatus(stage ? "Error in " + stage : "Error", "bad");
  }

  function escapeHTML(s) {
    return s.replace(
      /[&<>]/g,
      (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;" })[c]
    );
  }

  let debounce;
  editor.addEventListener("input", () => {
    // Any edit clears the parked source, so a fixed loop runs again.
    if (poisoned !== null && editor.value !== poisoned) poisoned = null;
    clearTimeout(debounce);
    debounce = setTimeout(send, DEBOUNCE_MS);
  });

  // Tab should indent, not leave the editor.
  editor.addEventListener("keydown", (e) => {
    if (e.key !== "Tab") return;
    e.preventDefault();
    const { selectionStart: s, selectionEnd: t, value } = editor;
    editor.value = value.slice(0, s) + "\t" + value.slice(t);
    editor.selectionStart = editor.selectionEnd = s + 1;
  });

  tabs.forEach((tab) => {
    tab.addEventListener("click", () => {
      tabs.forEach((t) => t.classList.toggle("is-active", t === tab));
      document.querySelectorAll("[data-pane-body]").forEach((body) => {
        body.hidden = body.dataset.paneBody !== tab.dataset.pane;
      });
    });
  });

  spawn();
})();
