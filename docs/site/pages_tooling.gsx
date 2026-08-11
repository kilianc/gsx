package site

func LiveReloadPage() Page {
	return Page{
		Slug:     "live-reload",
		Title:    "Live reload",
		Subtitle: "Save a file, see the change. gsx dev regenerates, restarts your server and reloads the browser.",
		Body: (
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
		Body: (
			<>
				{Section("how", "How it works",
					P(Text("GSX ships a language server that sits between your editor and "), Code("gopls"),
						Text(". It compiles the "), Code(".gsx"), Text(" buffer to a virtual Go view, forwards the request, and maps positions back — so diagnostics, hover and go-to-definition land on the code you wrote, not on generated output.")),
					<ul>
						<li>Diagnostics, including type errors from the generated code</li>
						<li>Hover</li>
						<li>Go to definition</li>
						<li>Completion</li>
					</ul>,
					P(Text("GSX's own parse errors are reported directly, positioned on the offending character.")),
				)}

				{Section("vscode", "VS Code and Cursor",
					P(Text("Install the extension from "), Code("vscode/gsx-vscode/"), Text(", and make sure "),
						Code("gsx"), Text(" is on your "), Code("PATH"), Text(":")),
					Shell("go install github.com/kilianc/gsx/cmd/gsx@latest"),
					P(Text("The extension starts "), Code("gsx lsp"), Text(" for every "), Code(".gsx"),
						Text(" file and nests generated files under their source in the explorer.")),
					Note(P(Text("On macOS, "), Code("gsx"), Text(" collides with Ghostscript's "), Code("gsx"),
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
