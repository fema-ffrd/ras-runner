package actions

import (
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"ras-runner/actions"
	"reflect"

	"github.com/fema-ffrd/cc-go-sdk"
	"github.com/usace-cloud-compute/go-hdf5"
	"github.com/usace-cloud-compute/go-hdf5/util"
)

func init() {
	cc.ActionRegistry.RegisterAction("update-boundary-condition", &UpdateBoundaryConditionAction{})
}

type UpdateBoundaryConditionAction struct {
	cc.ActionRunnerBase
}

func (a *UpdateBoundaryConditionAction) Run() error {
	log.Printf("Updating boundary condition %s\n", a.Action.Description)
	srcname := a.Action.Attributes["src"].(map[string]any)["name"].(string)
	srcdatapath := a.Action.Attributes["src"].(map[string]any)["datapath"].(string)
	dest := a.Action.Attributes["dest"].(map[string]any)["name"].(string)
	destdatapath := a.Action.Attributes["dest"].(map[string]any)["datapath"].(string)

	src, err := a.PluginManager.GetInputDataSource(srcname)
	if err != nil {
		return fmt.Errorf("error getting input source %s: %s", srcname, err)
	}

	srcstore, err := a.PluginManager.GetStore(src.StoreName)
	if err != nil {
		return fmt.Errorf("error getting input store %s: %s", src.StoreName, err)
	}

	err = MigrateBoundaryConditionData(src.Paths["0"], srcstore, srcdatapath, dest, destdatapath)
	if err != nil {
		return fmt.Errorf("unable to migrate boundary condition: %s", err)
	}
	log.Printf("finished updating boundary condition %s\n", a.Action.Description)

	return nil
}

func MigrateBoundaryConditionData(src string, srcstore *cc.DataStore, src_datapath string, dest string, dest_datapath string) error {
	if srcstore.StoreType == "S3" {
		profile := srcstore.DsProfile
		bucket := os.Getenv(fmt.Sprintf("%s_%s", profile, actions.AWSBUCKET))
		src = fmt.Sprintf(actions.S3BucketTemplate, bucket, srcstore.Parameters["root"], actions.EncodeUrlPath(src))
	}
	srcfile, err := util.OpenFile(src, srcstore.DsProfile)
	if err != nil {
		return err
	}
	defer srcfile.Close()

	destpath := fmt.Sprintf("%s/%s", actions.MODEL_DIR, dest)
	_, err = os.Stat(destpath)
	if err != nil {
		return fmt.Errorf("path %s does not exist", destpath)
	}

	var destfile *hdf5.File

	destfile, err = hdf5.OpenFile(destpath, hdf5.F_ACC_RDWR)
	if err != nil {
		return err
	}
	defer destfile.Close()

	//Get the data values from the source file
	//this is the RAS model output
	options := util.HdfReadOptions{
		Dtype:        reflect.Float32,
		File:         srcfile,
		ReadOnCreate: true,
	}

	srcVals, err := util.NewHdfDataset(src_datapath, options)
	if err != nil {
		return err
	}
	defer srcVals.Close()

	//Get the times corresponding to the source file values

	tsoptions := util.HdfReadOptions{
		Dtype:        reflect.Float64,
		File:         srcfile,
		ReadOnCreate: true,
	}

	srcTime, err := util.NewHdfDataset(actions.TimePath(src_datapath), tsoptions)
	if err != nil {
		return err
	}
	defer srcTime.Close()

	//Get a copy of the destination dataset
	var destVals *util.HdfDataset

	err = func() error {
		destoptions := util.HdfReadOptions{
			Dtype:        reflect.Float32,
			File:         destfile,
			ReadOnCreate: true,
		}
		destVals, err = util.NewHdfDataset(dest_datapath, destoptions)
		if err != nil {
			return err
		}
		defer destVals.Close()
		return nil
	}()
	if err != nil {
		return err
	}

	//create a new buffer with mutated boundary conditions
	boundaryConditionData := make([]float32, destVals.Rows()*2)

	for i := 0; i < destVals.Rows(); i++ {

		destRow := make([]float32, 2)
		err := destVals.ReadRow(i, &destRow)
		if err != nil {
			return err
		}

		val, err := getRowVal(srcVals, srcTime, destRow[0])
		if err != nil {
			return err
		}

		boundaryConditionData[i*2] = destRow[0]
		boundaryConditionData[i*2+1] = val
	}

	//write the new boundary condition buffer back to the destiation dataset
	destWriter, err := destfile.OpenDataset(dest_datapath)
	if err != nil {
		return err
	}
	defer destWriter.Close()
	err = destWriter.Write(&boundaryConditionData)
	if err != nil {
		return err
	}
	return nil
}

func getRowVal(srcVals *util.HdfDataset, srcTimes *util.HdfDataset, timeval float32) (float32, error) {
	srcdata := make([]float32, 5)
	srctime := make([]float64, 1)

	if timeval <= 0.0 {
		err := srcVals.ReadRow(0, &srcdata)
		if err != nil {
			return 0, err
		}
		return srcdata[0], nil
	}

	for i := 0; i < srcVals.Rows(); i++ {
		err := srcVals.ReadRow(i, &srcdata)
		if err != nil {
			return 0, err
		}
		err = srcTimes.ReadRow(i, &srctime)
		if err != nil {
			return 0, err
		}

		if math.Abs(float64(timeval)-srctime[0]) < actions.Tolerance {
			return srcdata[0], nil
		}
	}
	return 0, errors.New(fmt.Sprintf("Unable to find corresponding input source record for time %f", timeval))
}
