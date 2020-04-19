package main

import (
	"log"

	"github.com/simonswine/hcloud-metallb-floater/cmd"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		log.Fatalf("problem executing rootCmd: %v", err)
	}
}
