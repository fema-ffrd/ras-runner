package actions

import (
	"fmt"
	"log"
	"math"
	"os"
	"ras-runner/actions"
	"reflect"
	"strconv"

	"github.com/fema-ffrd/cc-go-sdk"
	"github.com/usace-cloud-compute/go-hdf5"
	"github.com/usace-cloud-compute/go-hdf5/util"
)

// ColumnToBc reads column oriented dataset from HDF5 RAS output files and writes it to boundary condition datasets in HDF5 RAS input files.
//
// Functionality:
//   - Validates column index and configuration parameters
//   - Opens source HDF5 file and retrieves specified dataset
//   - Opens destination HDF5 file and prepares target dataset
//   - Reads time-series data from source file
//   - Matches timestamps between source and destination datasets
//   - Extracts specified column data from source for each time step
//   - Writes updated boundary condition data to destination
//
// Data Format Requirements:
//   Source Dataset Structure: 2D hdf5 dataset
//   Destination Dataset Structure: 2D array with time as first column and boundary condition values as second column
//
// Supported Store Types:
//   S3 stores only (other store types will be added in future releases)
//
// Error Handling:
//   Returns descriptive error messages for invalid parameters, file access errors,
//   data read/write failures, and timestamp matching failures

func init() {
	cc.ActionRegistry.RegisterAction("column-to-boundary-condition", &ColumnToBcAction{})
}

type ColumnToBcAction struct {
	cc.ActionRunnerBase
}

const (
	colindexField = "column_index"
	nameField     = "name"
	dataPathField = "datapath"
	srcPathField  = "hdf"
)

// Run executes the column-to-boundary-condition action
func (a *ColumnToBcAction) Run() error {
	log.Printf("Updating boundary condition %s\n", a.Action.Description)
	column_index := a.Action.Attributes.GetStringOrFail(colindexField)
	readcol, err := strconv.Atoi(column_index)
	if err != nil {
		return fmt.Errorf("invalid column index: %s", column_index)
	}

	srcconfig, err := a.Action.Attributes.GetMap("src")
	if err != nil {
		return fmt.Errorf("missing src attribute data")
	}

	destconfig, err := a.Action.Attributes.GetMap("dest")
	if err != nil {
		return fmt.Errorf("missing dest attribute data")
	}

	//this type assertion is ugly but since we are stopping on error, a panic is ok
	srcname := srcconfig[nameField].(string)
	srcdatapath := srcconfig[dataPathField].(string)
	destname := destconfig[nameField].(string)
	destdatapath := destconfig[dataPathField].(string)

	src, err := a.PluginManager.GetInputDataSource(srcname)
	if err != nil {
		return fmt.Errorf("error getting input source %s: %s", srcname, err)
	}

	srcstore, err := a.PluginManager.GetStore(src.StoreName)
	if err != nil {
		return fmt.Errorf("error getting input store %s: %s", src.StoreName, err)
	}

	err = MigrateColumnData(src.Paths[srcPathField], srcstore, srcdatapath, destname, destdatapath, readcol)
	if err != nil {
		return fmt.Errorf("unable to migrate column data: %s", err)
	}

	log.Printf("finished updating boundary condition %s\n", a.Action.Description)

	return nil
}

// MigrateColumnData transfers columnar data from source to destination HDF5 files
// Parameters:
//   - src: Source file path
//   - srcstore: Data store for the source file
//   - src_datapath: Path to dataset within source file
//   - dest: Destination file name
//   - dest_datapath: Path to dataset within destination file
//   - readcol: Column index to read from source (1-based)
func MigrateColumnData(src string, srcstore *cc.DataStore, src_datapath string, dest string, dest_datapath string, readcol int) error {
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
		return err
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

		val, err := getRowVal2(srcVals, srcTime, destRow[0], readcol)
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

// getRowVal2 retrieves a value from source data at the specified time
// Parameters:
//   - srcVals: Source values dataset
//   - srcTimes: Source times dataset
//   - timeval: Target time value to match
//   - readcol: Column index to read (1-based)
//
// Returns:
//   - float32: Value from source data at matching time, or 0 if not found
//   - error: Error if any occurred during reading or matching
func getRowVal2(srcVals *util.HdfDataset, srcTimes *util.HdfDataset, timeval float32, readcol int) (float32, error) {
	numcols := srcVals.Dims()[1]
	//
	srcdata := make([]float32, numcols)
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
			return srcdata[readcol-1], nil
		}
	}
	return 0, fmt.Errorf("unable to find corresponding input source record for time %f", timeval)
}
