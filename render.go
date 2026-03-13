package main

import (
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// renderCompose writes a ComposeFile as indented block-style YAML to w.
func renderCompose(w io.Writer, cf ComposeFile) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	defer enc.Close()
	return enc.Encode(cf)
}

// writeOutput writes the compose file to path, or to stdout when path is empty.
func writeOutput(path string, cf ComposeFile) error {
	if path == "" {
		return renderCompose(os.Stdout, cf)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return renderCompose(f, cf)
}
