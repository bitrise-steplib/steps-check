package main

import (
	_ "embed"
	"fmt"
	"github.com/bitrise-io/go-utils/env"
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

var checkConfig = `format_version: 11
default_step_lib_source: https://github.com/bitrise-io/bitrise-steplib.git

workflows:
  lint:
    steps:
    - change-workdir@1:
        inputs: 
        - path: $STEP_DIR
    - script:
        title: YAML lint
        inputs:
        - content: |-
            #!/bin/env bash
            set -ex
            pip3 install yamllint
            yamllint --format colored . # Config file is implicitly set via $YAMLLINT_CONFIG_FILE
    - script@1:
        title: Audit step
        run_if: '{{enveq "SKIP_STEP_YML_VALIDATION" "false"}}'
        inputs:
        - content: |-
            #!/bin/env bash
            set -ex
            pwd

            stepman audit --step-yml ./step.yml
    - git::https://github.com/bitrise-steplib/steps-validate-json-schema.git@step-init:
        title: Validate JSON schema
        inputs:
        - schema_url: https://raw.githubusercontent.com/bitrise-io/bitrise-json-schemas/steplib-review-rules/step.schema.json
        - yaml_path: ./step.yml
        - warning_patterns: |-
            I\[#\] S\[#/additionalProperties\] additionalProperties .+ not allowed
            I\[#\] S\[#/required\] missing properties: .+
            I\[#/summary\] S\[#/properties/summary/pattern\] does not match pattern "\^\.\{1,100\}\$"
            I\[#/deps/(brew|apt_get)+/\d+/(name|bin_name)+\] S\[#/definitions/(BrewDepModel|AptGetDepModel)+/properties/(name|bin_name)+/not\] not failed
            I\[#/(inputs|outputs)+/\d+/opts\] S\[#/definitions/EnvVarOpts/required\] missing properties: "summary"
            I\[#/(inputs|outputs)+/\d+/opts/summary\] S\[#/definitions/EnvVarOpts/properties/summary/minLength\] length must be >= 1, but got 0
            I\[#/inputs/\d+/.+\] S\[#/definitions/InputEnvVar/additionalProperties/type\] expected .+, but got .+
            I\[#/inputs/\d+/opts/value_options\] S\[#/definitions/EnvVarOpts/properties/value_options/minItems\] minimum 2 items allowed, but found \d+ items
    - script@1:
        title: Run golangci-lint
        inputs:
        - content: |-
            #!/bin/env bash
            set -xeo pipefail
            curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.42.1
            golangci-lint run

  unit_test:
    steps:
    - change-workdir@1:
        inputs: 
        - path: $STEP_DIR
    - go-list: {}
    - go-test: {}
`

//go:embed .yamllint.yml
var yamllintConfig string

const e2eWorkflow = "e2e"
const yamllintEnvKey = "YAMLLINT_CONFIG_FILE"

// Config ...
type Config struct {
	WorkDir               string   `env:"step_dir,dir"`
	Workflow              []string `env:"workflow,multiline"`
	SkipStepYMLValidation bool     `env:"skip_step_yml_validation,opt[yes,no]"`
	SegmentWriteKey       string   `env:"SEGMENT_WRITE_KEY,required"`
	ParentBuildURL        string   `env:"PARENT_BUILD_URL"`
}

func mainR() error {
	envRepository := env.NewRepository()
	commandFactory := command.NewFactory(envRepository)
	var config Config
	configParser := stepconf.NewInputParser(envRepository)
	if err := configParser.Parse(&config); err != nil {
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
		if err := runE2E(commandFactory, config.WorkDir, config.SegmentWriteKey, config.ParentBuildURL); err != nil {
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

	yamllintPath := filepath.Join(tmpDir, ".yamllint.yml")
	if err := ioutil.WriteFile(yamllintPath, []byte(yamllintConfig), 0600); err != nil {
		return err
	}
	if err := os.Setenv(yamllintEnvKey, yamllintPath); err != nil {
		return err
	}

	for _, wf := range config.Workflow {
		workflowCmd :=
			commandFactory.Create(
				"bitrise",
				[]string{"run", wf, "--config", configPath},
				&command.Opts{
					Dir: config.WorkDir,
					Env: []string{
						fmt.Sprintf("STEP_DIR=%s", config.WorkDir),
						fmt.Sprintf("SKIP_STEP_YML_VALIDATION=%t", config.SkipStepYMLValidation)},
					Stdout: os.Stdout,
					Stderr: os.Stderr,
				})
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
