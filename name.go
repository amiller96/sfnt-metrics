package main

import (
	"encoding/binary"
	"unicode/utf16"
)

// name table nameIDs we care about. 16/17 (typographic family/subfamily)
// were added later and are preferred over 1/2 when present, since 1/2 are
// constrained to fit the four classic style-linking groups (regular, bold,
// italic, bold italic) and get mangled on fonts with more weights than that.
const (
	nameIDFamily         = 1
	nameIDSubfamily      = 2
	nameIDTypographicFam = 16
	nameIDTypographicSub = 17
)

// parseNameTable pulls the family and style strings out of the name table.
// It is best-effort: any structural problem just yields empty strings
// rather than failing the whole parse, since the metrics we care about most
// live elsewhere.
func parseNameTable(data []byte, rec tableRecord) (family, style string) {
	b, err := slice(data, rec)
	if err != nil || len(b) < 6 {
		return "", ""
	}

	count := int(binary.BigEndian.Uint16(b[2:4]))
	stringOffset := int(binary.BigEndian.Uint16(b[4:6]))
	const recordsStart = 6
	const recordSize = 12
	recordsEnd := recordsStart + count*recordSize
	if count < 0 || len(b) < recordsEnd {
		return "", ""
	}

	bestScore := map[uint16]int{}
	candidate := map[uint16]string{}

	for i := 0; i < count; i++ {
		r := b[recordsStart+i*recordSize : recordsStart+(i+1)*recordSize]
		platformID := binary.BigEndian.Uint16(r[0:2])
		encodingID := binary.BigEndian.Uint16(r[2:4])
		languageID := binary.BigEndian.Uint16(r[4:6])
		nameID := binary.BigEndian.Uint16(r[6:8])
		length := int(binary.BigEndian.Uint16(r[8:10]))
		strOff := int(binary.BigEndian.Uint16(r[10:12]))

		if nameID != nameIDFamily && nameID != nameIDSubfamily &&
			nameID != nameIDTypographicFam && nameID != nameIDTypographicSub {
			continue
		}

		start := stringOffset + strOff
		end := start + length
		if start < 0 || end > len(b) || start > end {
			continue
		}

		sc := nameRecordScore(platformID, encodingID, languageID)
		if prev, ok := bestScore[nameID]; ok && sc <= prev {
			continue
		}
		bestScore[nameID] = sc
		candidate[nameID] = decodeNameBytes(b[start:end], platformID)
	}

	family = candidate[nameIDTypographicFam]
	if family == "" {
		family = candidate[nameIDFamily]
	}
	style = candidate[nameIDTypographicSub]
	if style == "" {
		style = candidate[nameIDSubfamily]
	}
	return family, style
}

// nameRecordScore ranks name records so that when a nameID has entries on
// multiple platforms we pick the one most likely to be readable and
// complete: Windows/Unicode BMP in US English first, then any Windows or
// Unicode entry, then Macintosh Roman English, then whatever is left.
func nameRecordScore(platformID, encodingID, languageID uint16) int {
	switch {
	case platformID == 3 && (encodingID == 1 || encodingID == 10) && languageID == 0x0409:
		return 100
	case platformID == 3:
		return 80
	case platformID == 0:
		return 70
	case platformID == 1 && encodingID == 0 && languageID == 0:
		return 60
	case platformID == 1:
		return 50
	default:
		return 10
	}
}

func decodeNameBytes(raw []byte, platformID uint16) string {
	switch platformID {
	case 3, 0:
		return decodeUTF16BE(raw)
	case 1:
		return decodeMacRoman(raw)
	default:
		// Unknown platform: most non-Mac, non-Windows entries in practice
		// are still UTF-16BE, so try that rather than returning nothing.
		return decodeUTF16BE(raw)
	}
}

func decodeUTF16BE(b []byte) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	units := make([]uint16, len(b)/2)
	for i := range units {
		units[i] = binary.BigEndian.Uint16(b[i*2 : i*2+2])
	}
	return string(utf16.Decode(units))
}

func decodeMacRoman(b []byte) string {
	runes := make([]rune, len(b))
	for i, c := range b {
		if c < 0x80 {
			runes[i] = rune(c)
		} else {
			runes[i] = macRomanHighBytes[c-0x80]
		}
	}
	return string(runes)
}

// macRomanHighBytes maps bytes 0x80-0xFF of the Mac Roman encoding to their
// Unicode code points. Bytes 0x00-0x7F are plain ASCII. 0xF0 is the Apple
// logo, which has no standard Unicode mapping outside Apple's own private
// use area assignment.
var macRomanHighBytes = [128]rune{
	'Ä', 'Å', 'Ç', 'É', 'Ñ', 'Ö', 'Ü', 'á', 'à', 'â', 'ä', 'ã', 'å', 'ç', 'é', 'è',
	'ê', 'ë', 'í', 'ì', 'î', 'ï', 'ñ', 'ó', 'ò', 'ô', 'ö', 'õ', 'ú', 'ù', 'û', 'ü',
	'†', '°', '¢', '£', '§', '•', '¶', 'ß', '®', '©', '™', '´', '¨', '≠', 'Æ', 'Ø',
	'∞', '±', '≤', '≥', '¥', 'µ', '∂', '∑', '∏', 'π', '∫', 'ª', 'º', 'Ω', 'æ', 'ø',
	'¿', '¡', '¬', '√', 'ƒ', '≈', '∆', '«', '»', '…', ' ', 'À', 'Ã', 'Õ', 'Œ', 'œ',
	'–', '—', '“', '”', '‘', '’', '÷', '◊', 'ÿ', 'Ÿ', '⁄', '€', '‹', '›', 'ﬁ', 'ﬂ',
	'‡', '·', '‚', '„', '‰', 'Â', 'Ê', 'Á', 'Ë', 'È', 'Í', 'Î', 'Ï', 'Ì', 'Ó', 'Ô',
	'', 'Ò', 'Ú', 'Û', 'Ù', 'ı', 'ˆ', '˜', '¯', '˘', '˙', '˚', '¸', '˝', '˛', 'ˇ',
}
