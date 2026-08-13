package site

// The release history behind the releases page.
//
// The notes are checked in rather than fetched from the GitHub API at build
// time. The site is rebuilt and deployed on every push to main, so a page that
// reached out to the network would make an unrelated commit's deploy depend on
// GitHub being reachable, and would let a note edited after the fact rewrite a
// page nobody reviewed. Cutting a tag therefore includes editing this file:
// move what the tag carries out of unreleased and into a new Release.

// Release is one tagged version.
type Release struct {
	// Version is the tag, including the leading v.
	Version string
	// Date is when the tag was published, as YYYY-MM-DD.
	Date string
	// Summary is the one paragraph a reader needs to decide whether this
	// release is worth their afternoon.
	Summary string
	// Changes are the notes, most significant first.
	Changes []Change
	// Previous is the tag to diff against for the full changelog link. Empty
	// on the first release, which has nothing to diff.
	Previous string
}

// Change is one item in a release's notes.
type Change struct {
	// Title is the change in a line.
	Title string
	// Body is one paragraph per element. Text between backticks renders as
	// inline code.
	Body []string
	// Refs are pull request numbers, linked after the title.
	Refs []int
}

// Releases returns every tagged release, newest first.
func Releases() []Release {
	return []Release{
		{
			Version:  "v0.3.1",
			Date:     "2026-08-13",
			Previous: "v0.3.0",
			Summary: "Correctness in the parser, the highlighter and the language server. A `<` in Go code " +
				"is now read the way Go reads it, and a `<` that cannot start a tag is a diagnostic rather " +
				"than a hang. Nothing in the language changed and every generated file in this repository " +
				"is byte-identical, so upgrading is the install and nothing else.",
			Changes: []Change{{
				Title: "A `<` in Go code is read by the token before it",
				Refs:  []int{41, 46},
				Body: []string{
					"Tag detection asked one question — is this `<` followed by a letter — and that question is " +
						"only sound in child position. Go does not require spaces around a comparison, so in Go " +
						"code the answer was often yes for one: `m[a<b]` inside a splice failed with an error " +
						"naming a tag nobody wrote, and `var ok = a<b`, `a<<b` and the `i<n` of an ordinary `for` " +
						"header failed the same way.",
					"What separates a tag from an operator is position, not the byte after the `<`. A tag can " +
						"only start where an operand can start, whereas `<` and `<<` must follow one, so the " +
						"scanner now looks at the token it just passed. It records that token going forward as " +
						"each one is consumed, which is what lets a comment between the operand and the operator " +
						"— `{a /* note */ <b}` — stop hiding it.",
				},
			}, {
				Title: "A stray `<` in text is an error, not a hang",
				Refs:  []int{40},
				Body: []string{
					"`<p>a < b</p>` was enough. The `<` matched none of the shapes the child parser dispatches " +
						"on, and the text run it fell through to stopped on the `<` without consuming it: zero " +
						"bytes read, loop repeats, forever. The language server compiles the editor buffer on " +
						"every keystroke, so the input only had to exist between two keystrokes — one `gsx lsp` " +
						"session spun on such an intermediate state for 33 hours at 100% of a core, forwarding " +
						"nothing to gopls the whole time.",
					"It is now rejected the way JSX rejects it, pointing at the escapes the language reference " +
						"already teaches. Every parse loop asserts that a pass consumes at least one byte, so a " +
						"future stall is a diagnostic naming the file and column rather than a wedged process, " +
						"and a fuzz target hunts specifically for input that would trip it.",
				},
			}, {
				Title: "The highlighter reads `<` the way the parser does",
				Refs:  []int{44},
				Body: []string{
					"The highlighter carries its own lexer, which kept the old rule after the parser learned the " +
						"new one. `a<b`, `a<<b` and the `i<n` of a `for` header still coloured as if `<b` opened a " +
						"tag, and a snippet that reads as markup where the compiler reads a comparison teaches the " +
						"syntax wrong. A test now asks the parser the same question about the same source, so the " +
						"next drift fails there rather than in a rendered page.",
				},
			}, {
				Title: "The language server guards formatting like it guards compiling",
				Refs:  []int{43},
				Body: []string{
					"`textDocument/formatting` called the formatter directly, on the goroutine that pumps every " +
						"client message to gopls — the same wedge the compile watchdog exists to contain, reached " +
						"through a different door. Guarding two of the three ways into the parser and leaving the " +
						"third open is not a guardrail, so both now run under one guard.",
				},
			}, {
				Title: "The test suite runs under the race detector",
				Refs:  []int{45},
				Body: []string{
					"The language server proxy is concurrent by construction, and the detector has already " +
						"earned it: while the watchdog was being written it caught an abandoned goroutine still " +
						"reading a variable its caller had moved on from, in a suite that reported green without " +
						"it. Three seconds on this suite.",
				},
			}, {
				Title: "The site lists its releases",
				Refs:  []int{47},
				Body: []string{
					"This page. It carries the history and, above it, whatever is merged into main that no tag " +
						"carries yet — which is the one thing a releases list on GitHub structurally cannot show.",
				},
			}},
		},
		{
			Version:  "v0.3.0",
			Date:     "2026-08-12",
			Previous: "v0.2.0",
			Summary: "A binary that can identify itself, and a README demo generated by the compiler " +
				"rather than drawn to look like it. The language and the compiler are unchanged from v0.2.0.",
			Changes: []Change{{
				Title: "`gsx -version`",
				Body: []string{
					"An installed binary can now say which release it is, which is the first thing a bug " +
						"report needs: `gsx -version` prints `gsx v0.3.0 go1.26.3 darwin/arm64`.",
					"It reads the version the `go` command already records in the binary, so nothing is " +
						"stamped with `-ldflags` and a build from a checkout reports the commit it came from instead.",
				},
			}, {
				Title: "The README demo is rendered, not mocked",
				Body: []string{
					"The previous animation was a drawn window with text that looked like compiler output. " +
						"Nothing produced it, so it could not be regenerated and went stale whenever the palette " +
						"changed. `cmd/demogen` now writes each frame from the real pieces — the source pane is a " +
						"`.gsx` file coloured by the same highlighter this site uses, the generated pane is what " +
						"`gsx.CompileFile` returns, and the strip underneath is the HTML the component actually renders.",
					"It also compiles a loop instead of a greeting: an ordinary Go `for` with an `if` inside it, " +
						"appending to a `[]Node` spliced into a `<ul>`. Both panes are on screen together, and the " +
						"point is what does not change between them — the `for`, the `if` and the `class +=` are " +
						"byte-identical on the generated side.",
				},
			}},
		},
		{
			Version:  "v0.2.0",
			Date:     "2026-08-12",
			Previous: "v0.1.0",
			Summary: "Documentation, brand and playground polish. The language, the compiler and the CLI are " +
				"unchanged from v0.1.0 — there is nothing to migrate, and generated `.gsx.go` files from v0.1.0 " +
				"stay byte-identical.",
			Changes: []Change{{
				Title: "The README says what GSX is in its first line",
				Body: []string{
					"JSX for Go — and it shows Go control flow used directly inside markup rather than saving " +
						"it for a later section. An animated demo runs above the first code block.",
				},
			}, {
				Title: "A logo",
				Body: []string{
					"Drawn from the brace that holds a splice: `{` and `}` closing around the wordmark, the same " +
						"shape the editor colours as the Go/markup boundary.",
				},
			}, {
				Title: "The site is light in every colour scheme",
				Body: []string{
					"It takes Go's Gopher Blue as its accent and no longer flips to a dark palette based on a " +
						"system preference. The site is mostly code samples whose highlighting has to match the " +
						"prose around it and the playground editor beside it, and one palette is one thing to keep in tune.",
				},
			}, {
				Title: "The playground colours splice braces",
				Body: []string{
					"The way the extension's TextMate grammar does, which makes the boundary between Go and " +
						"markup visible in the same place you would read it in an editor.",
				},
			}},
		},
		{
			Version: "v0.1.0",
			Date:    "2026-08-12",
			Summary: "The first tagged release: the language, the tooling and the editor support, tagged so " +
				"there is something to pin and something to download.",
			Changes: []Change{{
				Title: "The language",
				Body: []string{
					"Tags as Go expressions, fragments `<>…</>`, `{expr}` splices, `{/* comments */}`, components " +
						"as plain functions with typed props, dotted and nested component names, spread attributes, " +
						"the JSX attribute spellings (`className`, `htmlFor`, …), JSX whitespace rules, and HTML " +
						"entities decoded at compile time.",
					"HTML coverage is derived from the gomponents source rather than transcribed, so all 110 " +
						"elements and 78 attributes map to their typed constructors.",
				},
			}, {
				Title: "The tooling",
				Body: []string{
					"`gsx ./...` generates a `.gsx.go` beside every `.gsx`, and `gsx -check ./...` is the CI gate " +
						"that fails if any generated file is stale. `gsx fmt` runs gofmt over the Go and re-indents " +
						"the markup, `gsx dev` regenerates and restarts the app and reloads the browser, and " +
						"`gsx lsp` is a gopls-backed language server.",
				},
			}, {
				Title: "Errors carry a position and a snippet",
				Body: []string{
					"On the terminal, as an editor diagnostic, and as a full-page overlay under `gsx dev`.",
				},
			}, {
				Title: "Editor support",
				Body: []string{
					"Syntax highlighting that knows the Go/markup boundary, auto-closing and linked tag editing, " +
						"snippets, format-on-save, tag and attribute completion, and diagnostics.",
				},
			}},
		},
	}
}

// Unreleased are the changes on main that no tag carries yet.
//
// This site deploys from main, so it is also the honest answer to what an
// install from `@main` gets that an install from `@latest` does not. Empty is
// the normal state immediately after a release: the page drops the section and
// its jump-list entry rather than claiming there is nothing worth shipping.
func Unreleased() []Change {
	return nil
}

// Latest is the newest tagged release.
func Latest() Release { return Releases()[0] }
