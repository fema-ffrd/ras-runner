package actions

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"ras-runner/actions"
	"ras-runner/actions/utils"
	"reflect"

	"github.com/fema-ffrd/cc-go-sdk"
	"github.com/usace-cloud-compute/go-hdf5"
	"github.com/usace-cloud-compute/go-hdf5/util"
)

const (
	reflineAttrName = "refline"
)

func init() {
	cc.ActionRegistry.RegisterAction("refline-to-boundary-condition", &ReflineToBc{})
}

// ReflineToBc reads reference line data from HDF5 RAS output files and writes it to boundary condition datasets in HDF5 RAS input files.
//
// This action facilitates the transfer of reference line flow data from RAS output results to boundary conditions in RAS input models.
// Unlike the column-to-boundary-condition action, this action assumes that the time arrays in source and destination datasets are identical
// and does not perform time matching between datasets.
type ReflineToBc struct {
	cc.ActionRunnerBase
}

// Run executes the refline-to-boundary-condition action.
//
// It performs the following steps:
// 1. Retrieves the reference line name from action attributes
// 2. Gets source and destination data sources from IOManager
// 3. Calls MigrateRefLineData to process the data transfer
// 4. Returns any errors encountered during processing
//
// The action requires:
// - "refline" attribute specifying which reference line to extract
// - "source" configuration with name and datapath for input data
// - "destination" configuration with name and datapath for output data
func (a *ReflineToBc) Run() error {
	log.Printf("Updating refline to boundary condition %s\n", a.Action.Description)
	refline := a.Action.Attributes["refline"].(string)

	useRemote := a.Action.Attributes.GetBooleanOrDefault("use-remote-reads", true)
	src, err := a.Action.IOManager.GetInputDataSource("source")
	if err != nil {
		return fmt.Errorf("error getting input source %s: %s", "source", err)
	}

	srcstore, err := a.Action.IOManager.GetStore(src.StoreName)
	if err != nil {
		return fmt.Errorf("error getting input store %s: %s", src.StoreName, err)
	}

	dest, err := a.Action.IOManager.GetOutputDataSource("destination")
	if err != nil {
		return fmt.Errorf("error getting input source %s: %s", "source", err)
	}

	err = MigrateRefLineData(src.Paths["hdf"], srcstore, src.DataPaths["refline"], dest.Paths["hdf"], dest.DataPaths["bcline"], refline, useRemote)
	if err != nil {
		return fmt.Errorf("failed to migrate refline data: %s", err)
	}

	log.Printf("finished updating boundary condition %s\n", a.Action.Description)

	return nil
}

func MigrateRefLineData(src string, srcstore *cc.DataStore, src_datapath string, dest string, dest_datapath string, refline string, useRemote bool) error {
	if useRemote {
		profile := srcstore.DsProfile
		bucket := os.Getenv(fmt.Sprintf("%s_%s", profile, actions.AWSBUCKET))
		template := os.Getenv("HDF_AWS_S3_TEMPLATE")
		src = fmt.Sprintf(template, bucket, srcstore.Parameters["root"], actions.EncodeUrlPath(src))
	} else {
		src = fmt.Sprintf("%s/%s", actions.MODEL_DIR, filepath.Base(src))
	}

	srcfile, err := util.OpenFile(src, srcstore.DsProfile)
	if err != nil {
		return err
	}
	defer srcfile.Close()

	srcTime, err := util.NewHdfDataset(actions.TimePath(src_datapath), util.HdfReadOptions{
		Dtype:        reflect.Float64,
		File:         srcfile,
		ReadOnCreate: true,
	})

	if err != nil {
		return err
	}
	defer srcTime.Close()

	//get the reference line flow dataset
	refLineVals, err := util.NewHdfDataset(src_datapath+"/Flow", util.HdfReadOptions{
		Dtype:        reflect.Float32,
		File:         srcfile,
		ReadOnCreate: true,
	})
	if err != nil {
		return err
	}
	defer refLineVals.Close()

	//get the reference line positions
	mt := utils.DatasetMetadata
	attr, err := utils.GetAttrMetadata(srcfile, mt, src_datapath+"/Name", "")
	if err != nil {
		return err
	}

	refLineNames, err := util.NewHdfDataset(src_datapath+"/Name", util.HdfReadOptions{
		Dtype:        reflect.String,
		Strsizes:     util.NewHdfStrSet(int(attr.AttrSize)),
		File:         srcfile,
		ReadOnCreate: true,
	})

	if err != nil {
		return err
	}
	defer refLineNames.Close()

	refLineColumnIndex := -1
	for i := 0; i < refLineNames.Rows(); i++ {
		name := []string{}
		err := refLineNames.ReadRow(i, &name)
		if err != nil || len(name) == 0 {
			return errors.New("error reading reference line Names")
		}
		fmt.Println(refline)
		fmt.Println(name[0])
		if refline == name[0] {
			refLineColumnIndex = i
		}
	}
	if refLineColumnIndex < 0 {
		return fmt.Errorf("invalid reference line: %s", refline)
	}

	destpath := fmt.Sprintf("%s/%s", actions.MODEL_DIR, dest)
	_, err = os.Stat(destpath)
	if err != nil {
		return err
	}

	var destfile *hdf5.File

	destfile, err = hdf5.OpenFile(destpath, hdf5.F_ACC_RDWR)
	if err != nil {
		return err
	}
	defer destfile.Close()

	//get a copy of the destination data
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

	//create a new dataset
	boundaryConditionData := make([]float32, destVals.Rows()*2)

	for i := 0; i < destVals.Rows(); i++ {
		refLineRow := []float32{}
		err = refLineVals.ReadRow(i, &refLineRow)
		if err != nil {
			return err
		}
		destRow := []float32{}
		err = destVals.ReadRow(i, &destRow)
		if err != nil {
			return err
		}

		boundaryConditionData[i*2] = destRow[0]
		boundaryConditionData[i*2+1] = refLineRow[refLineColumnIndex]
	}
	//write the new boundary condition buffer back to the destiation dataset
	destWriter, err := destfile.OpenDataset(dest_datapath)
	if err != nil {
		return err
	}
	defer destWriter.Close()
	return destWriter.Write(&boundaryConditionData)
}
