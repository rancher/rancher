package functions

import (
	"os"
)

func CleanupVarsTF(module string) error {

	path := "../../modules/"

	if module == "aks" || module == "eks" || module == "gke" {
		path = path + "hosted/"
	}

	if module == "ec2_k3s" || module == "ec2_rke1" || module == "ec2_rke2" || module == "linode_k3s" || module == "linode_rke1" || module == "linode_rke2" {
		path = path + "node_driver/"
	}

	// TODO: Add path logic for custom clusters, once supported

	file, err := os.Create(path + module + "/terraform.tfvars")

	if err != nil {
		return err
	}

	defer file.Close()

	_, err = file.WriteString("// Leave blank - terraform.tfvars will be set during testing")

	if err != nil {
		return err
	}

	return nil
}
