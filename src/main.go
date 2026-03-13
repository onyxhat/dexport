package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	outputFile := flag.String("o", "", "write output to `file` (default: stdout)")
	includeAll := flag.Bool("a", false, "include stopped containers when no names are given")
	showVersion := flag.Bool("v", false, "print version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: dexport [flags] [CONTAINER...]\n\n")
		fmt.Fprintf(os.Stderr, "Export running Docker containers as a docker-compose YAML file.\n\n")
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  CONTAINER  container name or ID (exports all running containers if omitted)\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Println("dexport", version)
		os.Exit(0)
	}

	ctx := context.Background()

	cli, err := newDockerClient()
	dieOnErr(err)
	defer cli.Close()

	containerArgs := flag.Args()

	var ids []string
	if len(containerArgs) > 0 {
		ids = containerArgs
	} else {
		ids, err = listContainers(ctx, cli, *includeAll)
		dieOnErr(err)
	}

	if len(ids) == 0 {
		fmt.Fprintln(os.Stderr, "dexport: no containers found")
		os.Exit(0)
	}

	inspected, err := inspectContainers(ctx, cli, ids)
	dieOnErr(err)

	cf := convertToComposeFile(inspected)
	dieOnErr(writeOutput(*outputFile, cf))
}

func dieOnErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "dexport:", err)
		os.Exit(1)
	}
}
