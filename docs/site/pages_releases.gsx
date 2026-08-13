package site

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// repo is where the releases, tags and pull requests linked from this page live.
const repo = "https://github.com/kilianc/gsx"

func ReleasesPage() Page {
	releases := Releases()

	var sections []Node
	for i, r := range releases {
		sections = append(sections, releaseSection(r, i == 0))
	}

	return Page{
		Slug:     "releases",
		Title:    "Releases",
		Subtitle: "Every tagged version, what changed in it, and what is on main that no tag carries yet.",
		Body:     (
			<>
				{releaseIndex(releases)}

				{Section("install", "Installing a version",
					P(Text("The CLI is a Go program, so "), Code("go install"), Text(" is the whole story. "),
						Code("@latest"), Text(" follows the newest tag:")),
					Shell("go install " + strings.TrimPrefix(repo, "https://") + "/cmd/gsx@latest"),
					P(Text("Pin the version when a build has to keep producing the same output:")),
					Shell("go install " + strings.TrimPrefix(repo, "https://") + "/cmd/gsx@" + Latest().Version),
					P(Text("Every release below carries a "), Code(".vsix"), Text(" of the editor extension as an asset — "),
						Link("./editors.html", "installing it"), Text(" is on the editors page. The extension runs the "),
						Code("gsx"), Text(" binary, so install the CLI first.")),
					Note(P(Text("Generated "), Code(".gsx.go"), Text(" files are checked in, so upgrading the compiler is a "),
						Code("gsx ./..."), Text(" and a diff you can read. If the generated output changed, the release notes say so."))),
				)}

				{unreleasedSection()}
				{Group(sections)}
			</>
		),
	}
}

// releaseIndex is the jump list under the page head. It earns its place as the
// history grows: the entries themselves are long enough that the shape of the
// list is not visible from any one of them.
func releaseIndex(releases []Release) Node {
	var pills []Node
	if len(Unreleased()) > 0 {
		pills = append(pills, <a class="release-pill" href="#unreleased">on main</a>)
	}
	for _, r := range releases {
		pills = append(pills, <a class="release-pill" href={"#" + anchorID(r.Version)}>{r.Version}</a>)
	}
	return <nav class="release-index" aria-label="Versions">{pills}</nav>
}

// unreleasedSection lists what main carries beyond the newest tag. It is
// rendered from the same Change type as a release so the entries do not have to
// be rewritten when they become part of one.
func unreleasedSection() Node {
	changes := Unreleased()
	if len(changes) == 0 {
		return nil
	}

	return (
		<section class="section release" id="unreleased">
			<div class="release-head">
				<h2><a class="anchor" href="#unreleased">On main</a></h2>
				<span class="release-badge release-badge-open">unreleased</span>
				<a class="release-date ext" href={repo + "/compare/" + Latest().Version + "...main"}>
					{"since " + Latest().Version}
				</a>
			</div>
			<p class="release-summary">
				Merged and deployed — this site is built from main — but not yet in a tag. To run it, install
				from <code>@main</code> rather than <code>@latest</code>.
			</p>
			{Group(changeNodes(changes))}
		</section>
	)
}

// releaseSection renders one tagged release: what it is, what changed, and the
// links back to the tag it came from.
func releaseSection(r Release, latest bool) Node {
	id := anchorID(r.Version)

	var badge Node
	if latest {
		badge = <span class="release-badge">latest</span>
	}

	links := []Node{
		<a class="ext" href={repo + "/releases/tag/" + r.Version}>Release notes and downloads</a>,
	}
	if r.Previous != "" {
		links = append(links, <a class="ext" href={repo + "/compare/" + r.Previous + "..." + r.Version}>
			{"Every commit since " + r.Previous}
		</a>)
	}

	return (
		<section class="section release" id={id}>
			<div class="release-head">
				<h2><a class="anchor" href={"#" + id}>{r.Version}</a></h2>
				{badge}
				<time class="release-date" datetime={r.Date}>{humanDate(r.Date)}</time>
			</div>
			<p class="release-summary">{prose(r.Summary)}</p>
			{Group(changeNodes(r.Changes))}
			<p class="release-links">{links}</p>
		</section>
	)
}

func changeNodes(changes []Change) []Node {
	var out []Node
	for _, c := range changes {
		out = append(out, <h3>{prose(c.Title)}{Group(refLinks(c.Refs))}</h3>)
		for _, para := range c.Body {
			out = append(out, <p>{prose(para)}</p>)
		}
	}
	return out
}

func refLinks(refs []int) []Node {
	var out []Node
	for _, n := range refs {
		out = append(out, <a class="release-ref" href={fmt.Sprintf("%s/pull/%d", repo, n)}>{"#" + strconv.Itoa(n)}</a>)
	}
	return out
}

// prose renders a note, setting runs between backticks as inline code.
//
// That is deliberately the whole of the markup a note may carry: the entries
// are prose about code, and anything that wants more than an identifier set in
// mono belongs on a documentation page the note can link to.
func prose(s string) Node {
	var out []Node
	for i, part := range strings.Split(s, "`") {
		if part == "" {
			continue
		}
		if i%2 == 1 {
			out = append(out, <code>{part}</code>)
			continue
		}
		out = append(out, Text(part))
	}
	return Group(out)
}

// anchorID turns a version into a fragment. The dots are dropped because a
// fragment carrying them is awkward to select and to quote, not because it
// would be invalid.
func anchorID(version string) string {
	return strings.ReplaceAll(version, ".", "-")
}

// humanDate renders a stored YYYY-MM-DD for reading. A malformed date is shown
// as it was written rather than failing the build: the date is the least
// load-bearing thing on the page.
func humanDate(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return t.Format("2 January 2006")
}
