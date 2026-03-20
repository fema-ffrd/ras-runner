package hdf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"reflect"

	"ras-runner/actions"

	"github.com/fema-ffrd/cc-go-sdk"
)

func init() {
	cc.ActionRegistry.RegisterAction("ras-extract", &RasExtractAction{})
}

var dataTypeMap map[string]reflect.Kind = map[string]reflect.Kind{
	"float32": reflect.Float32,
	"float64": reflect.Float64,
	"int32":   reflect.Int32,
	"int64":   reflect.Int64,
	"string":  reflect.String,
}

type RasExtractAction struct {
	cc.ActionRunnerBase
}

// Run executes the RAS extraction action based on configured attributes.
//
// It supports three main extraction modes:
// 1. Dataset Extraction: Extracts a specific dataset from the HDF5 file.
// 2. Group Extraction: Enumerates datasets within an HDF5 group and extracts them.
// 3. Attribute Extraction: Extracts attributes from datasets or groups.
//
// The action reads configuration parameters such as data paths, output formats,
// data types, and processing options to determine how to extract and format the data.
//
// It handles both immediate writing of results and accumulation for later writing
// based on the 'accumulate-results' attribute.
//
// If 'accumulate-results' is true, the extracted data is stored in an accumulator
// and can be written later using the specified 'outputDataSource'.
//
// Returns an error if configuration is invalid or extraction fails.
func (a *RasExtractAction) Run() error {

	var modelPrefix string
	var plan string
	var err error

	modelPrefix, err = a.Action.Attributes.GetString("modelPrefix")
	if err != nil {
		modelPrefix = a.PluginManager.Attributes.GetStringOrFail("modelPrefix")
	}

	plan, err = a.Action.Attributes.GetString("plan")
	if err != nil {
		plan = a.PluginManager.Attributes.GetStringOrFail("plan")
	}

	modelResultsPath := fmt.Sprintf("%s/%s.p%s.tmp.hdf", actions.MODEL_DIR, modelPrefix, plan)

	blockName := a.Action.Attributes.GetStringOrDefault("block-name", "data")

	colnames, err := a.Action.Attributes.GetStringSlice("colnames")
	if err != nil {
		log.Println("error reading or no column names for extraction")
		colnames = nil
	}

	postprocessing, err := a.Action.Attributes.GetStringSlice("postprocess")
	if err != nil {
		log.Println("error or no postprocessing requested")
		postprocessing = nil
	}

	stringSizes, err := a.Action.Attributes.GetIntSlice("stringsizes")
	if err != nil {
		log.Println("no string sizes found")
		stringSizes = nil
	}

	input := RasExtractInput{
		DataPath:        a.Action.Attributes.GetStringOrDefault("datapath", ""),
		Attributes:      a.Action.Attributes.GetBooleanOrDefault("attributes", false),
		GroupPath:       a.Action.Attributes.GetStringOrDefault("grouppath", ""),
		GroupSuffix:     a.Action.Attributes.GetStringOrDefault("groupsuffix", ""),
		MatchPattern:    a.Action.Attributes.GetStringOrDefault("match", ""),
		ExcludePattern:  a.Action.Attributes.GetStringOrDefault("exclude", ""),
		Postprocess:     postprocessing,
		Colnames:        colnames,
		ColNamesDataset: a.Action.Attributes.GetStringOrDefault("coldata", ""),
		//ColSize:         a.Action.Attributes.GetIntOrDefault("colsize", 0),
		StringSizes: stringSizes,
		//DataType:        dt,
		WriteData:      a.Action.Attributes.GetBooleanOrDefault("writedata", false),
		WriteSummary:   a.Action.Attributes.GetBooleanOrDefault("writesummary", false),
		WriterType:     RasExtractWriterType(a.Action.Attributes.GetStringOrDefault("outputformat", "console")),
		WriteBlockName: blockName,
		Accumulate:     a.Action.Attributes.GetBooleanOrDefault("accumulate-results", false),
	}

	attrpath, err := a.Action.Attributes.GetString("attributepath")
	if err == nil {
		//this means we have an attr path, so we will process an attr extraction
		fmt.Println(attrpath)
		return err
	}

	if input.Attributes {
		aeinput := AttributeExtractInput{
			AttributePath:  input.DataPath,
			AttributeNames: input.Colnames,
			WriteBlockName: input.WriteBlockName,
		}

		err := AttributeExtract(aeinput, modelResultsPath)
		if err != nil {
			return err
		}

	} else {
		var dt reflect.Kind
		var ok bool
		dataType := a.Action.Attributes.GetStringOrFail("datatype")
		if dt, ok = dataTypeMap[dataType]; !ok {
			return fmt.Errorf("invalid data type: %s", dataType)
		}
		input.DataType = dt
		switch dt {
		case reflect.Float32:
			err = DataExtract[float32](input, modelResultsPath)
		case reflect.Float64:
			err = DataExtract[float64](input, modelResultsPath)
		case reflect.Int:
			err = DataExtract[int](input, modelResultsPath)
		case reflect.Int8:
			err = DataExtract[int8](input, modelResultsPath)
		case reflect.Int16:
			err = DataExtract[int16](input, modelResultsPath)
		case reflect.Int32:
			err = DataExtract[int32](input, modelResultsPath)
		case reflect.Int64:
			err = DataExtract[int64](input, modelResultsPath)
		}

		if err != nil {
			return err
		}
	}

	if input.Accumulate {
		//do nothing
		return nil
	} else {
		//refer to ras-extractor-writers for the writerAccumulator
		json, err := json.Marshal(&writerAccumulator)
		if err != nil {
			return err
		}
		outputDataSource := a.Action.Attributes.GetStringOrFail("outputDataSource")
		_, err = a.Action.Put(cc.PutOpInput{
			SrcReader: bytes.NewReader(json),
			DataSourceOpInput: cc.DataSourceOpInput{
				DataSourceName: outputDataSource,
				PathKey:        "extract",
			},
		})
		//reset accumulator
		writerAccumulator = make(map[string][]map[string]any)
		return err
	}
}
