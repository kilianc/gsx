package dev

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
)

// NewProxy returns a handler that forwards to the application and injects the
// reload client into HTML responses.
//
// Injecting at the proxy rather than asking the application to render a script
// tag is what keeps `gsx dev` zero-configuration: nothing in the user's code
// refers to GSX, and shipping to production cannot accidentally include a dev
// client.
func NewProxy(target *url.URL, b *Broker) http.Handler {
	rp := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL.Scheme = target.Scheme
			r.URL.Host = target.Host
			r.Host = target.Host

			// Ask for an identity encoding so the body can be rewritten
			// without decompressing whatever the app chose. Responses that are
			// compressed anyway are handled below.
			r.Header.Set("Accept-Encoding", "identity")
		},
		ModifyResponse: func(resp *http.Response) error {
			return injectIntoResponse(resp)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			// The app is mid-restart. Serve a page that carries the reload
			// client so the tab reconnects and refreshes itself once the app
			// is listening again, instead of showing a dead browser error.
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("<!doctype html><title>starting…</title><body>"))
			_, _ = w.Write(Script())
			_, _ = w.Write([]byte("</body>"))
		},
	}

	mux := http.NewServeMux()
	mux.Handle(EventPath, b)
	mux.Handle("/", rp)
	return mux
}

func injectIntoResponse(resp *http.Response) error {
	if !isHTML(resp.Header.Get("Content-Type")) {
		return nil
	}

	body, err := readBody(resp)
	if err != nil {
		return err
	}

	body = inject(body, Script())

	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.Header.Del("Content-Encoding")
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	// The whole point of dev mode is seeing the latest render.
	resp.Header.Set("Cache-Control", "no-store")
	resp.Header.Del("ETag")
	return nil
}

// readBody reads a response body, transparently decompressing gzip in case the
// application compresses regardless of Accept-Encoding.
func readBody(resp *http.Response) ([]byte, error) {
	var r io.Reader = resp.Body
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		r = gz
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	_ = resp.Body.Close()
	return body, nil
}

func isHTML(contentType string) bool {
	return strings.HasPrefix(strings.TrimSpace(strings.ToLower(contentType)), "text/html")
}
