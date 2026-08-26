package main

// Metrics is the set of font-wide values a layout engine or type designer
// cares about. The OS/2 table was added after TrueType 1.0 and grew fields
// over several revisions, so anything that only exists in OS/2 version 1 or
// 2 is a pointer here and left nil (and dropped from JSON) on older fonts.
type Metrics struct {
	Format      string `json:"format"`
	UnitsPerEm  uint16 `json:"unitsPerEm"`
	NumGlyphs   uint16 `json:"numGlyphs"`
	Ascender    int16  `json:"ascender"`
	Descender   int16  `json:"descender"`
	LineGap     int16  `json:"lineGap"`
	WeightClass uint16 `json:"weightClass"`
	WidthClass  uint16 `json:"widthClass"`
	Italic      bool   `json:"italic"`

	TypoAscender  *int16  `json:"typoAscender,omitempty"`
	TypoDescender *int16  `json:"typoDescender,omitempty"`
	TypoLineGap   *int16  `json:"typoLineGap,omitempty"`
	WinAscent     *uint16 `json:"winAscent,omitempty"`
	WinDescent    *uint16 `json:"winDescent,omitempty"`
	CapHeight     *int16  `json:"capHeight,omitempty"`
	XHeight       *int16  `json:"xHeight,omitempty"`
}
