package fastansi

import "fmt"

// Like the rest of "fast"ansi, this is just a weird utility thing, to create these fancy multiline TUI statuses. BUT with minimal code.
type StatusPrinter struct {
	maxlines      int
	maxlines_prev int
}

func NewStatusPrinter() *StatusPrinter {
	return &StatusPrinter{maxlines: 0, maxlines_prev: 0}
}

func (sp *StatusPrinter) Status(height int, str ...any) {
	if sp.maxlines < height {
		sp.maxlines = height
	}
	/*if sp.maxlines > sp.maxlines_prev {
		for range sp.maxlines - sp.maxlines_prev {
			fmt.Print("\n\n")
		}
		Up(sp.maxlines - sp.maxlines_prev)
	}*/
	CR()
	Up(height + 1)
	EraseLine()
	fmt.Print(str...)
	Down(height + 1)
	CR()
}

func (sp *StatusPrinter) PushLines() {
	for range sp.maxlines + 1 {
		fmt.Print("\n")
	}
	sp.maxlines_prev = sp.maxlines
	sp.maxlines = 0
}
