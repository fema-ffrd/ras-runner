package actions

import (
	"fmt"
	"log"
	"reflect"

	"github.com/fema-ffrd/cc-go-sdk"
	"github.com/usace-cloud-compute/go-hdf5"
	"github.com/usace-cloud-compute/go-hdf5/util"
)

const (
	srcDataSourcePath = "hdf"
)

func init() {
	cc.ActionRegistry.RegisterAction("hdf-to-hdf", &HdftoHdfDatasetAction{})
}

/*
	copies content of one hdf5 data file into another one that is local
*/

type HdftoHdfDatasetAction struct {
	cc.ActionRunnerBase
}

func (a *HdftoHdfDatasetAction) Run() error {
	log.Printf("Ready to %s\n", a.Action.Description)

	src, err := a.Action.GetInputDataSource("src")
	if err != nil {
		return fmt.Errorf("missing input datasource named src")
	}

	dest, err := a.Action.GetOutputDataSource("dest")
	if err != nil {
		return fmt.Errorf("missing output source named dest")
	}

	if len(src.DataPaths) != len(dest.DataPaths) {
		return fmt.Errorf("src and dest datapath lengths do not match")
	}

	for srckey, srcdatapath := range src.DataPaths {
		err = CopyHdf5Dataset(src.Paths["hdf"], srcdatapath, dest.Paths["hdf"], dest.DataPaths[srckey])
		if err != nil {
			return fmt.Errorf("error copying from src to dest %s: %s", srcdatapath, dest.DataPaths[srckey])
		}
	}
	return nil
}

func CopyHdf5Dataset(src string, srcdataset string, dest string, destdataset string) error {

	srcfile, err := hdf5.OpenFile(src, hdf5.F_ACC_RDWR)
	if err != nil {
		return err
	}
	defer srcfile.Close()

	destpath := dest //fmt.Sprintf("%s/%s", actions.MODEL_DIR, dest)

	destfile, err := hdf5.OpenFile(destpath, hdf5.F_ACC_RDWR)
	if err != nil {
		return err
	}
	defer destfile.Close()

	//need to perform a read/write rather than a copy to.

	var srcVals *util.HdfDataset
	err = func() error {
		srcoptions := util.HdfReadOptions{
			Dtype:        reflect.Float32,
			File:         srcfile,
			ReadOnCreate: true,
		}
		srcVals, err = util.NewHdfDataset(srcdataset, srcoptions)
		if err != nil {
			return err
		}
		defer srcVals.Close()
		return nil
	}()
	if err != nil {
		return err
	}

	var dstVals *util.HdfDataset
	err = func() error {
		dstoptions := util.HdfReadOptions{
			Dtype:        reflect.Float32,
			File:         destfile,
			ReadOnCreate: true,
		}
		dstVals, err = util.NewHdfDataset(destdataset, dstoptions)
		if err != nil {
			return err
		}
		defer dstVals.Close()
		return nil
	}()
	if err != nil {
		return err
	}

	if srcVals.Cols() != dstVals.Cols() {
		return fmt.Errorf("source column count doesnt equal dest column count")
	}

	if srcVals.Rows() != dstVals.Rows() {
		return fmt.Errorf("source row count doesnt equal dest row count")
	}
	writer, err := destfile.OpenDataset(destdataset)
	if err != nil {
		return err
	}
	data := make([]float32, dstVals.Rows()*dstVals.Cols())
	index := 0
	for i := 0; i < dstVals.Rows(); i++ {

		srcRow := make([]float32, dstVals.Cols())
		err := srcVals.ReadRow(i, &srcRow)
		if err != nil {
			return err
		}
		for j := 0; j < dstVals.Cols(); j++ {
			data[index] = srcRow[j]
			index++
		}
	}
	err = writer.Write(data)
	if err != nil {
		return err
	}
	return nil
}
