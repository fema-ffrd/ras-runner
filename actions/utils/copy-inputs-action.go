package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"ras-runner/actions"

	"github.com/fema-ffrd/cc-go-sdk"
)

func init() {
	cc.ActionRegistry.RegisterAction("copy-inputs", &CopyInputsAction{})
}

type CopyInputsAction struct {
	cc.ActionRunnerBase
}

func (ca *CopyInputsAction) Run() error {
	log.Println("Starting copy inputs action")
	for _, ds := range ca.PluginManager.Inputs {
		for k := range ds.Paths {
			err := func() error {
				source, err := ca.PluginManager.GetReader(cc.DataSourceOpInput{
					DataSourceName: ds.Name,
					PathKey:        k,
				})
				if err != nil {
					return err
				}
				defer source.Close()
				destfile := fmt.Sprintf("%s/%s", actions.MODEL_DIR, filepath.Base(ds.Paths[k]))
				log.Printf("Copying %s to %s\n", ds.Paths[k], destfile)
				destination, err := os.Create(destfile)
				if err != nil {
					return err
				}
				defer destination.Close()
				_, err = io.Copy(destination, source)
				return err
			}()
			if err != nil {
				log.Printf("Error fetching %s", ds.Paths[k])
				return err
			}
		}
	}
	return nil
}
