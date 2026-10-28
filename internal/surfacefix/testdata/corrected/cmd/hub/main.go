// Command hub serves the panel.
package main

import (
	"io"
	"os"
)

func main() {
	if err := run(os.Stdout); err != nil {
		os.Exit(1)
	}
}

func run(out io.Writer) error {
	_, err := io.WriteString(out, greeting("hub")+"\n")
	return err
}

func greeting(name string) string { return "hello " + name }
