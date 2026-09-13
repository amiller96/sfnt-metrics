package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"os"
	"testing"
	"unicode/utf16"
)

// update regenerates the golden files from the current output instead of
// checking against them. Run with `go test -update` after a deliberate
// change to Metrics or its JSON encoding.
var update = flag.Bool("update", false, "update golden files")

// buildOS2 returns an OS/2 version 2 table with the fields applyOS2 reads.
// Version 2 is used (rather than 0 or 1) so the golden test also exercises
// the sxHeight/sCapHeight fields that only exist from version 2 onward.
func buildOS2(weightClass, widthClass uint16, italic bool, typoAscender, typoDescender, typoLineGap int16, winAscent, winDescent uint16, xHeight, capHeight int16) []byte {
	b := make([]byte, 90)
	binary.BigEndian.PutUint16(b[0:2], 2)
	binary.BigEndian.PutUint16(b[4:6], weightClass)
	binary.BigEndian.PutUint16(b[6:8], widthClass)
	var fsSelection uint16
	if italic {
		fsSelection |= 0x01
	}
	binary.BigEndian.PutUint16(b[62:64], fsSelection)
	binary.BigEndian.PutUint16(b[68:70], uint16(typoAscender))
	binary.BigEndian.PutUint16(b[70:72], uint16(typoDescender))
	binary.BigEndian.PutUint16(b[72:74], uint16(typoLineGap))
	binary.BigEndian.PutUint16(b[74:76], winAscent)
	binary.BigEndian.PutUint16(b[76:78], winDescent)
	binary.BigEndian.PutUint16(b[86:88], uint16(xHeight))
	binary.BigEndian.PutUint16(b[88:90], uint16(capHeight))
	return b
}

// buildNameTable returns a name table with a single Windows/US-English
// record for the family (nameID 1) and style (nameID 2) strings, encoded as
// parseNameTable expects platform 3 entries to be: UTF-16BE.
func buildNameTable(family, style string) []byte {
	type record struct {
		nameID uint16
		data   []byte
	}
	records := []record{
		{1, utf16beBytes(family)},
		{2, utf16beBytes(style)},
	}

	const recordsStart = 6
	const recordSize = 12
	stringOffset := recordsStart + len(records)*recordSize

	header := make([]byte, stringOffset)
	binary.BigEndian.PutUint16(header[2:4], uint16(len(records)))
	binary.BigEndian.PutUint16(header[4:6], uint16(stringOffset))

	var strings []byte
	for i, r := range records {
		e := header[recordsStart+i*recordSize : recordsStart+(i+1)*recordSize]
		binary.BigEndian.PutUint16(e[0:2], 3)      // platformID: Windows
		binary.BigEndian.PutUint16(e[2:4], 1)      // encodingID: Unicode BMP
		binary.BigEndian.PutUint16(e[4:6], 0x0409) // languageID: US English
		binary.BigEndian.PutUint16(e[6:8], r.nameID)
		binary.BigEndian.PutUint16(e[8:10], uint16(len(r.data)))
		binary.BigEndian.PutUint16(e[10:12], uint16(len(strings)))
		strings = append(strings, r.data...)
	}
	return append(header, strings...)
}

func utf16beBytes(s string) []byte {
	units := utf16.Encode([]rune(s))
	b := make([]byte, len(units)*2)
	for i, u := range units {
		binary.BigEndian.PutUint16(b[i*2:i*2+2], u)
	}
	return b
}

// compareGolden marshals m the same way the CLI's --json output does and
// checks it against the checked-in file at path, or rewrites that file when
// -update is passed.
func compareGolden(t *testing.T, path string, m *Metrics) {
	t.Helper()

	got, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("marshaling metrics: %v", err)
	}
	got = append(got, '\n')

	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file: %v (run with -update to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s: output does not match golden file\ngot:\n%s\nwant:\n%s", path, got, want)
	}
}

// TestGoldenFull covers a font with every table Parse understands present at
// once: OS/2 version 2 (so cap-height and x-height are populated) and a name
// table with both family and style.
func TestGoldenFull(t *testing.T) {
	tables := map[string][]byte{
		"head": buildHead(1000, false),
		"hhea": buildHhea(1899, -501, 0),
		"maxp": buildMaxp(300),
		"OS/2": buildOS2(700, 5, true, 1901, -499, 100, 1950, 550, 520, 700),
		"name": buildNameTable("Golden Test Sans", "Bold Italic"),
	}
	data := buildSFNT(tagTrueType, tables)

	m, err := Parse(data, 0)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	compareGolden(t, "testdata/full.golden.json", m)
}

// TestGoldenMinimal covers the opposite end: no OS/2 and no name table, so
// italic comes from head's macStyle and every OS/2-derived field is absent
// from the JSON rather than present as a zero value.
func TestGoldenMinimal(t *testing.T) {
	tables := map[string][]byte{
		"head": buildHead(2048, true),
		"hhea": buildHhea(1854, -434, 67),
		"maxp": buildMaxp(10),
	}
	data := buildSFNT(tagTrueType, tables)

	m, err := Parse(data, 0)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	compareGolden(t, "testdata/minimal.golden.json", m)
}
