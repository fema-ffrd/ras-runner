package actions

import (
	"fmt"
	"log"
	"os"
	"ras-runner/actions"
	"strings"

	"github.com/fema-ffrd/cc-go-sdk"
)

func init() {
	cc.ActionRegistry.RegisterAction("update-bfile-skip-dss", &UpdateBfileSkipDSSAction{})
}

const SKIPDSS = "Extra Commands\n1\nSKIP_HDF_DSS"

type UpdateBfileSkipDSSAction struct {
	cc.ActionRunnerBase
	ModelDir string
}

// instruct the linux engine not to write DSS
func (uba *UpdateBfileSkipDSSAction) Run() error {
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
	rawbytes, err := os.ReadFile(bfilePath)
	if err != nil {
		return err
	}
	rawString := string(rawbytes)
	if !strings.Contains(rawString, SKIPDSS) {
		rawString = fmt.Sprintf("%v\n%v", rawString, SKIPDSS)
	}
	resultBytes := []byte(rawString)
	return os.WriteFile(bfilePath, resultBytes, 0600)

}
