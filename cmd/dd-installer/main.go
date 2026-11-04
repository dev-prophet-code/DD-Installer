// Command dd-installer is the cross-platform native desktop installer for
// Dune Docker (Windows, macOS, Linux). It opens a real application window.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dev-prophet-code/dd-installer/internal/app"
	"github.com/dev-prophet-code/dd-installer/internal/platform"
	"github.com/dev-prophet-code/dd-installer/internal/ui"
)

func main() {
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *version {
		fmt.Println("dd-installer", app.Version)
		return
	}
	pf := platform.Current()
	if pf == nil {
		fmt.Fprintln(os.Stderr, "unsupported operating system")
		os.Exit(1)
	}
	ui.Run(app.New(pf))
}
