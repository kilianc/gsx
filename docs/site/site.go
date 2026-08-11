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
func Serve(addr string) error {
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
		http.NotFound(w, r)
	})

	fmt.Printf("docs: http://%s\n", addr)
	return http.ListenAndServe(addr, nil)
}
