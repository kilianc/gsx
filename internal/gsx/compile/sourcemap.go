package compile

import (
	"sort"
	"strings"
)

// Position is a 0-based LSP-style position.
type Position struct {
	Line int
	Col  int
}

type offsetMapper struct {
	// edits are non-overlapping, sorted by srcStart/tgtStart.
	edits []edit
}

type edit struct {
	// Replacement of src[srcStart:srcEnd] with tgt[tgtStart:tgtEnd].
	srcStart int
	srcEnd   int
	tgtStart int
	tgtEnd   int

	// If true, mapping from within tgt replacement is allowed and maps to srcStart.
	// If false, mapping within the replaced region returns ok=false.
	mapTgtToSrcInside bool

	// If true, mapping from within src replaced range is allowed and maps to tgtStart.
	// If false, mapping within the replaced region returns ok=false.
	mapSrcToTgtInside bool
}

func (m offsetMapper) tgtOffsetFromSrcOffset(srcOff int) (tgtOff int, ok bool) {
	ok = true
	delta := 0
	for _, e := range m.edits {
		if srcOff < e.srcStart {
			break
		}
		if srcOff >= e.srcStart && srcOff < e.srcEnd {
			if !e.mapSrcToTgtInside {
				return 0, false
			}
			return e.tgtStart + delta, true
		}
		delta += (e.tgtEnd - e.tgtStart) - (e.srcEnd - e.srcStart)
	}
	return srcOff + delta, ok
}

func (m offsetMapper) srcOffsetFromTgtOffset(tgtOff int) (srcOff int, ok bool) {
	ok = true
	delta := 0
	for _, e := range m.edits {
		adjTgtStart := e.tgtStart + delta
		adjTgtEnd := e.tgtEnd + delta
		if tgtOff < adjTgtStart {
			break
		}
		if tgtOff >= adjTgtStart && tgtOff < adjTgtEnd {
			if !e.mapTgtToSrcInside {
				return 0, false
			}
			return e.srcStart, true
		}
		delta += (e.tgtEnd - e.tgtStart) - (e.srcEnd - e.srcStart)
	}
	return tgtOff - delta, ok
}

type lineIndex struct {
	s      string
	starts []int // starts[i] is byte offset of line i
}

func newLineIndex(s string) *lineIndex {
	starts := []int{0}
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &lineIndex{s: s, starts: starts}
}

func (li *lineIndex) offsetFromPos(p Position) (int, bool) {
	if p.Line < 0 || p.Line >= len(li.starts) {
		return 0, false
	}
	off := li.starts[p.Line] + p.Col
	if off < 0 || off > len(li.s) {
		return 0, false
	}
	// clamp column at end of line
	lineEnd := len(li.s)
	if p.Line+1 < len(li.starts) {
		lineEnd = li.starts[p.Line+1] - 1
	}
	if off > lineEnd {
		off = lineEnd
	}
	return off, true
}

func (li *lineIndex) posFromOffset(off int) (Position, bool) {
	if off < 0 || off > len(li.s) {
		return Position{}, false
	}
	// find last start <= off
	i := sort.Search(len(li.starts), func(i int) bool { return li.starts[i] > off }) - 1
	if i < 0 {
		i = 0
	}
	col := off - li.starts[i]
	if col < 0 {
		col = 0
	}
	return Position{Line: i, Col: col}, true
}

// SourceMap maps between original .gsx (source) and the virtual Go view (target) that we feed to gopls.
//
// It is constructed as a composition of text edits:
// - tags -> placeholder calls
// - (optional) import block injection/augmentation
// - placeholder calls -> inline func literal calls (so locals remain in scope)
type SourceMap struct {
	src *lineIndex
	tgt *lineIndex

	// src -> rewritten (placeholders)
	srcToRewritten offsetMapper
	rewrittenToSrc offsetMapper

	// rewritten -> afterImports
	rewrittenToAfterImports offsetMapper
	afterImportsToRewritten offsetMapper

	// afterImports -> target (inline expansions)
	afterImportsToTgt offsetMapper
	tgtToAfterImports offsetMapper
}

func buildRewriteMappers(src string, placeholders []placeholder) (srcToRewritten offsetMapper, rewrittenToSrc offsetMapper) {
	// Each placeholder replaced src[srcStart:srcEnd] with rewritten[tgtStart:tgtEnd].
	edits := make([]edit, 0, len(placeholders))
	for _, p := range placeholders {
		edits = append(edits, edit{
			srcStart: p.srcStart,
			srcEnd:   p.srcEnd,
			tgtStart: p.tgtStart,
			tgtEnd:   p.tgtEnd,
			// Positions inside tags/placeholder regions aren't meaningfully mappable.
			mapTgtToSrcInside: true, // map to tag start for diagnostics
			mapSrcToTgtInside: true, // map tag interior to placeholder start for requests
		})
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].srcStart < edits[j].srcStart })
	srcToRewritten = offsetMapper{edits: edits}

	// For inverse, we can reuse the same edits because srcOffsetFromTgtOffset understands tgt ranges.
	rewrittenToSrc = offsetMapper{edits: edits}
	return
}

func (sm *SourceMap) TargetPositionFromSource(p Position) (Position, bool) {
	srcOff, ok := sm.src.offsetFromPos(p)
	if !ok {
		return Position{}, false
	}
	rewOff, ok := sm.srcToRewritten.tgtOffsetFromSrcOffset(srcOff)
	if !ok {
		return Position{}, false
	}
	impOff, ok := sm.rewrittenToAfterImports.tgtOffsetFromSrcOffset(rewOff)
	if !ok {
		return Position{}, false
	}
	tgtOff, ok := sm.afterImportsToTgt.tgtOffsetFromSrcOffset(impOff)
	if !ok {
		return Position{}, false
	}
	return sm.tgt.posFromOffset(tgtOff)
}

func (sm *SourceMap) SourcePositionFromTarget(p Position) (Position, bool) {
	tgtOff, ok := sm.tgt.offsetFromPos(p)
	if !ok {
		return Position{}, false
	}
	impOff, ok := sm.tgtToAfterImports.srcOffsetFromTgtOffset(tgtOff)
	if !ok {
		return Position{}, false
	}
	rewOff, ok := sm.afterImportsToRewritten.srcOffsetFromTgtOffset(impOff)
	if !ok {
		return Position{}, false
	}
	srcOff, ok := sm.rewrittenToSrc.srcOffsetFromTgtOffset(rewOff)
	if !ok {
		return Position{}, false
	}
	return sm.src.posFromOffset(srcOff)
}

type requiredImports struct {
	needsTags       bool
	needsComponents bool
	qualifyHTML     bool
}

func detectRequiredImports(placeholders []placeholder, loweredExprs []string) requiredImports {
	if len(placeholders) == 0 {
		return requiredImports{}
	}
	needsComponents := false
	for _, s := range loweredExprs {
		if strings.Contains(s, "JoinAttrs(") || strings.Contains(s, "Classes(") {
			needsComponents = true
			break
		}
	}
	return requiredImports{needsTags: true, needsComponents: needsComponents}
}

type importEditResult struct {
	out string
	mp  offsetMapper // maps old rewritten offsets <-> afterImports offsets
}

// applyImportEdits augments or inserts an import block with dot imports required for tags.
// It keeps the rest of the file unchanged.
func applyImportEdits(rewritten string, req requiredImports) importEditResult {
	if !req.needsTags {
		return importEditResult{out: rewritten}
	}

	want := []string{`. "maragu.dev/gomponents"`}
	if req.qualifyHTML {
		want = append(want, `html "maragu.dev/gomponents/html"`)
	} else {
		want = append(want, `. "maragu.dev/gomponents/html"`)
	}
	if req.needsComponents {
		want = append(want, `. "maragu.dev/gomponents/components"`)
	}

	// Find an existing import decl in a very conservative way: only within the leading region before the first "func".
	// This is a simple text approach but stable for mapping.
	leadEnd := len(rewritten)
	if i := strings.Index(rewritten, "\nfunc "); i >= 0 {
		leadEnd = i + 1
	}
	lead := rewritten[:leadEnd]

	// Group import block.
	if idx := strings.Index(lead, "\nimport (\n"); idx >= 0 {
		// Find matching closing ")\n".
		closeIdx := strings.Index(lead[idx:], "\n)\n")
		if closeIdx > 0 {
			closeIdx += idx
			blockStart := idx + 1 // includes leading '\n'
			blockEnd := closeIdx + 2
			block := rewritten[blockStart:blockEnd]
			lines := strings.Split(block, "\n")
			existing := map[string]bool{}
			for _, ln := range lines {
				existing[strings.TrimSpace(ln)] = true
			}
			var add []string
			for _, w := range want {
				if !existing[w] {
					add = append(add, "\t"+w)
				}
			}
			if len(add) == 0 {
				return importEditResult{out: rewritten}
			}
			ins := strings.Join(add, "\n") + "\n"
			// Insert before closing paren line.
			insertAt := blockEnd - 2 // before "\n)"
			out := rewritten[:insertAt] + ins + rewritten[insertAt:]
			return importEditResult{
				out: out,
				mp: offsetMapper{edits: []edit{{
					srcStart: insertAt, srcEnd: insertAt,
					tgtStart: insertAt, tgtEnd: insertAt + len(ins),
					mapTgtToSrcInside: false, mapSrcToTgtInside: true,
				}}},
			}
		}
	}

	// Single import line: `import "x"` or `import . "x"`.
	if idx := strings.Index(lead, "\nimport "); idx >= 0 {
		// Find end of line.
		lineEnd := strings.Index(lead[idx+1:], "\n")
		if lineEnd >= 0 {
			lineEnd = idx + 1 + lineEnd + 1
			line := rewritten[idx+1 : lineEnd]
			trim := strings.TrimSpace(line)
			// Convert to group import.
			var b strings.Builder
			b.WriteString("import (\n")
			b.WriteString("\t")
			b.WriteString(strings.TrimPrefix(trim, "import "))
			if !strings.HasSuffix(trim, "\n") {
				b.WriteString("\n")
			}
			for _, w := range want {
				b.WriteString("\t")
				b.WriteString(w)
				b.WriteString("\n")
			}
			b.WriteString(")\n")
			newBlock := b.String()
			out := rewritten[:idx+1] + newBlock + rewritten[lineEnd:]
			return importEditResult{
				out: out,
				mp: offsetMapper{edits: []edit{{
					srcStart: idx + 1, srcEnd: lineEnd,
					tgtStart: idx + 1, tgtEnd: idx + 1 + len(newBlock),
					mapTgtToSrcInside: false, mapSrcToTgtInside: true,
				}}},
			}
		}
	}

	// No imports: insert after "package <name>\n"
	pkgIdx := strings.Index(rewritten, "package ")
	if pkgIdx >= 0 {
		nl := strings.Index(rewritten[pkgIdx:], "\n")
		if nl >= 0 {
			insertAt := pkgIdx + nl + 1
			block := "\nimport (\n"
			for _, w := range want {
				block += "\t" + w + "\n"
			}
			block += ")\n"
			out := rewritten[:insertAt] + block + rewritten[insertAt:]
			return importEditResult{
				out: out,
				mp: offsetMapper{edits: []edit{{
					srcStart: insertAt, srcEnd: insertAt,
					tgtStart: insertAt, tgtEnd: insertAt + len(block),
					mapTgtToSrcInside: false, mapSrcToTgtInside: true,
				}}},
			}
		}
	}

	// Fallback: don't modify.
	return importEditResult{out: rewritten}
}

type expansionEditResult struct {
	out string
	mp  offsetMapper
}

// applyInlineExpansions replaces each placeholder call with a single-line inline func literal call:
//
//	func() Node { return <loweredExpr> }()
//
// It records each replacement as an edit.
func applyInlineExpansions(in string, placeholders []placeholder, loweredExprs map[string]string, mapTgtToSrcInside bool, mapSrcToTgtInside bool) expansionEditResult {
	if len(placeholders) == 0 {
		return expansionEditResult{out: in}
	}
	// Work in ascending order of placeholder call position.
	type phPos struct {
		start int
		end   int
		name  string
		// Source range in original.
		srcStart int
	}
	pp := make([]phPos, 0, len(placeholders))
	for _, p := range placeholders {
		pp = append(pp, phPos{start: p.tgtStart, end: p.tgtEnd, name: p.name, srcStart: p.srcStart})
	}
	sort.Slice(pp, func(i, j int) bool { return pp[i].start < pp[j].start })

	var out strings.Builder
	out.Grow(len(in) + len(placeholders)*16)
	var edits []edit

	cursor := 0
	delta := 0
	for _, p := range pp {
		if p.start < cursor || p.end > len(in) {
			continue
		}
		out.WriteString(in[cursor:p.start])
		repl := "func() Node { return " + loweredExprs[p.name] + " }()"
		tgtStart := out.Len()
		out.WriteString(repl)
		tgtEnd := out.Len()

		edits = append(edits, edit{
			srcStart: p.start, srcEnd: p.end,
			tgtStart: tgtStart - delta, tgtEnd: tgtEnd - delta,
			mapTgtToSrcInside: mapTgtToSrcInside,
			mapSrcToTgtInside: mapSrcToTgtInside,
		})
		// Update delta for subsequent adjusted mapping.
		delta += len(repl) - (p.end - p.start)
		cursor = p.end
	}
	out.WriteString(in[cursor:])
	return expansionEditResult{out: out.String(), mp: offsetMapper{edits: edits}}
}
