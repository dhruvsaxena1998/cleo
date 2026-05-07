package main

import (
	"github.com/dhruvsaxena1998/cleo/internal/cli"
	"github.com/dhruvsaxena1998/cleo/internal/tui"
)

func main() {
	cli.Execute(tui.Run)
}
