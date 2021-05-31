package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/log"
)

const checkConfig = `format_version: 11
default_step_lib_source: https://github.com/bitrise-io/bitrise-steplib.git

workflows:
  lint:
    steps:
    - change-workdir:
        inputs: 
        - path: $STEP_DIR
    - script:
        inputs:
        - content: |-
            #!/bin/env bash
            set -ex
            pwd
            stepman audit --step-yml ./step.yml
    - go-list: {}
    - golint: {}
    - errcheck: {}

  unit_test:
    steps:
    - change-workdir:
        inputs: 
        - path: $STEP_DIR
    - go-list: {}
    - go-test: {}`

// Config ...
type Config struct {
	WorkDir  string   `env:"step_dir,dir"`
	Workflow []string `env:"workflow,multiline"`
}

func mainR() error {
	var config Config
	if err := stepconf.Parse(&config); err != nil {
		return fmt.Errorf("Invalid inputs: %v", err)
	}

	stepconf.Print(config)

	if len(config.Workflow) == 0 {
		return fmt.Errorf("no workflow specified")
	}

	if err := os.Chdir(config.WorkDir); err != nil {
		return fmt.Errorf("failed to change working directory (%s): %v", config.WorkDir, err)
	}

	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}

	configPath := filepath.Join(tmpDir, "bitrise.yml")
	if err := ioutil.WriteFile(configPath, []byte(checkConfig), 0600); err != nil {
		return err
	}

	for _, wf := range config.Workflow {
		workflowCmd := command.NewWithStandardOuts("bitrise", "run", wf, "--config", configPath)
		workflowCmd.SetDir(config.WorkDir)
		workflowCmd.AppendEnvs(fmt.Sprintf("STEP_DIR=%s", config.WorkDir))

		log.Donef("$ %s", workflowCmd.PrintableCommandArgs())
		if err := workflowCmd.Run(); err != nil {
			if errorutil.IsExitStatusError(err) {
				return fmt.Errorf("workflow %s failed: %v", wf, err)
			}
			return fmt.Errorf("failed to run command: %v", err)
		}

		log.Donef("Check '%s' succeeded", wf)
	}

	return nil
}

func main() {
	if err := mainR(); err != nil {
		log.Errorf("%s", err)
		os.Exit(1)
	}
}
