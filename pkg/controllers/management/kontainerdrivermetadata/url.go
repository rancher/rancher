package kontainerdrivermetadata

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/git"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rke/types/kdm"
	"github.com/rancher/wrangler/v2/pkg/randomtoken"
	"github.com/sirupsen/logrus"
)

func parseURL(rkeData map[string]interface{}) (*MetadataURL, error) {
	url := &MetadataURL{}
	path, ok := rkeData["url"]
	if !ok {
		return nil, fmt.Errorf("url not present in settings %s", settings.RkeMetadataConfig.Get())
	}
	url.path = convert.ToString(path)
	branch, ok := rkeData["branch"]
	if !ok {
		return url, nil
	}
	url.branch = convert.ToString(branch)
	latestHash, err := git.RemoteBranchHeadCommit(url.path, url.branch)
	if err != nil {
		return nil, fmt.Errorf("error getting latest commit %s %s %v", url.path, url.branch, err)
	}
	url.latestHash = latestHash
	if strings.HasSuffix(url.path, ".git") {
		url.isGit = true
	}
	return url, nil
}

func loadData(url *MetadataURL) (kdm.Data, error) {
	if url.isGit {
		return getDataGit(url.path, url.branch)
	}
	return getDataHTTP(url.path)
}

func getDataHTTP(url string) (kdm.Data, error) {
	var data kdm.Data
	resp, err := httpClient.Get(url)
	if err != nil {
		return data, fmt.Errorf("driverMetadata err %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return data, fmt.Errorf("driverMetadata statusCode %v", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return data, fmt.Errorf("driverMetadata read response body error %v", err)
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return data, fmt.Errorf("driverMetadata %v", err)
	}
	return data, nil
}

func getDataGit(urlPath, branch string) (kdm.Data, error) {
	var data kdm.Data

	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		if err := os.MkdirAll(dataPath, 0755); err != nil {
			return data, fmt.Errorf("error creating directory %v", err)
		}
	}

	name, err := randomtoken.Generate()
	if err != nil {
		return data, fmt.Errorf("error generating metadata dirName %v", err)
	}

	path := fmt.Sprintf("%s/%s", dataPath, fmt.Sprintf("data-%s", name))
	if err := git.CloneWithDepth(path, urlPath, branch, 1); err != nil {
		return data, fmt.Errorf("error cloning repo %s %s: %v", urlPath, branch, err)
	}

	filePath := fmt.Sprintf("%s/%s", path, fileLoc)
	file, err := os.Open(filePath)
	if err != nil {
		return data, fmt.Errorf("error opening file %s %v", filePath, err)
	}
	defer file.Close()

	buf, err := ioutil.ReadAll(file)
	if err != nil {
		return data, fmt.Errorf("error reading file %s %v", filePath, err)
	}

	if err := json.Unmarshal(buf, &data); err != nil {
		return data, fmt.Errorf("error unmarshaling metadata contents %v", err)
	}

	if err := os.RemoveAll(path); err != nil {
		logrus.Errorf("error removing metadata path %s %v", path, err)
	}
	return data, nil
}

func getSettingValues(value string) (map[string]interface{}, error) {
	urlData := map[string]interface{}{}
	if err := json.Unmarshal([]byte(value), &urlData); err != nil {
		return nil, fmt.Errorf("unmarshal err %v", err)
	}
	return urlData, nil
}

func setFinalPath(url *MetadataURL) {
	if url.isGit {
		prevHash = url.latestHash
	}
}

func toSync(url *MetadataURL) bool {
	// check if hash changed for Git, can't do much for normal url
	if url.isGit {
		return prevHash != url.latestHash
	}
	return true
}

func deleteMap(url *MetadataURL) {
	key := getKey(url)
	fileMapLock.Lock()
	delete(fileMapData, key)
	fileMapLock.Unlock()
}

func storeMap(url *MetadataURL) bool {
	key := getKey(url)
	fileMapLock.Lock()
	defer fileMapLock.Unlock()
	if _, ok := fileMapData[key]; ok {
		return false
	}
	fileMapData[key] = true
	return true
}

func getKey(url *MetadataURL) string {
	if url.isGit {
		return url.latestHash
	}
	return url.path
}
