package formatting

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// PrettyPrintTable prints a table (slice of rows, each a slice of string)
// maxWidths is an optional slice of maximum widths for each column.
// Pass nil (or a slice of different length) to skip max-width limitations.
func PrettyPrintTable(rows [][]string, maxWidths []int, header ...[]string) {
	var headerRow []string
	if len(header) > 0 {
		headerRow = header[0]
	}

	if len(rows) == 0 && headerRow == nil {
		return
	}

	numColumns := 0
	if headerRow != nil {
		numColumns = len(headerRow)
	} else if len(rows) > 0 {
		numColumns = len(rows[0])
	}

	if numColumns == 0 {
		return
	}
	// Sort rows by the first column.
	sort.Slice(rows, func(i, j int) bool {
		return strings.ToLower(rows[i][0]) < strings.ToLower(rows[j][0])
	})
	colWidths := make([]int, numColumns)

	for i, cell := range headerRow {
		if w := runeWidth(cell); w > colWidths[i] {
			colWidths[i] = w
		}
	}

	for _, row := range rows {
		for i, cell := range row {
			if i < numColumns {
				if w := runeWidth(cell); w > colWidths[i] {
					colWidths[i] = w
				}
			}
		}
	}
	// If provided, limit the width per column.
	if maxWidths != nil && len(maxWidths) == numColumns {
		for i := range colWidths {
			colWidths[i] = intMin(colWidths[i], maxWidths[i])
		}
	}

	printRow := func(row []string) {
		for i, cell := range row {
			if i >= numColumns {
				break
			}
			// Truncate the cell to the allowed width.
			s := truncateString(cell, colWidths[i])
			// Pad the cell so that each column aligns.
			s += spaces(colWidths[i] - runeWidth(s))
			fmt.Print(s)
			if i < numColumns-1 {
				fmt.Print(" | ")
			}
		}
		fmt.Println()
	}
	if headerRow != nil {
		printRow(headerRow)
	}
	for _, row := range rows {
		printRow(row)
	}
}

func intMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func spaces(n int) string {
	return strings.Repeat(" ", n)
}

func runeWidth(s string) int {
	return utf8.RuneCountInString(s)
}

// truncateString shortens s to fit the given width, appending an ellipsis if possible.
func truncateString(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width > 1 {
		return string(runes[:width-1]) + "…"
	}
	return string(runes[:width])
}
