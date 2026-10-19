// Command durak runs the Durak card game in the terminal.
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"durak/internal/ui"
)

func main() {
	if _, err := tea.NewProgram(ui.New()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
