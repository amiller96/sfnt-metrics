package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

// fileResult pairs a parsed font's metrics with the path it came from, so
// batch JSON output can tell results apart. The single-file case skips this
// wrapper and encodes the bare Metrics, keeping that output unchanged.
type fileResult struct {
	Path    string   `json:"path"`
	Metrics *Metrics `json:"metrics"`
}

func main() {
	jsonOut := flag.Bool("json", false, "emit machine-readable JSON instead of a text summary")
	fontIndex := flag.Int("font-index", 0, "index of the font to read within a collection (.ttc/.otc)")
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() < 1 {
		usage()
		os.Exit(2)
	}

	paths := flag.Args()
	results := make([]fileResult, 0, len(paths))
	exitCode := 0

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sfnt-metrics: %v\n", err)
			exitCode = 1
			continue
		}

		m, err := Parse(data, *fontIndex)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sfnt-metrics: %s: %v\n", path, err)
			exitCode = 1
			continue
		}

		results = append(results, fileResult{Path: path, Metrics: m})
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		var err error
		switch {
		case len(paths) == 1:
			if len(results) == 1 {
				err = enc.Encode(results[0].Metrics)
			}
		default:
			err = enc.Encode(results)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "sfnt-metrics: %v\n", err)
			os.Exit(1)
		}
	} else {
		for i, r := range results {
			if i > 0 {
				fmt.Println()
			}
			printText(r.Metrics, r.Path)
		}
	}

	os.Exit(exitCode)
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: sfnt-metrics [--json] [--font-index N] <font-file>...\n\nReads one or more TrueType or OpenType font files and prints their\nfont-wide metrics. With --json and more than one file, output is a JSON\narray of {\"path\", \"metrics\"} objects instead of a single object.\nFor a font collection (.ttc/.otc), --font-index selects which font to\nread, and applies to every file given.\n")
}

func printText(m *Metrics, path string) {
	fmt.Printf("%s\n", path)
	fmt.Printf("  format          %s\n", m.Format)
	printOptionalString("family", m.Family)
	printOptionalString("style", m.Style)
	fmt.Printf("  units per em    %d\n", m.UnitsPerEm)
	fmt.Printf("  glyphs          %d\n", m.NumGlyphs)
	fmt.Printf("  ascender        %d\n", m.Ascender)
	fmt.Printf("  descender       %d\n", m.Descender)
	fmt.Printf("  line gap        %d\n", m.LineGap)
	fmt.Printf("  weight class    %d\n", m.WeightClass)
	fmt.Printf("  width class     %d\n", m.WidthClass)
	fmt.Printf("  italic          %t\n", m.Italic)
	printOptionalInt16("typo ascender", m.TypoAscender)
	printOptionalInt16("typo descender", m.TypoDescender)
	printOptionalInt16("typo line gap", m.TypoLineGap)
	printOptionalUint16("win ascent", m.WinAscent)
	printOptionalUint16("win descent", m.WinDescent)
	printOptionalInt16("cap height", m.CapHeight)
	printOptionalInt16("x-height", m.XHeight)
}

func printOptionalString(label, v string) {
	if v == "" {
		fmt.Printf("  %-15s n/a\n", label)
		return
	}
	fmt.Printf("  %-15s %s\n", label, v)
}

func printOptionalInt16(label string, v *int16) {
	if v == nil {
		fmt.Printf("  %-15s n/a\n", label)
		return
	}
	fmt.Printf("  %-15s %d\n", label, *v)
}

func printOptionalUint16(label string, v *uint16) {
	if v == nil {
		fmt.Printf("  %-15s n/a\n", label)
		return
	}
	fmt.Printf("  %-15s %d\n", label, *v)
}
