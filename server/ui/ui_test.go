package ui

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_disableIndexList(t *testing.T) {

	AddTestFile()

	srv := StartHTTPServer()

	time.Sleep(1 * time.Second)

	CaseForTest(t)

	if err := srv.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	DeleteTestFile()

}

//testAddFile creates resource prepare for the test
func AddTestFile() {

	os.Mkdir("./folder1", 0777)
	os.MkdirAll("./folder2/sub-folder", 0777)

	file1, err := os.Create("./folder1" + "/a.txt")
	if err != nil {
		fmt.Println(err)
	}
	defer file1.Close()

	file2, err := os.Create("./folder2/sub-folder" + "/b.txt")
	if err != nil {
		fmt.Println(err)
	}
	defer file2.Close()

	file3, err := os.Create("./index.html")
	if err != nil {
		fmt.Println(err)
	}
	defer file3.Close()
}

//testDeleteFile remove resource  after the test
func DeleteTestFile() {

	os.RemoveAll("./folder1")
	os.RemoveAll("./folder2")
	os.Remove("./index.html")

}

func CaseForTest(t *testing.T) {
	assert := assert.New(t)

	resp1, err := http.Get("http://127.0.0.1:8000/")
	assert.Nil(err)
	ioutil.ReadAll(resp1.Body)
	assert.Equal(resp1.StatusCode, 200)

	resp2, err := http.Get("http://127.0.0.1:8000/folder1/")
	assert.Nil(err)
	ioutil.ReadAll(resp2.Body)
	assert.Equal(resp2.StatusCode, 404)

	resp3, err := http.Get("http://127.0.0.1:8000/folder1/a.txt")
	assert.Nil(err)
	ioutil.ReadAll(resp3.Body)
	assert.Equal(resp3.StatusCode, 200)

	resp4, err := http.Get("http://127.0.0.1:8000/folder2")
	assert.Nil(err)
	ioutil.ReadAll(resp4.Body)
	assert.Equal(resp4.StatusCode, 404)

	resp5, err := http.Get("http://127.0.0.1:8000/folder2/sub-folder/")
	assert.Nil(err)
	ioutil.ReadAll(resp5.Body)
	assert.Equal(resp5.StatusCode, 404)
}

//start the http server
func StartHTTPServer() *http.Server {
	srv := &http.Server{Addr: ":8000"}
	http.Handle("/", Content())

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			fmt.Println(err)
		}
	}()

	return srv
}
