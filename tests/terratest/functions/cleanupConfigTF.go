package functions

import (
	"os"
)

func CleanupConfigTF(module string) error {

	path := "../../modules/"

	if module == "aks" || module == "eks" || module == "gke" {
		path = path + "hosted/"
	}

	if module == "ec2_k3s" || module == "ec2_rke1" || module == "ec2_rke2" || module == "linode_k3s" || module == "linode_rke1" || module == "linode_rke2" {
		path = path + "node_driver/"
	}

	// TODO: Add path logic for custom clusters, once supported

	file, err := os.Create(path + module + "/main.tf")

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
		delete_file = path + module + delete_file
		err = os.Remove(delete_file)

		if err != nil {
			return err
		}
	}

	err = os.RemoveAll(path + module + "/.terraform")
	if err != nil {
		return err
	}

	return nil
}

// Use this to clean up terraform config files after testing - [terraform.tfstate, terraform.tfstate.backup, .terraform.lock.hcl, .terraform]
