package main

import (
	"encoding/binary"
	"fmt"
)

// sfnt version tags, as big-endian uint32s of the four-byte magic at the
// start of the file. See the OpenType spec, "Organization of an OpenType
// Font".
const (
	tagTrueType   = 0x00010000
	tagOpenType   = 0x4F54544F // "OTTO", CFF outlines
	tagTrue       = 0x74727565 // "true", old-style Mac TrueType
	tagCollection = 0x74746366 // "ttcf"
)

type tableRecord struct {
	offset uint32
	length uint32
}

// ttcHeaderSize is the length, in bytes, of the fixed part of a TTC header:
// tag, version, and numFonts. It is followed by numFonts uint32 offsets,
// one per font in the collection.
const ttcHeaderSize = 12

// Parse reads a single sfnt font out of data. If data is a bare font file,
// index must be 0. If data is a font collection (.ttc/.otc), index selects
// which of its fonts to read.
func Parse(data []byte, index int) (*Metrics, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("file too small to be a font")
	}

	version := binary.BigEndian.Uint32(data[0:4])
	base := uint32(0)
	if version == tagCollection {
		off, err := ttcFontOffset(data, index)
		if err != nil {
			return nil, err
		}
		base = off
	} else if index != 0 {
		return nil, fmt.Errorf("font index %d requested but file is not a font collection", index)
	}

	return parseSFNT(data, base)
}

// ttcFontOffset reads the TTC header and returns the file offset of the
// sfnt table directory for the font at index.
func ttcFontOffset(data []byte, index int) (uint32, error) {
	if len(data) < ttcHeaderSize {
		return 0, fmt.Errorf("truncated ttc header")
	}
	numFonts := int(binary.BigEndian.Uint32(data[8:12]))
	if index < 0 || index >= numFonts {
		return 0, fmt.Errorf("font index %d out of range (collection has %d fonts)", index, numFonts)
	}
	entryOffset := ttcHeaderSize + index*4
	if len(data) < entryOffset+4 {
		return 0, fmt.Errorf("truncated ttc directory")
	}
	return binary.BigEndian.Uint32(data[entryOffset : entryOffset+4]), nil
}

// parseSFNT reads the sfnt table directory starting at base and pulls the
// handful of tables (head, hhea, maxp, OS/2) that carry font-wide metrics.
// It does not touch glyph outlines, cmap, or naming data beyond the name
// table itself. base is 0 for a bare font file and the per-font offset
// found in the TTC header for a font collection; table offsets within the
// directory are always absolute from the start of the file either way.
func parseSFNT(data []byte, base uint32) (*Metrics, error) {
	if uint64(base)+12 > uint64(len(data)) {
		return nil, fmt.Errorf("truncated font in collection")
	}
	start := int(base)

	version := binary.BigEndian.Uint32(data[start : start+4])
	var format string
	switch version {
	case tagTrueType, tagTrue:
		format = "TrueType"
	case tagOpenType:
		format = "CFF (OpenType)"
	default:
		return nil, fmt.Errorf("not a recognized sfnt font (unexpected version tag)")
	}

	numTables := int(binary.BigEndian.Uint16(data[start+4 : start+6]))
	const dirEntrySize = 16
	dirStart := start + 12
	dirEnd := dirStart + numTables*dirEntrySize
	if numTables < 0 || len(data) < dirEnd {
		return nil, fmt.Errorf("truncated table directory")
	}

	tables := make(map[string]tableRecord, numTables)
	for i := 0; i < numTables; i++ {
		rec := data[dirStart+i*dirEntrySize : dirStart+(i+1)*dirEntrySize]
		tag := string(rec[0:4])
		tables[tag] = tableRecord{
			offset: binary.BigEndian.Uint32(rec[8:12]),
			length: binary.BigEndian.Uint32(rec[12:16]),
		}
	}

	head, ok := tables["head"]
	if !ok {
		return nil, fmt.Errorf("missing required head table")
	}
	hhea, ok := tables["hhea"]
	if !ok {
		return nil, fmt.Errorf("missing required hhea table")
	}
	maxp, ok := tables["maxp"]
	if !ok {
		return nil, fmt.Errorf("missing required maxp table")
	}

	headBytes, err := slice(data, head)
	if err != nil {
		return nil, fmt.Errorf("head table: %w", err)
	}
	if len(headBytes) < 54 {
		return nil, fmt.Errorf("head table: too short")
	}
	unitsPerEm := binary.BigEndian.Uint16(headBytes[18:20])
	macStyle := binary.BigEndian.Uint16(headBytes[44:46])

	hheaBytes, err := slice(data, hhea)
	if err != nil {
		return nil, fmt.Errorf("hhea table: %w", err)
	}
	if len(hheaBytes) < 10 {
		return nil, fmt.Errorf("hhea table: too short")
	}
	ascender := int16(binary.BigEndian.Uint16(hheaBytes[4:6]))
	descender := int16(binary.BigEndian.Uint16(hheaBytes[6:8]))
	lineGap := int16(binary.BigEndian.Uint16(hheaBytes[8:10]))

	maxpBytes, err := slice(data, maxp)
	if err != nil {
		return nil, fmt.Errorf("maxp table: %w", err)
	}
	if len(maxpBytes) < 6 {
		return nil, fmt.Errorf("maxp table: too short")
	}
	numGlyphs := binary.BigEndian.Uint16(maxpBytes[4:6])

	m := &Metrics{
		Format:     format,
		UnitsPerEm: unitsPerEm,
		NumGlyphs:  numGlyphs,
		Ascender:   ascender,
		Descender:  descender,
		LineGap:    lineGap,
		// bit 1 of macStyle is "italic"; OS/2, when present, is more
		// specific and overrides this below.
		Italic: macStyle&0x02 != 0,
	}

	if os2, ok := tables["OS/2"]; ok {
		if err := applyOS2(data, os2, m); err != nil {
			return nil, fmt.Errorf("OS/2 table: %w", err)
		}
	}

	if name, ok := tables["name"]; ok {
		m.Family, m.Style = parseNameTable(data, name)
	}

	return m, nil
}

// applyOS2 fills in the metrics that only live in the OS/2 table. Versions
// 0 and 1 stop at usWinDescent; version 2 and later add sxHeight/sCapHeight,
// which is why those two fields are conditional on the table's own version
// field rather than just its length.
func applyOS2(data []byte, rec tableRecord, m *Metrics) error {
	b, err := slice(data, rec)
	if err != nil {
		return err
	}
	if len(b) < 78 {
		return fmt.Errorf("too short")
	}

	os2Version := binary.BigEndian.Uint16(b[0:2])
	m.WeightClass = binary.BigEndian.Uint16(b[4:6])
	m.WidthClass = binary.BigEndian.Uint16(b[6:8])

	fsSelection := binary.BigEndian.Uint16(b[62:64])
	if fsSelection&0x01 != 0 {
		m.Italic = true
	}

	typoAscender := int16(binary.BigEndian.Uint16(b[68:70]))
	typoDescender := int16(binary.BigEndian.Uint16(b[70:72]))
	typoLineGap := int16(binary.BigEndian.Uint16(b[72:74]))
	winAscent := binary.BigEndian.Uint16(b[74:76])
	winDescent := binary.BigEndian.Uint16(b[76:78])
	m.TypoAscender = &typoAscender
	m.TypoDescender = &typoDescender
	m.TypoLineGap = &typoLineGap
	m.WinAscent = &winAscent
	m.WinDescent = &winDescent

	if os2Version >= 2 && len(b) >= 90 {
		xHeight := int16(binary.BigEndian.Uint16(b[86:88]))
		capHeight := int16(binary.BigEndian.Uint16(b[88:90]))
		m.XHeight = &xHeight
		m.CapHeight = &capHeight
	}

	return nil
}

func slice(data []byte, rec tableRecord) ([]byte, error) {
	start := int(rec.offset)
	end := start + int(rec.length)
	if start < 0 || end > len(data) || start > end {
		return nil, fmt.Errorf("out of bounds (offset=%d length=%d file=%d bytes)", rec.offset, rec.length, len(data))
	}
	return data[start:end], nil
}
