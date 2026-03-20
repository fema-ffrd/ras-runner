package utils

import (
	"fmt"
	"log"
	"ras-runner/actions"

	"github.com/fema-ffrd/cc-go-sdk"
)

const (
	defaultDatasourcePath string = "default"
)

func init() {
	cc.ActionRegistry.RegisterAction("post-outputs", &PostOutputsAction{})
}

type PostOutputsAction struct {
	cc.ActionRunnerBase
}

func (a *PostOutputsAction) Run() error {
	err := postOutputFiles(a.PluginManager)
	if err != nil {
		return fmt.Errorf("failed to post outputs: %s", err)
	}
	return nil
}

func postOutputFiles(pm *cc.PluginManager) error {
	modelPrefix := pm.Attributes.GetStringOrFail("modelPrefix")
	plan := pm.Attributes.GetStringOrFail("plan")
	reservedfilename := fmt.Sprintf("%s.p%s.tmp.hdf", modelPrefix, plan)
	for _, ds := range pm.Outputs {
		err := func() error {
			//check if the datasource name is reserved// rasoutput or pxx.tmp.hdf -> ignore these two.
			if ds.Name == "rasoutput" {
				return nil
			}

			if ds.Name == reservedfilename {
				return nil
			}
			//get the local file from the datasource name.
			return pm.CopyFileToRemote(cc.CopyFileToRemoteInput{
				LocalPath:       fmt.Sprintf("%s/%s", actions.MODEL_DIR, ds.Name),
				RemoteStoreName: ds.Name,
				RemotePath:      defaultDatasourcePath,
			})
		}()
		if err != nil {
			log.Printf("Error fetching %s", ds.Paths[defaultDatasourcePath])
			return err
		}
	}
	return nil
}
