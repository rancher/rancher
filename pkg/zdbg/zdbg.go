package zdbg

import (
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"time"

	"context"

	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/v3/pkg/ticker"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

/*
*   Log operations that take longer than LoggingTriggerInterval.
*   LoggingTriggerInterval is specified in milliseconds and is set in configmap 'zks-config'
*   in 'cattle-system' namespace. (example below)
*
*	apiVersion: v1
*	kind: ConfigMap
*	metadata:
*  		name: zks-config
*  		namespace: cattle-system
*	data:
*  	  	LoggingTriggerInterval: "2000"
*
*
*  If 'zks-config' configmap is absent, then this interval defaults to DefaultLoggingTriggerInterval
 */

// Global Zededa configuration - we populate it from configmap mentioned above or use default values.
type zedConfig struct {
	mu   sync.RWMutex
	data zConfData
}

type zConfData struct {
	loggingTriggerInterval time.Duration
	//more stuff will go here later
}

const (
	ZksConfigmapName              = "zks-config"
	ZksConfigmapNamespace         = namespace.System //cattle-system
	ZksConfigPollInterval         = time.Minute * 5
	DefaultLoggingTriggerInterval = time.Second
	// String to make our added debugging prints pop out in the logs. Keep it out of .yaml for now.
	LogPrefix = "XXXX"
)

var (
	Zconf zedConfig // global config datastructue.
)

// returns copy of configuration data
func (z *zedConfig) getData() zConfData {
	z.mu.RLock()
	retval := z.data //retval is a copy
	z.mu.RUnlock()   //do not defer for performance reasons

	return retval
}

func (z *zedConfig) setData(d zConfData) {
	z.mu.Lock()
	z.data = d
	z.mu.Unlock() //do not defer for performance reasons
}

// Exported API.
// We can find the calling function using runtime package - but don't overcomplicate things for now -
// just print the passed-in message. The final format of the message will be something like
// "XXXX myCoolFunction() is done timer: 2s"
// where:
//
//	'XXXX' is the log prefix,
//	'myCoolFunction() is done' is the passed in message
//	'2s' is the time that passed since 'then' argument.
func Log(then time.Time, msg string) {
	elapsed := time.Since(then)
	data := Zconf.getData()

	if elapsed > data.loggingTriggerInterval {
		logrus.Printf("%s %s timer: %v \n", LogPrefix, msg, elapsed)
	}
}

// Exported API.
// Caller calls this function to initialize the ZKS config data
func InitZConfig(ctx context.Context) {
	zConf()

	//now spawn the go routine that will keep updating our global config
	go pollZConf(ctx, ZksConfigPollInterval)
}

// Set globabl Zconf to either hardcoded values or to values from 'zks-config' configmap
func zConf() {
	newConf, err := getZConfigMap()
	if err != nil {
		// did not process configmap successfully -
		logrus.Printf("%s could not process configmap %s in namespace %s, will default ZKS configuration values \n",
			LogPrefix, ZksConfigmapName, ZksConfigmapNamespace)
		newConf = getDefaultZConf()
	}

	//if values have changed, update Zconf
	oldConf := Zconf.getData()

	if !reflect.DeepEqual(oldConf, newConf) {
		Zconf.setData(newConf)
	}
}

// If Zededa configuration is not provided, get the default config.
func getDefaultZConf() zConfData {
	return zConfData{
		loggingTriggerInterval: DefaultLoggingTriggerInterval,
		// other fields will go here
	}
}

// Get values from zededa config map and convert it to zConfData
func getZConfigMap() (zConfData, error) {
	empty := zConfData{}
	cfg, err := kubeconfig.GetNonInteractiveClientConfig("").ClientConfig()
	if err != nil {
		return empty, err
	}
	k8s, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return empty, err
	}
	cm, err := k8s.CoreV1().ConfigMaps(ZksConfigmapNamespace).Get(context.Background(), ZksConfigmapName, metav1.GetOptions{})
	if err != nil {
		return empty, err
	}
	return cmToZConfData(cm)
}

// helper function to translate configmap to our zedConfig struct
func cmToZConfData(cm *v1.ConfigMap) (zConfData, error) {
	empty := zConfData{}
	v, ok := cm.Data["LoggingTriggerInterval"]
	if !ok {
		return empty, fmt.Errorf("%s value LoggingTriggerInterval does not exist in configmap ", LogPrefix)
	}
	LogTrigInterval, err := strconv.Atoi(v)
	if err != nil {
		return empty, fmt.Errorf("%s: error converting value to int: %w", LogPrefix, err)
	}
	logrus.Printf("%s: will set log trigger interval to %v \n", LogPrefix, LogTrigInterval)
	return zConfData{
		loggingTriggerInterval: time.Duration(LogTrigInterval) * time.Millisecond,
	}, nil
}

// this is the only writer of the config info.
// poll configmap every 'interval'
func pollZConf(ctx context.Context, interval time.Duration) {
	for range ticker.Context(ctx, interval) {
		zConf()
	}
}
