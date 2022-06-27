package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	_ "go.uber.org/automaxprocs"

	"github.com/pierdipi/sacura"
)

const (
	filePathFlag = "config"
)

func main() {

	path := flag.String(filePathFlag, "", "Path to the configuration file")
	flag.Parse()

	if path == nil || *path == "" {
		log.Printf("invalid flag %s", filePathFlag)
		usage()
		return
	}

	if err := run(*path); err != nil {
		log.Fatal(err)
	}
}

func usage() {
	log.Printf(`
sacura --%s <absolute_path_to_config_file>
`, filePathFlag)
}

func run(path string) error {

	log.Println("Reading configuration ...")

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer f.Close()

	config, err := sacura.FileConfig(f)
	if err != nil {
		return fmt.Errorf("failef to read config from file %s: %w", path, err)
	}

	return sacura.Main(NewContext(), config)
}

// NewContext creates a new context with signal handling.
func NewContext() context.Context {
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	return ctx
}
