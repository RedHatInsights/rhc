package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"golang.org/x/sys/unix"
	"golang.org/x/text/width"
)

const (
	colorGreen  = "\u001B[32m"
	colorYellow = "\u001B[33m"
	colorRed    = "\u001B[31m"
	colorReset  = "\u001B[0m"
)

var Indent = indent{
	Small:  " ",
	Medium: "  ",
}

type indent struct {
	Small  string
	Medium string
}

type icons struct {
	Ok    string
	Info  string
	Error string
}

var Icons icons
var isOutputRich bool
var isOutputMachineReadable bool

func init() {
	// Default to colored and animated terminal experience
	ConfigureOutput(true, true, false)
}

// IsInteractive returns true if the standard output is a terminal.
func IsInteractive() bool {
	return isTerminal(os.Stdout.Fd())
}

// isTerminal returns true if the file descriptor is a terminal.
func isTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
	return err == nil
}

// ConfigureOutput sets up global state for communicating information to the user.
// 'rich' represents output's ability to display animations or colors,
// 'colored' represents user's preference to display colors, and requires 'rich' to be true,
// 'machine' is true when the output is formatted as JSON or similar machine-readable format.
func ConfigureOutput(rich bool, colored bool, machine bool) {
	if machine {
		isOutputMachineReadable = true
		isOutputRich = false
	}
	if rich {
		isOutputRich = true
	}

	Icons = icons{
		Ok:    "✓",
		Info:  "●",
		Error: "𐄂",
	}
	if rich && colored {
		Icons.Ok = colorGreen + Icons.Ok + colorReset
		Icons.Info = colorYellow + Icons.Info + colorReset
		Icons.Error = colorRed + Icons.Error + colorReset
	}
}

// IsOutputMachineReadable returns true when the output should be formatted as
// JSON or similar machine-readable format.
func IsOutputMachineReadable() bool {
	return isOutputMachineReadable
}

// IsOutputRich returns true when the output should be displayed in a terminal
// supporting animations and colors.
func IsOutputRich() bool {
	return isOutputRich
}

// Printf acts as a no-op if the output is machine-readable.
// Otherwise, passes the input to fmt.Printf.
func Printf(
	format string,
	a ...interface{},
) {
	if IsOutputMachineReadable() {
		return
	}
	fmt.Printf(format, a...)
}

// displayWidth returns the display width of a string in terminal columns.
// It properly handles wide characters (CJK, emojis) that take 2 columns,
// while narrow characters (ASCII, most symbols) take 1 column.
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		kind := width.LookupRune(r).Kind()
		switch kind {
		case width.EastAsianWide, width.EastAsianFullwidth:
			w += 2
		default:
			w += 1
		}
	}
	return w
}

// truncateString truncates a string to the specified display width and appends "..."
// It handles UTF-8 properly by truncating at rune boundaries and accounts for
// wide characters that take 2 display columns.
func truncateString(s string, maxWidth int) string {
	if displayWidth(s) <= maxWidth {
		return s
	}
	// We need to truncate to maxWidth-3 to leave room for "..."
	targetWidth := maxWidth - 3
	if targetWidth < 0 {
		targetWidth = 0
	}

	// Iterate through runes to find the truncation point
	currentWidth := 0
	truncated := strings.Builder{}
	for _, r := range s {
		runeWidth := displayWidth(string(r))
		if currentWidth+runeWidth > targetWidth {
			break
		}
		truncated.WriteRune(r)
		currentWidth += runeWidth
	}
	return truncated.String() + "..."
}

// PrintTable prints a left-justified table and expects the table to be a 2D slice of strings.
// Each column is left-justified so that all values in a column align with the column header.
// The function properly handles UTF-8 characters including wide characters (CJK, emojis)
// that take 2 display columns, ensuring proper alignment regardless of character width.
// Rows that exceed termWidth are truncated and end with "..."
func PrintTable(table [][]string, sep string, termWidth int) {
	// validate table, table must include at least one row and one column
	if len(table) == 0 || len(table[0]) == 0 {
		return
	}

	// Find the maximum display width for each column
	columnWidths := make([]int, len(table[0]))
	for _, row := range table {
		for col, cell := range row {
			cellWidth := displayWidth(cell)
			if cellWidth > columnWidths[col] {
				columnWidths[col] = cellWidth
			}
		}
	}

	// Build and print each row
	tableOutput := ""
	for _, row := range table {
		rowString := ""
		for col, cell := range row {
			// Add the cell content
			rowString += cell

			// If not the last column, add padding and separator
			if col < len(row)-1 {
				cellWidth := displayWidth(cell)
				// Padding fills the gap between cell width and column width
				padding := columnWidths[col] - cellWidth
				if padding > 0 {
					rowString += strings.Repeat(" ", padding)
				}
				// Add separator after all columns except the last
				rowString += sep
			}
		}

		// Truncate if the row exceeds terminal width
		if displayWidth(rowString) > termWidth {
			rowString = truncateString(rowString, termWidth)
		}

		tableOutput += rowString + "\n"
	}
	fmt.Printf("%s", tableOutput)
}

// Spinner calls a function and displays a spinner with explanatory message.
// The spinner is not displayed if the output isn't a rich terminal.
func Spinner(
	function func() error,
	prefix string,
	message string,
) error {
	var s *spinner.Spinner
	if IsOutputRich() {
		s = spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Prefix = prefix + "["
		s.Suffix = "]" + " " + message
		s.Start()
		// Stop the spinner when the function exits.
		defer func() { s.Stop() }()
	}
	return function()
}
