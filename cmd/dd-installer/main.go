// Command dd-installer is the cross-platform GUI installer for Dune Docker.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/dev-prophet-code/dd-installer/internal/gui"
	"github.com/dev-prophet-code/dd-installer/internal/platform"
)

func main() {
	noOpen := flag.Bool("no-browser", false, "do not open the browser automatically; print the URL instead")
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *version {
		fmt.Println("dd-installer", gui.Version)
		return
	}
	pf := platform.Current()
	if pf == nil {
		fmt.Fprintln(os.Stderr, "unsupported operating system")
		os.Exit(1)
	}
	srv := gui.New(pf)
	ln, url, err := srv.Listen()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot start GUI:", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	fmt.Println("Dune Docker Installer is running. Open this address in your browser (it is private to this computer):")
	fmt.Println("  " + url)
	if !*noOpen {
		if err := pf.OpenBrowser(url); err != nil {
			fmt.Println("Could not open a browser automatically; open the address above manually.")
		}
	}
	if err := srv.Serve(ctx, ln); err != nil {
		fmt.Fprintln(os.Stderr, "server error:", err)
		os.Exit(1)
	}
}
