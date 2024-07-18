package fastansi

import "fmt"

func Up(lines int) {
	fmt.Print("\x1b[" + fmt.Sprintf("%v", lines) + "A")
}

func Down(lines int) {
	fmt.Print("\x1b[" + fmt.Sprintf("%v", lines) + "B")
}

func EraseLine() {
	fmt.Print("\x1b[K")
}

func CR() {
	fmt.Print("\x1b[0E")
}
