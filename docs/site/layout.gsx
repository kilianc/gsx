package site

import (
	"strings"

	"github.com/kilianc/gsx/internal/gsx/highlight"
)

// Page is one documentation page.
type Page struct {
	Slug     string
	Title    string
	Subtitle string
	Body     Node

	// Scripts are script URLs to load at the end of the body, relative to the
	// site root. Only the playground uses this; prose pages stay static.
	Scripts []string
	// Styles are extra stylesheet URLs. The site's own CSS is inlined; this is
	// for vendored CSS too large to inline, like the editor's.
	Styles []string
	// Modules are ES module URLs. The editor bundle is ESM because Monaco is.
	Modules []string
	// Wide drops the sidebar and lets the page use the full window, which an
	// editor beside its output needs and an article does not.
	Wide bool
}

// Layout wraps a page in the site chrome.
func Layout(p Page, pages []Page) Node {
	shell := "shell"
	var sidebar Node
	if p.Wide {
		shell += " shell-wide"
	} else {
		sidebar = Sidebar(p, pages)
	}

	var scripts []Node
	for _, src := range p.Scripts {
		scripts = append(scripts, <script src={src} defer></script>)
	}

	var styles []Node
	for _, href := range p.Styles {
		styles = append(styles, <link rel="stylesheet" href={href} />)
	}

	// Modules come before the deferred scripts: both wait for parsing, and the
	// controller polls for the editor rather than assuming it has arrived.
	for _, src := range p.Modules {
		scripts = append(scripts, <script type="module" src={src}></script>)
	}

	var footer Node
	if !p.Wide {
		footer = Footer()
	}

	return (
		<>
			{Raw("<!doctype html>\n")}
			<html lang="en">
				<head>
					<meta charset="utf-8" />
					<meta name="viewport" content="width=device-width, initial-scale=1" />
					<title>{p.Title + " — GSX"}</title>
					<meta name="description" content={p.Subtitle} />
					{El("style", Raw(stylesheet))}
					{Group(styles)}
				</head>
				<body>
					<a class="skip" href="#content">Skip to content</a>
					{Header()}
					<div class={shell}>
						{sidebar}
						<main id="content">
							<div class="page-head">
								<h1>{p.Title}</h1>
								<p class="lede">{p.Subtitle}</p>
							</div>
							{p.Body}
							{footer}
						</main>
					</div>
					{Group(scripts)}
				</body>
			</html>
		</>
	)
}

func Header() Node {
	return (
		<header class="topbar">
			<a class="brand" href="./index.html">
				<span class="brand-mark">{"<gsx/>"}</span>
			</a>
			<nav class="topnav">
				<a href="./playground.html">Playground</a>
				<a href="./language.html">Language</a>
				<a href="./components.html">Components</a>
				<a href="./live-reload.html">Live reload</a>
				<a href="./editors.html">Editors</a>
				<a class="ext" href="https://github.com/kilianc/gsx">GitHub</a>
			</nav>
		</header>
	)
}

func Sidebar(current Page, pages []Page) Node {
	var links []Node
	for _, p := range pages {
		cls := "side-link"
		if p.Slug == current.Slug {
			cls += " is-current"
		}
		links = append(links, <li><a class={cls} href={"./" + p.Slug + ".html"}>{p.Title}</a></li>)
	}
	return (
		<aside class="sidebar">
			<nav>
				<p class="side-title">Documentation</p>
				<ul>{links}</ul>
			</nav>
		</aside>
	)
}

func Footer() Node {
	return (
		<footer class="footer">
			<p>
				GSX is generated ahead of time — the output is plain, readable Go using
				{" "}<a href="https://pkg.go.dev/maragu.dev/gomponents">gomponents</a>.
			</p>
			<p class="muted">This site is itself written in GSX.</p>
		</footer>
	)
}

// Section is a documentation section with a linkable heading.
func Section(id string, title string, children ...Node) Node {
	return (
		<section class="section" id={id}>
			<h2><a class="anchor" href={"#" + id}>{title}</a></h2>
			{Group(children)}
		</section>
	)
}

func P(children ...Node) Node  { return El("p", Group(children)) }
func Em(s string) Node         { return El("em", Text(s)) }
func Strong(s string) Node     { return El("strong", Text(s)) }
func Link(href, s string) Node { return <a href={href}>{s}</a> }

// Code renders an inline code span.
func Code(s string) Node { return El("code", Text(s)) }

// GSX renders a highlighted GSX or Go snippet.
//
// The highlighter understands tag expressions and the `{...}` boundary, which
// is exactly what a reader needs to see on a page explaining that boundary.
func GSX(src string) Node {
	return codeBlock("gsx", highlight.HTML(strings.TrimSpace(src)))
}

// Out renders generated output or rendered HTML, shown unhighlighted so it
// reads as a result rather than as something to write.
func Out(label, src string) Node {
	return codeBlock("out", `<span class="hl-out-label">`+label+`</span>`+htmlEscape(strings.TrimSpace(src)))
}

// Shell renders a terminal command.
func Shell(src string) Node {
	return codeBlock("sh", htmlEscape(strings.TrimSpace(src)))
}

func codeBlock(kind, inner string) Node {
	return <div class={"code code-" + kind}>{El("pre", El("code", Raw(inner)))}</div>
}

// Split shows source and its result side by side, which is the clearest way to
// present a compiler.
func Split(left, right Node) Node {
	return (
		<div class="split">
			<div>{left}</div>
			<div>{right}</div>
		</div>
	)
}

// Note renders an aside.
func Note(children ...Node) Node {
	return <div class="note">{Group(children)}</div>
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}
