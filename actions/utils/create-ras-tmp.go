package utils

// #include <stdlib.h>
import "C"
import (
	"fmt"
	"log"
	"os"
	"ras-runner/actions"
	"strconv"
	"unsafe"

	"github.com/fema-ffrd/cc-go-sdk"
	"github.com/usace-cloud-compute/go-hdf5"
)

const (
	srcRasTempPath string = "default"
)

var RasTmpDatasets []string = []string{"Geometry", "Plan Data", "Event Conditions"}

func init() {
	cc.ActionRegistry.RegisterAction("create-ras-tmp", &CreateRasTmpAction{})
}

type CreateRasTmpAction struct {
	cc.ActionRunnerBase
}

func (a *CreateRasTmpAction) Run() error {
	log.Printf("Ready to create temp for %s\n", a.Action.Description)
	srcname, err := a.Action.Attributes.GetString("src")
	if err != nil {
		return fmt.Errorf("action attributes do not include a src")
	}

	local_dest, err := a.Action.Attributes.GetString("local_dest")
	if err != nil {
		return fmt.Errorf("action attributes do not include a local_dest")
	}

	save_to_remote := a.Action.Attributes.GetStringOrDefault("save_to_remote", "false")
	remote_dest_name := a.Action.Attributes.GetStringOrDefault("remote_dest", "")

	saveRemotely, err := strconv.ParseBool(save_to_remote)
	if err != nil {
		return fmt.Errorf("could not parse save_to_remote to bool")
	}

	src_local_path := fmt.Sprintf("%s/%s", actions.MODEL_DIR, srcname)
	err = MakeRasHdfTmp(src_local_path, local_dest)
	if err != nil {
		return fmt.Errorf("failed to create a blank temp file: %s", err)
	}
	if saveRemotely {
		if remote_dest_name == "" {
			return fmt.Errorf("user requested to save tmp file remotely but provided no output destination name")
		}
		//need to make sure the dest file is closed.
		//get the bytes of the dest file and push them to the dest datasource.
		destpath := fmt.Sprintf("%s/%s", actions.MODEL_DIR, local_dest)

		err = a.PluginManager.CopyFileToRemote(cc.CopyFileToRemoteInput{
			LocalPath:    destpath,
			RemoteDsName: remote_dest_name,
			DsPathKey:    srcRasTempPath,
		})
		if err != nil {
			return fmt.Errorf("failed to copy file to remote location: %s", err)
		}
	}

	log.Printf("Finished creating temp for %s\n", a.Action.Description)
	return nil
}

func MakeRasHdfTmp(src string, local_dest string) error {
	srcfile, err := hdf5.OpenFile(src, hdf5.F_ACC_RDONLY)
	if err != nil {
		return err
	}
	defer srcfile.Close()

	destpath := fmt.Sprintf("%s/%s", actions.MODEL_DIR, local_dest)
	_, err = os.Stat(destpath)

	var destfile *hdf5.File

	if os.IsNotExist(err) {
		destfile, err = hdf5.CreateFile(destpath, hdf5.F_ACC_EXCL)
		if err != nil {
			return err
		}
		defer destfile.Close()
	} else {
		destfile, err = hdf5.OpenFile(destpath, hdf5.F_ACC_RDWR)
		if err != nil {
			return err
		}
		defer destfile.Close()
	}
	for _, v := range RasTmpDatasets {
		err := srcfile.CopyTo(v, destfile, v)
		if err != nil {
			return err
		}
	}
	scalar, err := hdf5.CreateDataspace(hdf5.S_SCALAR)
	if err != nil {
		return err
	}
	defer scalar.Close()
	attrs := []string{"File Type", "File Version", "Projection", "Units System"}
	vals := make([]string, 4)
	srcrootgroup, err := srcfile.OpenGroup("/")
	if err != nil {
		return err
	}
	defer srcrootgroup.Close()
	destrootgroup, err := destfile.OpenGroup("/")
	if err != nil {
		return err
	}
	defer destrootgroup.Close()

	for i, v := range attrs {
		err := func() error {
			if srcrootgroup.AttributeExists(v) {
				attr, err := srcrootgroup.OpenAttribute(v)
				if err != nil {
					return err
				}
				defer attr.Close()

				var attrdata string
				/*dt, err := hdf5.T_C_S1.Copy()
				if err != nil {
					return err
				}*/
				attr.Read(&attrdata, hdf5.T_GO_STRING) //strong limiting restriction acceptable given our use case.
				vals[i] = string(attrdata)
				sdt, err := hdf5.T_C_S1.Copy()
				if err != nil {
					return err
				}

				err = sdt.SetSize(len(attrdata))
				if err != nil {
					return err
				}

				destattribute, err := destrootgroup.CreateAttribute(v, sdt, scalar)
				if err != nil {
					return err
				}
				defer destattribute.Close()
				cstring := C.CString(attrdata)
				defer C.free(unsafe.Pointer(cstring))
				err = destattribute.Write(cstring, sdt)
				if err != nil {
					return err
				}
			}
			return nil
		}()
		if err != nil {
			return err
		}
	}

	return nil
}
