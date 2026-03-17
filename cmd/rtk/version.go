package main

import "fmt"

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func printVersion() {
	fmt.Printf("rtk %s\ncommit %s\ndate %s\n", version, commit, date)
}
