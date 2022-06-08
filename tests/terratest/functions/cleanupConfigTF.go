package functions

import (
	"fmt"
	"os"
)

func CleanupConfigTF(module string) {

	f, err := os.Create("../../modules/" + module + "/main.tf")

	if err != nil {

		fmt.Println(err)

	}

	fmt.Printf("%v", f)

	defer f.Close()

	_, err = f.WriteString("// Leave blank - main.tf will be set during testing")

	if err != nil {

		fmt.Println(err)

	}

}
