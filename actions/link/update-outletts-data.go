package actions

import (
	"fmt"
	"log"
	"os"
	"ras-runner/actions"
	"ras-runner/ras"
	"reflect"
	"strings"

	"github.com/fema-ffrd/cc-go-sdk"
	"github.com/usace-cloud-compute/go-hdf5"
	"github.com/usace-cloud-compute/go-hdf5/util"
)

func init() {
	cc.ActionRegistry.RegisterAction("update-outlet-ts-bfile", &UpdateOutletTSAction{})
}

type UpdateOutletTSAction struct {
	cc.ActionRunnerBase
	ModelDir string
}

func (a *UpdateOutletTSAction) Run() error {
	// Assumes bFile and hdf file  were copied local with the CopyLocal a.Action.
	if a.ModelDir == "" {
		a.ModelDir = actions.MODEL_DIR
	}
	log.Printf("Ready to update bFile with new observed flows.")
	bFileName, err := a.Action.Attributes.GetString("bFile")
	if err != nil {
		return fmt.Errorf("action attributes do not include a bFile")
	}

	outletTSName, err := a.Action.Attributes.GetString("outletTS")
	if err != nil {
		return fmt.Errorf("action attributes do not include an outletTS")
	}

	bfilePath := fmt.Sprintf("%v/%v", a.ModelDir, bFileName)
	if !actions.FileExists(bfilePath) {
		return fmt.Errorf("input source %s, was not found in local directory. Run copy-local first", bfilePath)
	}
	bf, err := ras.InitBFile(bfilePath)
	if err != nil {
		return fmt.Errorf("unable to initialize the bfile: %s", err)
	}
	outletTSIdx := 0
	var outletTS *ras.OutletTS
	for idx, block := range bf.BfileBlocks {
		outts, ok := block.(*ras.OutletTS)
		if ok {
			if strings.Contains(outts.Name, outletTSName) {
				outletTSIdx = idx //will never be zero, that is the ras version block.
				outletTS = outts
				break
			}
		}
	}

	if outletTSIdx == 0 {
		return fmt.Errorf("could not find the outlet TS named %s", outletTSName)
	}
	hdfDataPath, err := a.Action.Attributes.GetString("hdfDataPath")
	if err != nil {
		return fmt.Errorf("action attributes do not include a hdfDataPath")
	}

	hdfFileName, err := a.Action.Attributes.GetString("hdfFile")
	if err != nil {
		return fmt.Errorf("action attributes do not include a hdfFile")
	}

	hdfFilePath := fmt.Sprintf("%v/%v", a.ModelDir, hdfFileName)
	destfile, err := hdf5.OpenFile(hdfFilePath, hdf5.F_ACC_RDWR) //@TODO ..not closing..
	if err != nil {
		return fmt.Errorf("unable to open the hdf destination file: %s", err)
	}
	defer destfile.Close()

	options := util.HdfReadOptions{
		Dtype:        reflect.Float32,
		File:         destfile,
		ReadOnCreate: true,
	}
	destVals, err := util.NewHdfDataset(hdfDataPath, options)
	if err != nil {
		return fmt.Errorf("unable to get the desitional dataset values from hdf: %s", err)
	}
	defer destVals.Close()

	flows := make([]float32, outletTS.RowCount)
	err = destVals.ReadColumn(1, &flows)
	if err != nil {
		return fmt.Errorf("failed to read flows column from outletTS hdf: %s", err)
	}

	err = outletTS.UpdateFloatArray(flows)
	if err != nil {
		return fmt.Errorf("failed to update the outlet float array: %s", err)
	}

	bf.BfileBlocks[outletTSIdx] = outletTS
	resultBytes, err := bf.Write()
	if err != nil {
		return fmt.Errorf("failed to write bfile to a byte array: %s", err)
	}

	return os.WriteFile(bfilePath, resultBytes, 0600)
}
