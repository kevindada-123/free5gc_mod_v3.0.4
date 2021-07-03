/*
 * AMF Configuration Factory
 */

package factory

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"

	"free5gc/src/smpcf/logger"
)

var SmpcfConfig Config
var UERoutingConfig RoutingConfig

func checkErr(err error) {
	if err != nil {
		err = fmt.Errorf("[Configuration] %s", err.Error())
		logger.AppLog.Fatal(err)
	}
}

// TODO: Support configuration update from REST api
func InitConfigFactory(f string) {
	content, err := ioutil.ReadFile(f)
	checkErr(err)

	SmpcfConfig = Config{}

	err = yaml.Unmarshal([]byte(content), &SmpcfConfig)
	checkErr(err)

	logger.InitLog.Infof("Successfully initialize configuration %s", f)
}

func InitRoutingConfigFactory(f string) {
	content, err := ioutil.ReadFile(f)
	checkErr(err)

	UERoutingConfig = RoutingConfig{}

	err = yaml.Unmarshal([]byte(content), &UERoutingConfig)
	checkErr(err)

	logger.InitLog.Infof("Successfully initialize configuration %s", f)

}
