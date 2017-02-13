package clients

import (
	"io/ioutil"

	"github.com/cloudfoundry/bosh-cli/director"
	"github.com/cloudfoundry/bosh-cli/uaa"
	"github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-utils/logger"
)

func anonymousUserConfig(host string, port int, CACert string) director.Config {
	return director.Config{
		Host:   host,
		Port:   port,
		CACert: CACert,
	}
}

func userConfig(host string, port int, CACert, username, password string) director.Config {
	config := anonymousUserConfig(host, port, CACert)
	config.Client = username
	config.ClientSecret = password
	return config
}

func GetDirector(host string, port int, username, password, caCertFile string) (director.Director, error) {

	logger := logger.NewLogger(0)
	caCertBytes, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		return nil, err
	}
	config := uaa.Config{
		Host:         host,
		Port:         port,
		CACert:       string(caCertBytes),
		Client:       username,
		ClientSecret: password,
	}

	info, err := getInfo(config.Host, config.Port, config.CACert, logger)
	if err != nil {
		return nil, err
	}

	dirConfig := userConfig(config.Host, config.Port, config.CACert, config.Client, config.ClientSecret)

	if info.Auth.Type == "uaa" {
		uaaClient, err := getUaa(info, config.Client, config.ClientSecret, config.CACert, logger)
		if err != nil {
			return nil, err
		}

		dirConfig.Client = ""
		dirConfig.ClientSecret = ""

		dirConfig.TokenFunc = uaa.NewClientTokenSession(uaaClient).TokenFunc
	}

	taskReporter := director.NewNoopTaskReporter()
	fileReporter := director.NewNoopFileReporter()

	return director.NewFactory(logger).New(dirConfig, taskReporter, fileReporter)
}

func getUaa(info director.Info, client, clientSecret, CACert string, logger logger.Logger) (uaa.UAA, error) {
	uaaURL := info.Auth.Options["url"]

	uaaURLStr, ok := uaaURL.(string)
	if !ok {
		return nil, errors.Errorf("Expected URL '%s' to be a string", uaaURL)
	}

	uaaConfig, err := uaa.NewConfigFromURL(uaaURLStr)
	if err != nil {
		return nil, err
	}

	uaaConfig.CACert = CACert
	uaaConfig.Client = client
	uaaConfig.ClientSecret = clientSecret

	if len(uaaConfig.Client) == 0 {
		uaaConfig.Client = "downtimer"
	}

	return uaa.NewFactory(logger).New(uaaConfig)
}

func getInfo(host string, port int, CACert string, logger logger.Logger) (director.Info, error) {
	dirConfig := anonymousUserConfig(host, port, CACert)
	directorClient, err := director.NewFactory(logger).New(dirConfig, nil, nil)
	if err != nil {
		return director.Info{}, err
	}
	return directorClient.Info()
}