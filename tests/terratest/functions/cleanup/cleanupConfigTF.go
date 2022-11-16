package functions

import (
	"os"
)

func CleanupConfigTF() error {
	path := "../../modules/cluster"
	file, err := os.Create(path + "/main.tf")

	if err != nil {
		return err
	}

	defer file.Close()

	_, err = file.WriteString("// Leave blank - main.tf will be set during testing")

	if err != nil {
		return err
	}

	delete_files := [3]string{"/terraform.tfstate", "/terraform.tfstate.backup", "/.terraform.lock.hcl"}

	for _, delete_file := range delete_files {
		delete_file = path + delete_file
		err = os.Remove(delete_file)

		if err != nil {
			return err
		}
	}

	err = os.RemoveAll(path + "/.terraform")
	if err != nil {
		return err
	}

	return nil
}

// This cleans up terraform config files after testing - [terraform.tfstate, terraform.tfstate.backup, .terraform.lock.hcl, .terraform]
