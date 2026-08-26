# sfnt-metrics

A command-line tool that reads a TrueType or OpenType font file and prints
its font-wide metrics: units per em, ascender/descender, line gap, weight
and width class, cap height, x-height, and glyph count.

Font metrics live in a handful of binary tables inside the font file
(`head`, `hhea`, `maxp`, `OS/2`), and most of the time getting at them means
either opening the font in a design tool or pulling in a full font-parsing
library just to read a few numbers. This tool parses the sfnt table
directory directly and reads only what it needs, so it has no dependencies
beyond the Go standard library.

## Usage

```
$ go build -o sfnt-metrics .

$ ./sfnt-metrics ./testdata/Inter-Regular.ttf
./testdata/Inter-Regular.ttf
  format          TrueType
  units per em    2048
  glyphs          3892
  ascender        1984
  descender       -494
  line gap        0
  weight class    400
  width class     5
  italic          false
  typo ascender   1901
  typo descender  -483
  typo line gap   0
  win ascent      2189
  win descent     600
  cap height      1490
  x-height        1096
```

Pass `--json` for machine-readable output:

```
$ ./sfnt-metrics --json ./testdata/Inter-Regular.ttf
{
  "format": "TrueType",
  "unitsPerEm": 2048,
  "numGlyphs": 3892,
  "ascender": 1984,
  "descender": -494,
  "lineGap": 0,
  "weightClass": 400,
  "widthClass": 5,
  "italic": false,
  "typoAscender": 1901,
  "typoDescender": -483,
  "typoLineGap": 0,
  "winAscent": 2189,
  "winDescent": 600,
  "capHeight": 1490,
  "xHeight": 1096
}
```

Fields that come from the `OS/2` table (`typoAscender` through `xHeight`)
are only present when that table exists and, for `capHeight`/`xHeight`, when
its version is 2 or higher. On fonts without them the fields are omitted
from JSON and printed as `n/a` in text mode.

## Current limitations

- Font collections (`.ttc` / `.otc`) are rejected, not read.
- Family and style names (the `name` table) aren't extracted yet.
- One file per invocation.

## License

MIT, see [LICENSE](LICENSE).
