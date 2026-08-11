package dev

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		path string
		kind changeKind
		want bool
	}{
		{"page.gsx", changeGSX, true},
		{"/a/b/page.gsx", changeGSX, true},
		{"main.go", changeGo, true},

		// Our own generated output. A `.gsx` edit already queued the rebuild
		// that wrote this file, so reacting to it too would rebuild and reload
		// twice for every save.
		{"page.gsx.go", changeNone, false},
		{"/a/b/page.gsx.go", changeNone, false},

		// Editor scratch files.
		{".#page.gsx", changeNone, false},
		{"page.gsx~", changeNone, false},
		{"/a/.!4771!page.gsx", changeNone, false},
		{"page.go.swp", changeNone, false},

		{"README.md", changeNone, false},
		{"style.css", changeNone, false},
	}
	for _, tt := range tests {
		kind, ok := classify(tt.path)
		if kind != tt.kind || ok != tt.want {
			t.Errorf("classify(%q) = (%v, %v), want (%v, %v)", tt.path, kind, ok, tt.kind, tt.want)
		}
	}
}

func TestInject(t *testing.T) {
	script := []byte("<script>x</script>")
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "before body close",
			in:   "<html><body><p>hi</p></body></html>",
			want: "<html><body><p>hi</p><script>x</script></body></html>",
		},
		{
			// A handler may omit </body>; fall back to </html>.
			name: "before html close",
			in:   "<html><p>hi</p></html>",
			want: "<html><p>hi</p><script>x</script></html>",
		},
		{
			// Fragments are common with htmx-style partial responses.
			name: "fragment appends",
			in:   "<p>hi</p>",
			want: "<p>hi</p><script>x</script>",
		},
		{
			// Only the last </body> counts, so a literal one in page text does
			// not capture the injection.
			name: "uses last body close",
			in:   "<body><code>&lt;/body&gt;</code></body>",
			want: "<body><code>&lt;/body&gt;</code><script>x</script></body>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(inject([]byte(tt.in), script)); got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestProxyInjectsIntoHTMLOnly(t *testing.T) {
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/data.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"a":1}`))
		default:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<html><body><h1>hi</h1></body></html>"))
		}
	}))
	defer app.Close()

	target, err := url.Parse(app.URL)
	if err != nil {
		t.Fatal(err)
	}
	proxy := httptest.NewServer(NewProxy(target, NewBroker()))
	defer proxy.Close()

	t.Run("html gets the client", func(t *testing.T) {
		body := get(t, proxy.URL+"/")
		if !strings.Contains(body, "data-gsx-live") {
			t.Errorf("reload client not injected: %s", body)
		}
		if !strings.Contains(body, "<h1>hi</h1>") {
			t.Errorf("original content lost: %s", body)
		}
	})

	t.Run("json is untouched", func(t *testing.T) {
		if body := get(t, proxy.URL+"/data.json"); body != `{"a":1}` {
			t.Errorf("json body = %q, want it unmodified", body)
		}
	})
}

// While the app is restarting the proxy cannot connect. It must still serve the
// reload client, or the tab shows a dead browser error and never recovers.
func TestProxyServesClientWhenAppIsDown(t *testing.T) {
	// Port 1 is reserved and refuses connections.
	target, err := url.Parse("http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	proxy := httptest.NewServer(NewProxy(target, NewBroker()))
	defer proxy.Close()

	resp, err := http.Get(proxy.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	if !strings.Contains(string(buf[:n]), "data-gsx-live") {
		t.Error("fallback page must carry the reload client so the tab recovers")
	}
}

func TestBrokerFansOutAndReplaysErrors(t *testing.T) {
	b := NewBroker()

	a, replay := b.subscribe()
	if replay != nil {
		t.Errorf("nothing published yet, got replay %+v", replay)
	}
	c, _ := b.subscribe()

	b.Publish(Event{Kind: "reload"})
	for i, ch := range []chan Event{a, c} {
		select {
		case ev := <-ch:
			if ev.Kind != "reload" {
				t.Errorf("client %d got %q", i, ev.Kind)
			}
		case <-time.After(time.Second):
			t.Errorf("client %d received nothing", i)
		}
	}

	// A tab that connects while the build is broken must be told so, rather
	// than waiting for the next event that may never come.
	b.Publish(Event{Kind: "error", Message: "boom"})
	<-a
	<-c
	late, replay := b.subscribe()
	if replay == nil || replay.Kind != "error" || replay.Message != "boom" {
		t.Errorf("late subscriber replay = %+v, want the error", replay)
	}

	// A successful build clears it.
	b.Publish(Event{Kind: "reload"})
	<-late
	if _, replay := b.subscribe(); replay != nil {
		t.Errorf("replay = %+v, want nil after a successful build", replay)
	}
}

// A client that stops reading is mid-reload and about to reconnect. It must not
// block the build loop.
func TestBrokerDoesNotBlockOnSlowClient(t *testing.T) {
	b := NewBroker()
	b.subscribe()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			b.Publish(Event{Kind: "reload"})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked on a client that stopped reading")
	}
}

func TestJSONString(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain", `"plain"`},
		{"a\nb", `"a\nb"`},
		{`say "hi"`, `"say \"hi\""`},
		{`back\slash`, `"back\\slash"`},
		{"tab\there", `"tab\there"`},
		{"ctrl\x01", `"ctrl\u0001"`},
	}
	for _, tt := range tests {
		if got := jsonString(tt.in); got != tt.want {
			t.Errorf("jsonString(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

// A build failure must reach the browser instead of silently leaving stale HTML.
func TestRebuildPublishesGenerateErrors(t *testing.T) {
	b := NewBroker()
	ch, _ := b.subscribe()

	opts := Options{
		Generate: func() error { return errPlain("page.gsx:3:5: mismatched closing tag") },
	}
	runner := &Runner{Command: "true", Addr: "127.0.0.1:1"}

	rebuild(context.Background(), opts, runner, b, Change{GSX: true}, false)

	select {
	case ev := <-ch:
		if ev.Kind != "error" {
			t.Fatalf("kind = %q, want error", ev.Kind)
		}
		if !strings.Contains(ev.Message, "mismatched closing tag") {
			t.Errorf("message = %q, want the compiler output", ev.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("no event published for a failed generate")
	}
}

type errPlain string

func (e errPlain) Error() string { return string(e) }

func get(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	buf := make([]byte, 1<<16)
	n, _ := resp.Body.Read(buf)
	return string(buf[:n])
}
