package actions

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/fema-ffrd/cc-go-sdk"
)

const ONE_BREACH_FILE string = "/workspaces/cc-ras-runner/testData/Duwamish_17110013.b01"

// const MULTI_BREACH_FILE string = "/workspaces/cc-ras-runner/testData/multiDamBreach.b01"
const FRAG_CURVE_PATH string = "/workspaces/cc-ras-runner/testData/failure_elevations.json"
const Geometry_HDF_PATH string = "/workspaces/cc-ras-runner/testData/Duwamish_17110013.g01.hdf"

func TestBfileAction(t *testing.T) {
	parameters := make(map[string]any)
	parameters["bFile"] = filepath.Base(ONE_BREACH_FILE)
	parameters["fcFile"] = filepath.Base(FRAG_CURVE_PATH)
	parameters["geoHdfFile"] = filepath.Base(Geometry_HDF_PATH)
	modelDir := filepath.Dir(ONE_BREACH_FILE)
	action := cc.Action{
		Type:        "update-bfile",
		Description: "update bfile",
		IOManager: cc.IOManager{
			Attributes: parameters,
		},
	}
	runner := UpdateBfileAction{
		ActionRunnerBase: cc.ActionRunnerBase{
			ActionName: "update-breach-bfile",
			Action:     action,
		},
		ModelDir: modelDir,
	}
	err := runner.Run()
	if err != nil {
		fmt.Println(err)
		t.Fail()
	}
}
