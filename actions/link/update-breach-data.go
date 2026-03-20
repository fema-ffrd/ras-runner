package actions

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"ras-runner/actions"
	"ras-runner/fragilitycurve"
	"ras-runner/ras"

	"github.com/fema-ffrd/cc-go-sdk"
)

func init() {
	cc.ActionRegistry.RegisterAction("update-breach-bfile", &UpdateBfileAction{})
}

// UpdateBfileAction is an action that updates breach elevations in a bfile based on fragility curve results.
// It reads a fragility curve output file, amends the breach elevations in the bfile, and writes the updated
// bfile back to disk.
type UpdateBfileAction struct {
	cc.ActionRunnerBase
	ModelDir string
}

// Run executes the UpdateBfileAction. It performs the following steps:
// 1. Retrieves the path to the bFile and checks if it exists locally.
// 2. Initializes the BFile using ras.InitBFile.
// 3. Creates an SNET ID to name map from a specified geometry file.
// 4. Loads fragility curve results from a JSON file.
// 5. Amends the breach elevations in the bfile based on the fragility curve results.
// 6. Writes the updated bfile back to disk.
func (uba *UpdateBfileAction) Run() error {
	// Assumes bFile and fragility curve file  were copied local with the CopyLocal uba.Action.
	log.Printf("Ready to update bFile.")
	if uba.ModelDir == "" {
		uba.ModelDir = actions.MODEL_DIR
	}

	bFileName := uba.Action.Attributes.GetStringOrFail("bFile")
	bfilePath := fmt.Sprintf("%v/%v", uba.ModelDir, bFileName)
	if !actions.FileExists(bfilePath) {
		return fmt.Errorf("input source %s, was not found in local directory. Run copy-local first", bfilePath)
	}
	bf, err := ras.InitBFile(bfilePath)
	if err != nil {
		return fmt.Errorf("failed to initialized and read the b-file")
	}

	log.Print("Creating SNET ID to name map from geometry file")
	hdfFileName, err := uba.Action.Attributes.GetString("geoHdfFile")
	if err != nil {
		return fmt.Errorf("action attributes do not include a geoHdfFile")
	}

	hdfFilePath := fmt.Sprintf("%v/%v", uba.ModelDir, hdfFileName)
	err = bf.SetSNetIDToNameFromGeoHDF(hdfFilePath)
	if err != nil {
		return fmt.Errorf("unable to set the snetID from the geohdf: %s", err)
	}

	log.Print("Loading Fragility Curve Results")
	fcFileName, err := uba.Action.Attributes.GetString("fcFile")
	if err != nil {
		return fmt.Errorf("action attributes do not include a fcFile")
	}
	fcFilePath := fmt.Sprintf("%v/%v", uba.ModelDir, fcFileName)
	fcFileBytes, err := os.ReadFile(fcFilePath)
	if err != nil {
		return fmt.Errorf("unable to read the fcFile: %s", err)
	}

	var fcResult fragilitycurve.ModelResult
	err = json.Unmarshal(fcFileBytes, &fcResult)
	if err != nil {
		return fmt.Errorf("error unmarshaling fragility curve from %s", fcFileName)
	}

	for _, fclr := range fcResult.Results {
		err = bf.AmmendBreachElevations(fclr.Name, fclr.FailureElevation)
		if err != nil {
			//@TODO...dont like this...doe we really skip on fail?
			log.Printf("failed to ammend breach elevations: %s %f: %s\n", fclr.Name, fclr.FailureElevation, err)
		}
	}

	resultBytes, err := bf.Write()
	if err != nil {
		return fmt.Errorf("error writing b file: %s", err)
	}

	return os.WriteFile(bfilePath, resultBytes, 0600)

}
