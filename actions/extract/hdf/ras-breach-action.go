package hdf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"reflect"

	"ras-runner/actions"

	"github.com/fema-ffrd/cc-go-sdk"
)

const (
	breachLocationField string = "SaConn"
)

func init() {
	cc.ActionRegistry.RegisterAction("ras-breach-extract", &RasBreachExtractAction{})
}

// RasBreachExtractAction extracts breach data from RAS HDF5 files.
// It inspects 2D Hyd Conn datasets for breaching conditions and writes
// the results to ouput formats for further processing or output.
//
// The action reads breach data from the specified model results path,
// processes each flow area and connection, and accumulates breach records
// that are then written to the configured output data source.
type RasBreachExtractAction struct {
	cc.ActionRunnerBase
}

// Run executes the breach extraction action.
//
// It:
// 1. Determines model prefix and plan from action attributes or plugin manager
// 2. Constructs the model results path
// 3. Initializes breach data reader
// 4. Processes flow areas and connections to extract breach records
// 5. Writes extracted records to the output format
// 6. Sends output to the data to the configured output data source
//
// Returns error if any step fails during execution.
func (a *RasBreachExtractAction) Run() error {
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

	rb, err := NewRasBreachData(modelResultsPath)
	if err != nil {
		return err
	}
	defer rb.Close()

	flowAreas, err := rb.FlowAreas2D()
	if err != nil {
		return err
	}

	breachRecords := []BreachRecord{}
	for _, flowarea2d := range flowAreas {
		connectionNames, err := rb.ConnectionNames(flowarea2d)
		if err != nil {
			log.Printf("WARNING: no or invalid 2D Hyd Conn datasets for %s\n", flowarea2d)
			continue
		}
		for _, connectionName := range connectionNames {
			bd, err := rb.BreachData(flowarea2d, connectionName)
			if err != nil {
				log.Printf("No breach configuration for %s\n", connectionName)
			} else {
				log.Printf("Processing breach configuration for %s\n", connectionName)
				br := GetBreachRecord(a.PluginManager.EventIdentifier, flowarea2d, connectionName, &bd)
				breachRecords = append(breachRecords, br)
			}
		}
	}

	writer := JsonBreachDataExtractWriter{blockname: "breach_records"}
	writer.Write(breachRecords)

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
	return err
}

// BreachDataExtractWriter defines the interface for writing breach data records.
type BreachDataExtractWriter interface {
	Write(recs []BreachRecord) error
}

// JsonBreachDataExtractWriter implements the BreachDataExtractWriter interface
// for writing breach records in JSON format to an accumulator structure.
type JsonBreachDataExtractWriter struct {
	blockname string
}

// Write processes breach records and accumulates them in a structured format.
//
// For each record, it:
// 1. Converts the record to a map representation
// 2. Extracts the location field (SaConn) as dataset name
// 3. Formats the dataset path using the breachPathTemplate
// 4. Creates an output block with the dataset and record data
// 5. Appends the block to the writer accumulator under the blockname key
//
// This method populates the global writerAccumulator map for later JSON marshaling.
func (writer JsonBreachDataExtractWriter) Write(recs []BreachRecord) error {
	jsonRecs := breachRecordsToJsonAccumulatorMap(recs)
	for _, br := range jsonRecs {
		datasetName := br[breachLocationField]
		dataset := fmt.Sprintf(breachPathTemplate, datasetName.(string))
		outputBlock := RasExtractorOutputBlock[float32]{Dataset: dataset, Record: br}
		writerAccumulator[writer.blockname] = append(writerAccumulator[writer.blockname], map[string]any{datasetName.(string): outputBlock})
	}
	return nil
}

// breachRecordsToJsonAccumulatorMap converts a slice of BreachRecord structs
// into a slice of maps suitable for JSON marshaling.
//
// For each record:
// 1. Creates a new map for the record data
// 2. Uses reflection to iterate through struct fields
// 3. Handles float values specially to convert NaN to nil
// 4. Preserves all other field values as-is
// 5. Returns the slice of maps representing all records
func breachRecordsToJsonAccumulatorMap(recs []BreachRecord) []map[string]any {
	accumMaps := []map[string]any{}
	for _, r := range recs {
		recmap := make(map[string]any)
		v := reflect.ValueOf(r)
		t := v.Type()
		for j := 0; j < v.NumField(); j++ {
			field := t.Field(j)
			value := v.Field(j)
			if value.Kind() == reflect.Float32 || value.Kind() == reflect.Float64 {
				if math.IsNaN(value.Float()) {
					recmap[field.Name] = nil
				} else {
					recmap[field.Name] = value.Interface()
				}
			} else {
				recmap[field.Name] = value.Interface()
			}
		}
		accumMaps = append(accumMaps, recmap)
	}
	return accumMaps
}
