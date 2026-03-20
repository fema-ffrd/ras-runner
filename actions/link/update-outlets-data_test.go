package actions

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/fema-ffrd/cc-go-sdk"
)

const ELKATSUTTON_HDF string = "/ElkRiver_at_Sutton.p01.tmp.hdf"
const BLUESTONELOCAL_HDF string = "/BluestoneLocal.p01.tmp.hdf"
const UPPERNEW_HDF string = "/UpperNew.p01.tmp.hdf"
const ELKATSUTTON_BFILE = "/ElkRiver_at_Sutton.b01"
const Duwamish_hdf string = "/workspaces/cc-ras-runner/testData/Duwamish_17110013.p01.hdf"
const Duwamish_bfile string = "/workspaces/cc-ras-runner/testData/Duwamish_17110013.b01"

func TestOutletTSAction(t *testing.T) {
	parameters := make(map[string]any)
	parameters["bFile"] = filepath.Base(Duwamish_bfile)
	parameters["outletTS"] = "SA Conn: HowardHansonDam (Outlet TS: HH_TimeseriesOut)"
	parameters["hdfFile"] = filepath.Base(Duwamish_hdf)
	parameters["hdfDataPath"] = "/Event Conditions/Unsteady/Boundary Conditions/Flow Hydrographs/SA Conn: HowardHansonDam (Outlet TS: HH_TimeseriesOut)"
	modelDir := filepath.Dir(Duwamish_bfile)
	action := cc.Action{
		Type:        "update-outletTS",
		Description: "update-outletTS",
		IOManager: cc.IOManager{
			Attributes: parameters,
		},
	}

	runner := UpdateOutletTSAction{
		ActionRunnerBase: cc.ActionRunnerBase{
			ActionName: "update-outlet-ts-bfile",
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
