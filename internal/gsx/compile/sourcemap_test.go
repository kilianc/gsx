package compile

import (
	"testing"
)

func TestOffsetMapper_NoEdits(t *testing.T) {
	m := offsetMapper{}
	tgt, ok := m.tgtOffsetFromSrcOffset(10)
	if !ok || tgt != 10 {
		t.Fatalf("expected (10, true), got (%d, %v)", tgt, ok)
	}
	src, ok := m.srcOffsetFromTgtOffset(10)
	if !ok || src != 10 {
		t.Fatalf("expected (10, true), got (%d, %v)", src, ok)
	}
}

func TestOffsetMapper_SimpleReplacement(t *testing.T) {
	// src[5:8] ("abc", 3 bytes) replaced with tgt[5:10] ("ABCDE", 5 bytes)
	m := offsetMapper{edits: []edit{{
		srcStart: 5, srcEnd: 8,
		tgtStart: 5, tgtEnd: 10,
		mapSrcToTgtInside: true,
		mapTgtToSrcInside: true,
	}}}

	// Before the edit: identity.
	tgt, ok := m.tgtOffsetFromSrcOffset(3)
	if !ok || tgt != 3 {
		t.Fatalf("before edit: expected (3, true), got (%d, %v)", tgt, ok)
	}

	// Inside the edit: maps to tgtStart.
	tgt, ok = m.tgtOffsetFromSrcOffset(6)
	if !ok || tgt != 5 {
		t.Fatalf("inside edit: expected (5, true), got (%d, %v)", tgt, ok)
	}

	// After the edit: shifted by delta (+2).
	tgt, ok = m.tgtOffsetFromSrcOffset(10)
	if !ok || tgt != 12 {
		t.Fatalf("after edit: expected (12, true), got (%d, %v)", tgt, ok)
	}

	// Reverse: inside target edit maps to srcStart.
	src, ok := m.srcOffsetFromTgtOffset(7)
	if !ok || src != 5 {
		t.Fatalf("reverse inside: expected (5, true), got (%d, %v)", src, ok)
	}

	// Reverse: after target edit shifted back.
	src, ok = m.srcOffsetFromTgtOffset(12)
	if !ok || src != 10 {
		t.Fatalf("reverse after: expected (10, true), got (%d, %v)", src, ok)
	}
}

func TestOffsetMapper_InsideNotAllowed(t *testing.T) {
	m := offsetMapper{edits: []edit{{
		srcStart: 5, srcEnd: 8,
		tgtStart: 5, tgtEnd: 10,
		mapSrcToTgtInside: false,
		mapTgtToSrcInside: false,
	}}}

	_, ok := m.tgtOffsetFromSrcOffset(6)
	if ok {
		t.Fatal("expected mapping inside src to fail")
	}

	_, ok = m.srcOffsetFromTgtOffset(7)
	if ok {
		t.Fatal("expected mapping inside tgt to fail")
	}
}

func TestOffsetMapper_Insertion(t *testing.T) {
	// Zero-length source range = pure insertion at offset 10.
	// The inserted content is 10 bytes in target (tgt[10:20]).
	m := offsetMapper{edits: []edit{{
		srcStart: 10, srcEnd: 10,
		tgtStart: 10, tgtEnd: 20,
		mapSrcToTgtInside: true,
		mapTgtToSrcInside: false,
	}}}

	// At the insertion point: since srcStart==srcEnd, offset 10 is "after" the
	// edit and gets the full delta applied.
	tgt, ok := m.tgtOffsetFromSrcOffset(10)
	if !ok || tgt != 20 {
		t.Fatalf("at insertion: expected (20, true), got (%d, %v)", tgt, ok)
	}

	// Further after the insertion: also shifted by 10.
	tgt, ok = m.tgtOffsetFromSrcOffset(15)
	if !ok || tgt != 25 {
		t.Fatalf("after insertion: expected (25, true), got (%d, %v)", tgt, ok)
	}
}

func TestLineIndex_PosRoundTrip(t *testing.T) {
	text := "hello\nworld\nfoo"
	li := newLineIndex(text)

	tests := []struct {
		pos  Position
		want int
	}{
		{Position{0, 0}, 0},
		{Position{0, 4}, 4},
		{Position{1, 0}, 6},
		{Position{1, 3}, 9},
		{Position{2, 0}, 12},
		{Position{2, 2}, 14},
	}

	for _, tt := range tests {
		off, ok := li.offsetFromPos(tt.pos)
		if !ok {
			t.Fatalf("offsetFromPos(%v): expected ok", tt.pos)
		}
		if off != tt.want {
			t.Fatalf("offsetFromPos(%v): expected %d, got %d", tt.pos, tt.want, off)
		}

		pos, ok := li.posFromOffset(off)
		if !ok {
			t.Fatalf("posFromOffset(%d): expected ok", off)
		}
		if pos != tt.pos {
			t.Fatalf("posFromOffset(%d): expected %v, got %v", off, tt.pos, pos)
		}
	}
}

func TestLineIndex_OutOfBounds(t *testing.T) {
	li := newLineIndex("ab\ncd")

	if _, ok := li.offsetFromPos(Position{-1, 0}); ok {
		t.Fatal("expected failure for negative line")
	}
	if _, ok := li.offsetFromPos(Position{5, 0}); ok {
		t.Fatal("expected failure for line beyond end")
	}
	if _, ok := li.posFromOffset(-1); ok {
		t.Fatal("expected failure for negative offset")
	}
	if _, ok := li.posFromOffset(100); ok {
		t.Fatal("expected failure for offset beyond end")
	}
}

func TestSourceMap_EndToEnd(t *testing.T) {
	src := []byte(`package page

func Page() Node {
	title := "Hello"
	return <div class="main">{title}</div>
}
`)
	_, sm, err := CompileFileForLSP("page.gsx", src)
	if err != nil {
		t.Fatalf("CompileFileForLSP: %v", err)
	}

	// The package line (line 0, col 0) should round-trip through the sourcemap.
	pos := Position{Line: 0, Col: 0}
	tgt, ok := sm.TargetPositionFromSource(pos)
	if !ok {
		t.Fatal("TargetPositionFromSource(0,0) failed")
	}
	back, ok := sm.SourcePositionFromTarget(tgt)
	if !ok {
		t.Fatal("SourcePositionFromTarget round-trip failed")
	}
	if back != pos {
		t.Fatalf("round-trip: expected %v, got %v", pos, back)
	}

	// The `title := "Hello"` line (line 3) is pure Go and should round-trip.
	goCodePos := Position{Line: 3, Col: 1}
	tgt2, ok := sm.TargetPositionFromSource(goCodePos)
	if !ok {
		t.Fatal("TargetPositionFromSource(3,1) failed")
	}
	back2, ok := sm.SourcePositionFromTarget(tgt2)
	if !ok {
		t.Fatal("SourcePositionFromTarget round-trip for Go code failed")
	}
	if back2 != goCodePos {
		t.Fatalf("Go code round-trip: expected %v, got %v", goCodePos, back2)
	}
}

func TestSourceMap_NoTags(t *testing.T) {
	src := []byte(`package page

func Page() string {
	return "hello"
}
`)
	_, sm, err := CompileFileForLSP("notags.gsx", src)
	if err != nil {
		t.Fatalf("CompileFileForLSP: %v", err)
	}

	// With no tags, positions should pass through identity.
	pos := Position{Line: 2, Col: 5}
	tgt, ok := sm.TargetPositionFromSource(pos)
	if !ok {
		t.Fatal("TargetPositionFromSource failed")
	}
	if tgt != pos {
		t.Fatalf("expected identity mapping, got %v -> %v", pos, tgt)
	}
}
