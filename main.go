package main

import "github.com/benstroud/lazyreview/cmd"

// main is the entry point for the LazyReview CLI application.
// It executes the root command from the cmd package, which handles
// all subcommands and user interactions for the application.
func main() {
	cmd.Execute()
}
