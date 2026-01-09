package main

import (
	"github.com/githubnext/gh-aw-mcpg/internal/cmd"
	"github.com/githubnext/gh-aw-mcpg/internal/logger"
)

var log = logger.New("main:main")

func main() {
	log.Printf("Starting MCPG version %s", Version)
	cmd.SetVersion(Version)
	log.Print("Executing root command")
	cmd.Execute()
}
