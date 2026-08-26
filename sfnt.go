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

// Parse reads the sfnt table directory out of data and pulls the handful of
// tables (head, hhea, maxp, OS/2) that carry font-wide metrics. It does not
// touch glyph outlines, cmap, or naming data.
func Parse(data []byte) (*Metrics, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("file too small to be a font")
	}

	version := binary.BigEndian.Uint32(data[0:4])
	if version == tagCollection {
		return nil, fmt.Errorf("font collections (.ttc/.otc) are not supported yet")
	}

	var format string
	switch version {
	case tagTrueType, tagTrue:
		format = "TrueType"
	case tagOpenType:
		format = "CFF (OpenType)"
	default:
		return nil, fmt.Errorf("not a recognized sfnt font (unexpected version tag)")
	}

	numTables := int(binary.BigEndian.Uint16(data[4:6]))
	const dirEntrySize = 16
	dirEnd := 12 + numTables*dirEntrySize
	if numTables < 0 || len(data) < dirEnd {
		return nil, fmt.Errorf("truncated table directory")
	}

	tables := make(map[string]tableRecord, numTables)
	for i := 0; i < numTables; i++ {
		rec := data[12+i*dirEntrySize : 12+(i+1)*dirEntrySize]
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
