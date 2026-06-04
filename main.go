package main

import (
	_ "embed"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/env"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/sliceutil"
)

//go:embed checks.bitrise.yml
var checkConfig string

//go:embed .yamllint.yml
var yamllintConfig string

const e2eWorkflow = "e2e"
const yamllintEnvKey = "YAMLLINT_CONFIG_FILE"
const golangciLintVersionEnvKey = "GOLANGCI_LINT_VERSION"
const golangciLintConfigFileEnvKey = "GOLANGCI_LINT_CONFIG_FILE"
const yamlfmtExcludeEnvKey = "YAMLFMT_EXCLUDE"

// bitriseRepositoryURLEnvKey is read by bitrise to resolve `repository:` includes.
// We set it to a bitrise-steplib URL so the compatibility config's
// `repository: steps-check` includes resolve regardless of the consumer repo's org.
const bitriseRepositoryURLEnvKey = "BITRISE_CURRENT_REPOSITORY_URL"
const stepsCheckRepositoryURL = "https://github.com/bitrise-steplib/steps-check.git"

const defaultConfigName = "bitrise.yml"
const compatibilityConfigName = "compatibility.bitrise.yml"

// compatibilityConfig pulls the shared linter workflows (yamlfmt, golangci-lint)
// from the steps-check repo via repository-based includes. The `repository:
// steps-check` includes normally only resolve within the bitrise-steplib org;
// faking BITRISE_CURRENT_REPOSITORY_URL (see above) lets repos in any GitHub org
// use them.
const compatibilityConfig = `format_version: "11"
default_step_lib_source: https://github.com/bitrise-io/bitrise-steplib.git
include:
- repository: steps-check
  branch: master
  path: yamlfmt.bitrise.yml
- repository: steps-check
  branch: master
  path: golang.bitrise.yml
`

// compatibilityWorkflows are the linter workflows defined in the included
// steps-check configs; the step runs them against compatibilityConfig instead of
// the legacy checks.bitrise.yml.
var compatibilityWorkflows = map[string]bool{
	"yamlfmt":       true,
	"golangci-lint": true,
}

// Config ...
type Config struct {
	WorkDir               string   `env:"step_dir,dir"`
	Workflow              []string `env:"workflow,multiline"`
	SkipStepYMLValidation bool     `env:"skip_step_yml_validation,opt[yes,no]"`
	SkipGoChecks          bool     `env:"skip_go_checks,opt[yes,no]"`
	GolangciLintVersion   string   `env:"golangci_lint_version"`
	GolangciLintConfig    string   `env:"golangci_lint_config_file"`
	YamlfmtExclude        string   `env:"yamlfmt_exclude"`
	SegmentWriteKey       string   `env:"SEGMENT_WRITE_KEY"`
	ParentBuildURL        string   `env:"PARENT_BUILD_URL"`
	IsCI                  bool     `env:"CI"`
	IsPR                  bool     `env:"PR"`
}

func mainR() error {
	envRepository := env.NewRepository()
	commandFactory := command.NewFactory(envRepository)
	var config Config
	configParser := stepconf.NewInputParser(envRepository)
	if err := configParser.Parse(&config); err != nil {
		return fmt.Errorf("invalid inputs: %v", err)
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
		shouldFailOnFirstError := !config.IsCI || config.IsPR
		if err := runE2E(commandFactory, config.WorkDir, shouldFailOnFirstError, config.SegmentWriteKey, config.ParentBuildURL); err != nil {
			return fmt.Errorf("workflow %s failed: %w", e2eWorkflow, err)
		}

		log.Donef("Check '%s' succeeded", e2eWorkflow)
	}

	// Run other, non-e2e workflows
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}

	// Legacy checks config (has no includes).
	configPath := filepath.Join(tmpDir, defaultConfigName)
	if err := ioutil.WriteFile(configPath, []byte(checkConfig), 0600); err != nil {
		return err
	}

	// Compatibility config: pulls the shared linter workflows from the steps-check
	// repo via repository-based includes (resolved with BITRISE_CURRENT_REPOSITORY_URL).
	compatConfigPath := filepath.Join(tmpDir, compatibilityConfigName)
	if err := ioutil.WriteFile(compatConfigPath, []byte(compatibilityConfig), 0600); err != nil {
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
		// Run the newer, include-based linters against the compatibility config;
		// everything else uses the legacy checks.bitrise.yml.
		wfConfigPath := configPath
		workflowEnv := []string{
			fmt.Sprintf("STEP_DIR=%s", config.WorkDir),
			fmt.Sprintf("SKIP_STEP_YML_VALIDATION=%t", config.SkipStepYMLValidation),
			fmt.Sprintf("SKIP_GO_CHECKS=%t", config.SkipGoChecks),
			fmt.Sprintf("%s=%s", golangciLintVersionEnvKey, config.GolangciLintVersion),
			fmt.Sprintf("%s=%s", golangciLintConfigFileEnvKey, config.GolangciLintConfig),
			fmt.Sprintf("%s=%s", yamlfmtExcludeEnvKey, config.YamlfmtExclude),
		}
		if compatibilityWorkflows[wf] {
			wfConfigPath = compatConfigPath
			// Fake the bitrise-steplib org so the `repository: steps-check` includes
			// resolve even when the consumer repo lives in a different GitHub org.
			workflowEnv = append(workflowEnv, fmt.Sprintf("%s=%s", bitriseRepositoryURLEnvKey, stepsCheckRepositoryURL))
		}

		workflowCmd :=
			commandFactory.Create(
				"bitrise",
				[]string{"run", wf, "--config", wfConfigPath},
				&command.Opts{
					Dir:    config.WorkDir,
					Env:    workflowEnv,
					Stdout: os.Stdout,
					Stderr: os.Stderr,
				})
		fmt.Println()
		log.Donef("$ %s", workflowCmd.PrintableCommandArgs())
		if err := workflowCmd.Run(); err != nil {
			if errorutil.IsExitStatusError(err) {
				return fmt.Errorf("workflow %s failed: %w", wf, err)
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

		var exitErr *exec.ExitError
		if ok := errors.As(err, &exitErr); ok {
			os.Exit(exitErr.ExitCode())
		}

		os.Exit(1)
	}
}
