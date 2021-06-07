package main

import (
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/sliceutil"
)

//go:embed check.yml
var checkConfig []byte

const e2eWorkflow = "e2e"

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

	var err error
	config.WorkDir, err = pathutil.AbsPath(config.WorkDir)
	if err != nil {
		return fmt.Errorf("failed to expand path (%s): %v", config.WorkDir, err)
	}

	if len(config.Workflow) == 0 {
		return fmt.Errorf("no workflow specified")
	}

	runE2EWorkflow := false
	i := sliceutil.IndexOfStringInSlice(e2eWorkflow, config.Workflow)
	if i != -1 {
		runE2EWorkflow = true
		config.Workflow = append(config.Workflow[:i], config.Workflow[i+1:]...)
	}

	if err := os.Chdir(config.WorkDir); err != nil {
		return fmt.Errorf("failed to change working directory (%s): %v", config.WorkDir, err)
	}

	if runE2EWorkflow {
		log.Donef("Running '%s' workflow", e2eWorkflow)
		if err := runE2E(config.WorkDir); err != nil {
			return fmt.Errorf("workflow %s failed: %v", e2eWorkflow, err)
		}

		log.Donef("Check '%s' succeeded", e2eWorkflow)
	}

	// Run other, non-e2e workflows
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

		fmt.Println()
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
