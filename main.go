package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

var version = "dev"

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println("cloud_pcap", version)
		return
	}

	cfgPath := flag.String("config", "config.json", "path to configuration file")
	flag.Parse()

	cfg, err := LoadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := runCapture(cfg); err != nil {
		log.Fatalf("Capture failed: %v", err)
	}
}
