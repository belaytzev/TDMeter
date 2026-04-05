package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	flag.Parse()

	fmt.Printf("TDMeter starting with config: %s\n", *configPath)

	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "config file not found: %s\n", *configPath)
		os.Exit(1)
	}
}
