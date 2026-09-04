package main

import (
	"encoding/binary"
	"sort"
	"testing"
)

// buildHead returns a minimal head table with the fields Parse reads:
// unitsPerEm at offset 18 and macStyle at offset 44.
func buildHead(unitsPerEm uint16, italic bool) []byte {
	b := make([]byte, 54)
	binary.BigEndian.PutUint16(b[18:20], unitsPerEm)
	if italic {
		binary.BigEndian.PutUint16(b[44:46], 0x02)
	}
	return b
}

// buildHhea returns a minimal hhea table with ascender, descender, and
// lineGap, the only fields Parse reads.
func buildHhea(ascender, descender, lineGap int16) []byte {
	b := make([]byte, 10)
	binary.BigEndian.PutUint16(b[4:6], uint16(ascender))
	binary.BigEndian.PutUint16(b[6:8], uint16(descender))
	binary.BigEndian.PutUint16(b[8:10], uint16(lineGap))
	return b
}

// buildMaxp returns a minimal maxp table with numGlyphs.
func buildMaxp(numGlyphs uint16) []byte {
	b := make([]byte, 6)
	binary.BigEndian.PutUint16(b[4:6], numGlyphs)
	return b
}

// buildSFNT assembles a bare sfnt font (table directory plus tables) from
// the given tag and table contents. It does not compute real checksums or
// search ranges, since Parse never reads them.
func buildSFNT(versionTag uint32, tables map[string][]byte) []byte {
	tags := make([]string, 0, len(tables))
	for tag := range tables {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	numTables := len(tags)
	const dirEntrySize = 16
	headerSize := 12 + numTables*dirEntrySize

	out := make([]byte, headerSize)
	binary.BigEndian.PutUint32(out[0:4], versionTag)
	binary.BigEndian.PutUint16(out[4:6], uint16(numTables))

	offset := uint32(headerSize)
	for i, tag := range tags {
		body := tables[tag]
		entry := out[12+i*dirEntrySize : 12+(i+1)*dirEntrySize]
		copy(entry[0:4], tag)
		binary.BigEndian.PutUint32(entry[8:12], offset)
		binary.BigEndian.PutUint32(entry[12:16], uint32(len(body)))
		out = append(out, body...)
		offset += uint32(len(body))
	}
	return out
}

func minimalTables(unitsPerEm uint16, numGlyphs uint16) map[string][]byte {
	return map[string][]byte{
		"head": buildHead(unitsPerEm, false),
		"hhea": buildHhea(1900, -500, 0),
		"maxp": buildMaxp(numGlyphs),
	}
}

func buildTTC(fonts ...[]byte) []byte {
	headerSize := ttcHeaderSize + len(fonts)*4
	out := make([]byte, headerSize)
	binary.BigEndian.PutUint32(out[0:4], tagCollection)
	binary.BigEndian.PutUint32(out[4:8], 0x00010000)
	binary.BigEndian.PutUint32(out[8:12], uint32(len(fonts)))

	offset := uint32(headerSize)
	for i, f := range fonts {
		binary.BigEndian.PutUint32(out[ttcHeaderSize+i*4:ttcHeaderSize+i*4+4], offset)
		out = append(out, f...)
		offset += uint32(len(f))
	}
	return out
}

func TestParseBareFont(t *testing.T) {
	data := buildSFNT(tagTrueType, minimalTables(2048, 500))

	m, err := Parse(data, 0)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.UnitsPerEm != 2048 {
		t.Errorf("UnitsPerEm = %d, want 2048", m.UnitsPerEm)
	}
	if m.NumGlyphs != 500 {
		t.Errorf("NumGlyphs = %d, want 500", m.NumGlyphs)
	}
	if m.Format != "TrueType" {
		t.Errorf("Format = %q, want TrueType", m.Format)
	}
}

func TestParseBareFontRejectsNonZeroIndex(t *testing.T) {
	data := buildSFNT(tagTrueType, minimalTables(2048, 500))

	if _, err := Parse(data, 1); err == nil {
		t.Fatal("Parse with index 1 on a bare font: got nil error, want an error")
	}
}

func TestParseCollection(t *testing.T) {
	fontA := buildSFNT(tagTrueType, minimalTables(1000, 100))
	fontB := buildSFNT(tagOpenType, minimalTables(2048, 200))
	data := buildTTC(fontA, fontB)

	m0, err := Parse(data, 0)
	if err != nil {
		t.Fatalf("Parse(data, 0): %v", err)
	}
	if m0.UnitsPerEm != 1000 || m0.Format != "TrueType" {
		t.Errorf("font 0 = %+v, want unitsPerEm=1000 format=TrueType", m0)
	}

	m1, err := Parse(data, 1)
	if err != nil {
		t.Fatalf("Parse(data, 1): %v", err)
	}
	if m1.UnitsPerEm != 2048 || m1.Format != "CFF (OpenType)" {
		t.Errorf("font 1 = %+v, want unitsPerEm=2048 format=CFF (OpenType)", m1)
	}
}

func TestParseCollectionIndexOutOfRange(t *testing.T) {
	data := buildTTC(buildSFNT(tagTrueType, minimalTables(1000, 100)))

	if _, err := Parse(data, 5); err == nil {
		t.Fatal("Parse with out-of-range index: got nil error, want an error")
	}
}

func TestParseTooSmall(t *testing.T) {
	if _, err := Parse([]byte{0, 1, 2}, 0); err == nil {
		t.Fatal("Parse on truncated data: got nil error, want an error")
	}
}
