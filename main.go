package main

import (
	"log"
	_ "ras-runner/actions/extract/hdf"
	_ "ras-runner/actions/link"
	_ "ras-runner/actions/run"
	_ "ras-runner/actions/utils"

	"github.com/fema-ffrd/cc-go-sdk"
)

func main() {
	pm, err := cc.InitPluginManager()
	if err != nil {
		log.Fatalf("unable to initialize the CC plugin manager: %s\n", err)
	}
	err = pm.RunActions()
	if err != nil {
		log.Fatalf("Error running actions: %s\n", err)
	}
	log.Println("Finished")
}
