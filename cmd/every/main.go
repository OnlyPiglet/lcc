package main

import (
	"every"
	"os"
)

func main() { os.Exit((every.CLI{Args: os.Args[1:]}).Run()) }
