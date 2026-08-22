package main

import (
	"fmt"
	"os"
)

func fatal(step string, err error, out string) {
	if out != "" {
		fmt.Print(out)
	}
	fmt.Fprintf(os.Stderr, "rem: %s failed: %v\n", step, err)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "add":
		err = cmdAdd(os.Args[2:])
	case "search":
		err = cmdSearch(os.Args[2:])
	case "sync":
		err = cmdSync()
	case "dream":
		err = cmdDream(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "rem: unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fatal(os.Args[1], err, "")
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `rem - memory workflow for the knowledge vault

usage:
  rem add [-src harness] "<fact>"   capture a candidate fact to ai/YYYY-MM-DD.md
  rem search "<query>"              semantic search (memsearch, collection ai)
  rem dream [--apply]               consolidation report; --apply collapses exact dups
  rem sync                          commit+merge+push via guarded knowledge-sync
`)
}
