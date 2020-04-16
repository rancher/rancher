package main

import (
	"os"

	"github.com/sirupsen/logrus"
)

func main() {
	if err := os.RemoveAll("./pkg/wranger/generated"); err != nil {
		logrus.Fatal(err)
	}
}
