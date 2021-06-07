package main

import (
	"os"
	"path/filepath"

	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/log"
)

// Config ...
type Config struct {
	WorkDir string `env:"PACKAGE_SOURCE_PATH,required"`
}

func run() error {
	var conf Config
	if err := stepconf.Parse(&conf); err != nil {
		return err
	}

	e2eBitriseYMLPath := filepath.Join(conf.WorkDir, "e2e", "bitrise.yml")

	bitriseConfig, warnings, err := parseBitriseConfigFromFile(e2eBitriseYMLPath)
	if err != nil {
		return err
	}

	for _, warning := range warnings {
		log.Warnf(warning)
	}

	appendE2EExecutorWorkflow(&bitriseConfig)

	err = writeOutBitriseConfig(e2eBitriseYMLPath, bitriseConfig)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Errorf("e2e failed: %s", err)
		os.Exit(1)
	}
}
