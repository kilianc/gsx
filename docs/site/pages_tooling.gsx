package site

func LiveReloadPage() Page {
	return Page{
		Slug:     "live-reload",
		Title:    "Live reload",
		Subtitle: "Save a file, see the change. gsx dev regenerates, restarts your server and reloads the browser.",
		Body:     (
			<>
				{Section("start", "Start it",
					Shell("gsx dev"),
					P(Text("Then open "), Code("http://localhost:8080"), Text(".")),
					P(Text("On every save GSX regenerates the changed "), Code(".gsx"), Text(" files, restarts your app, waits for it to accept connections, and only then reloads the browser.")),
					Note(P(Text("The wait matters. A Go server has to be rebuilt and restarted before a refresh can show anything new, so a reload fired the moment a file changed would land on a stale process. This is why "),
						Code("gsx dev"), Text(" supervises your application rather than only watching files."))),
				)}

				{Section("config", "Configuration",
					Shell(`gsx dev \
  -run "go run ./cmd/server" \
  -app-addr localhost:3000 \
  -addr localhost:8080`),
					<table>
						<thead><tr><th>Flag</th><th>Default</th><th>Meaning</th></tr></thead>
						<tbody>
							<tr><td><code>-run</code></td><td><code>go run .</code></td><td>Command that builds and starts your app</td></tr>
							<tr><td><code>-app-addr</code></td><td><code>localhost:3000</code></td><td>Where your app listens</td></tr>
							<tr><td><code>-addr</code></td><td><code>localhost:8080</code></td><td>Where to point your browser</td></tr>
							<tr><td><code>-dir</code></td><td>current directory</td><td>Directory to watch</td></tr>
							<tr><td><code>-debounce</code></td><td><code>120ms</code></td><td>How long to let file events settle</td></tr>
						</tbody>
					</table>,
				)}

				{Section("noconfig", "Your app needs no changes",
					P(Text("The reload client is injected by a reverse proxy on the way through, not rendered by your code. Nothing in your application refers to GSX, and a production build cannot accidentally ship a development client.")),
					P(Text("Requests to "), Code("-addr"), Text(" are forwarded to "), Code("-app-addr"),
						Text(". HTML responses get a small script appended before "), Code("</body>"),
						Text("; everything else is passed through untouched.")),
				)}

				{Section("errors", "Build failures",
					P(Text("A failed build is pushed to the browser as a full-page overlay carrying the same message the terminal shows:")),
					Out("overlay", `
GSX build failed

app/page.gsx:7:22: mismatched closing tag </h2>, expected </h1>

  7 |     <h1>version three</h2>
    |                      ^
`),
					P(Text("The overlay stays until the next successful build. A tab opened "), Em("while"),
						Text(" the build is broken is shown the error too, rather than a blank page.")),
					P(Text("While the app restarts, the proxy serves a placeholder that also carries the reload client, so a tab that lands mid-restart recovers on its own instead of showing a browser error.")),
				)}

				{Section("ci", "Generation in CI",
					P(Text("Generated files are checked in, so CI should verify they are current:")),
					Shell("gsx -check ./..."),
					P(Text("It writes nothing and exits non-zero listing any stale file, so a "),
						Code(".gsx"), Text(" edit can never land without its regenerated "), Code(".gsx.go"), Text(".")),
				)}
			</>
		),
	}
}

func EditorsPage() Page {
	return Page{
		Slug:     "editors",
		Title:    "Editors",
		Subtitle: "A gopls-backed language server gives .gsx files the Go tooling you already use.",
		Body:     (
			<>
				{Section("how", "How it works",
					P(Text("GSX ships a language server that sits between your editor and "), Code("gopls"),
						Text(". It compiles the "), Code(".gsx"), Text(" buffer to a virtual Go view, forwards the request, and maps positions back — so diagnostics, hover and go-to-definition land on the code you wrote, not on generated output.")),
					<ul>
						<li>Diagnostics, including type errors from the generated code</li>
						<li>Hover</li>
						<li>Go to definition</li>
						<li>Completion for Go expressions</li>
					</ul>,
					P(Text("Some requests GSX answers itself, because gopls cannot: it has no notion of a tag name, and the generated Go view is not what you are editing.")),
					<ul>
						<li><strong>Tag completion</strong> after <code>&lt;</code> — components defined in the file first, then HTML elements</li>
						<li><strong>Attribute completion</strong> inside a start tag, including the JSX spellings, with the caret landing between the quotes</li>
						<li><strong>Closing tags</strong> after <code>&lt;/</code>, naming whichever tag is still open</li>
						<li><strong>Formatting</strong>, so format-on-save works</li>
					</ul>,
					P(Text("GSX's own parse errors are reported directly, positioned on the offending character.")),
					Note(P(Text("Completion works on a buffer that does not parse — which is the normal state while typing "),
						Code("<Car"), Text(". It scans rather than parses for exactly that reason: parsing would mean offering nothing at the moment you are asking."))),
				)}

				{Section("editing", "Editing",
					P(Text("The extension adds the editing behaviour a JSX-like language needs, none of which can come from the language server — it has to react to a keystroke before the buffer is parseable:")),
					<ul>
						<li><strong>Auto-close tags.</strong> Typing <code>&gt;</code> inserts the matching closing tag and leaves the caret between them. Void elements and self-closing tags are left alone.</li>
						<li><strong>Linked editing.</strong> Renaming an opening tag renames its closing tag as you type.</li>
						<li><strong>Snippets.</strong> <code>comp</code>, <code>layout</code>, <code>frag</code>, <code>each</code>, <code>if</code> and friends.</li>
						<li><strong>Indentation.</strong> Pressing enter between a tag pair indents the body and puts the closing tag on its own line.</li>
						<li><strong>File nesting.</strong> Generated <code>.gsx.go</code> files collapse under their source.</li>
					</ul>,
					P(Text("Syntax highlighting understands the boundary between Go and markup: components are coloured differently from HTML elements, prose between tags is not highlighted as Go, and a comparison like "),
						Code("a < b"), Text(" is never mistaken for a tag.")),
					<h3>Formatting</h3>,
					P(Text("The language server formats "), Code(".gsx"), Text(" buffers, so format-on-save works. The same formatter runs on the command line:")),
					Shell("gsx fmt -w ./..."),
					P(Text("Go code is formatted exactly as gofmt would format it. Markup keeps the shape you gave it and is only re-indented to match its surroundings — the formatter deliberately does not reflow attributes or move children onto their own lines.")),
					P(Text("Use "), Code("gsx fmt -l ./..."), Text(" in CI to fail on unformatted sources, and "),
						Code("gsx fmt -d"), Text(" to see the diff.")),
					<h3>Commands</h3>,
					<table>
						<thead><tr><th>Command</th><th>Does</th></tr></thead>
						<tbody>
							<tr><td>GSX: Generate All</td><td><code>gsx ./...</code> in a terminal</td></tr>
							<tr><td>GSX: Generate Current File</td><td>regenerate just this file</td></tr>
							<tr><td>GSX: Start Dev Server</td><td><code>gsx dev</code></td></tr>
							<tr><td>GSX: Open Generated Go File</td><td>open the <code>.gsx.go</code> beside the editor (<code>alt+o</code>)</td></tr>
							<tr><td>GSX: Restart Language Server</td><td>restart <code>gsx lsp</code></td></tr>
						</tbody>
					</table>,
				)}

				{Section("install", "Installing",
					<h3>1. The CLI</h3>,
					Shell("go install github.com/kilianc/gsx/cmd/gsx@latest"),
					P(Text("This is what the extension runs, so it has to be installed even if you only want editor support. "),
						Text("The binary lands in "), Code("$(go env GOPATH)/bin"), Text(", which must be on your "), Code("PATH"), Text(".")),
					P(Text("Check it:")),
					Shell("gsx -h"),

					<h3>2. The extension</h3>,
					P(Text("It is not on the Marketplace yet, so build it from the repository. This needs "),
						Link("https://www.docker.com", "Docker"), Text(" — the Node toolchain is pinned in a container so nothing is installed on your machine:")),
					Shell(`git clone https://github.com/kilianc/gsx
cd gsx
make vsix
code --install-extension vscode/gsx-vscode/gsx-vscode-*.vsix`),
					P(Text("For Cursor, swap the last line for "), Code("cursor --install-extension …"), Text(".")),
					P(Text("If you already have Node and would rather not use Docker:")),
					Shell(`cd vscode/gsx-vscode
npm install && npm run compile && npx vsce package --no-dependencies`),
					P(Text("Then reload the window and open a "), Code(".gsx"), Text(" file. "),
						Text("The status bar reports the language server starting; "),
						Code("GSX: Restart Language Server"), Text(" in the command palette restarts it.")),
					Note(P(Strong("On macOS, "), Code("gsx"), Text(" collides with Ghostscript's "), Code("gsx"),
						Text(" if that is installed. The extension prefers a workspace-local "), Code("./bin/gsx"),
						Text(" and the usual Go install paths before falling back to "), Code("PATH"),
						Text("; you can also set "), Code("gsx.executablePath"), Text(" explicitly."))),
				)}

				{Section("minimal", "Highlighting only",
					P(Text("Without the extension, treating "), Code(".gsx"), Text(" as Go gives you syntax highlighting that is close enough to read:")),
					Out("settings.json", `
{
  "files.associations": {
    "*.gsx": "go"
  }
}
`),
					P(Text("Tag expressions will be highlighted as Go rather than as markup, and gopls will report the tags as syntax errors — this is a stopgap, not a setup.")),
				)}

				{Section("api", "Embedding the compiler",
					P(Text("To compile "), Code(".gsx"), Text(" from your own tooling:")),
					GSX(`
import "github.com/kilianc/gsx/pkg/gsx"

out, err := gsx.CompileFile("page.gsx", src)
`),
					P(Text("The internal compiler under "), Code("internal/gsx/..."), Text(" is not part of the public API.")),
				)}
			</>
		),
	}
}
