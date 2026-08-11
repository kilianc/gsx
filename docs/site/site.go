// Package site is the GSX documentation site, written in GSX.
package site

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
)

// Pages returns every page in sidebar order.
func Pages() []Page {
	return []Page{
		IndexPage(),
		PlaygroundPage(),
		LanguagePage(),
		ComponentsPage(),
		CompositionPage(),
		OrganizingPage(),
		LiveReloadPage(),
		EditorsPage(),
	}
}

// Render renders one page to a complete HTML document.
func Render(p Page, all []Page) ([]byte, error) {
	var buf bytes.Buffer
	if err := Layout(p, all).Render(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Serve renders on every request, so `go run ./docs -serve` under `gsx dev`
// picks up edits without a separate build step.
//
// assets is a directory of prebuilt files — the playground's wasm bundle and
// its JavaScript — served for any path that is not a page. It may be empty,
// in which case only the prose pages work.
func Serve(addr string, assets string) error {
	var files http.Handler
	if assets != "" {
		files = http.FileServer(http.Dir(assets))
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		slug := strings.Trim(r.URL.Path, "/")
		slug = strings.TrimSuffix(slug, ".html")
		if slug == "" {
			slug = "index"
		}

		pages := Pages()
		for _, p := range pages {
			if p.Slug != slug {
				continue
			}
			html, err := Render(p, pages)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(html)
			return
		}

		if files != nil {
			files.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})

	fmt.Printf("docs: http://%s\n", addr)
	return http.ListenAndServe(addr, nil)
}
