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

//go:embed common.bitrise.yml
var commonConfig string

//go:embed golang.bitrise.yml
var golangConfig string

//go:embed yamlfmt.bitrise.yml
var yamlfmtConfig string

//go:embed .yamllint.yml
var yamllintConfig string

const e2eWorkflow = "e2e"
const yamllintEnvKey = "YAMLLINT_CONFIG_FILE"
const golangciLintVersionEnvKey = "GOLANGCI_LINT_VERSION"
const golangciLintConfigFileEnvKey = "GOLANGCI_LINT_CONFIG_FILE"
const yamlfmtExcludeEnvKey = "YAMLFMT_EXCLUDE"

const defaultConfigName = "bitrise.yml"

// includeSources maps embedded file names to their contents so the compatibility
// configs' `include:` directives can be resolved in-process (see inlineIncludes),
// instead of relying on bitrise to resolve relative includes from the temp dir.
var includeSources = map[string]string{
	"common.bitrise.yml":  commonConfig,
	"golang.bitrise.yml":  golangConfig,
	"yamlfmt.bitrise.yml": yamlfmtConfig,
}

// compatibilityWorkflows maps the newer, include-based linter workflows to the
// embedded bitrise.yml that defines a dedicated compatibility workflow for them. Repos
// in a different GitHub org than steps-check can't use the modular `include:`
// mechanism, so this step runs these workflows against the embedded configs instead of
// the legacy checks.bitrise.yml.
var compatibilityWorkflows = map[string]string{
	"yamlfmt":       "yamlfmt.bitrise.yml",
	"golangci-lint": "golang.bitrise.yml",
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
	if err := ioutil.WriteFile(filepath.Join(tmpDir, defaultConfigName), []byte(checkConfig), 0600); err != nil {
		return err
	}

	// Compatibility configs: resolve their `include:`s in-process and write them out
	// self-contained, so bitrise doesn't have to resolve relative includes itself.
	for _, configName := range compatibilityWorkflows {
		inlined, err := inlineIncludes(configName, includeSources)
		if err != nil {
			return fmt.Errorf("failed to resolve includes in %s: %w", configName, err)
		}
		if err := ioutil.WriteFile(filepath.Join(tmpDir, configName), []byte(inlined), 0600); err != nil {
			return err
		}
	}

	yamllintPath := filepath.Join(tmpDir, ".yamllint.yml")
	if err := ioutil.WriteFile(yamllintPath, []byte(yamllintConfig), 0600); err != nil {
		return err
	}
	if err := os.Setenv(yamllintEnvKey, yamllintPath); err != nil {
		return err
	}

	for _, wf := range config.Workflow {
		// Run the newer, include-based linters against their embedded compatibility
		// config; everything else uses the legacy checks.bitrise.yml.
		configName := defaultConfigName
		if compatConfigName, ok := compatibilityWorkflows[wf]; ok {
			configName = compatConfigName
		}
		wfConfigPath := filepath.Join(tmpDir, configName)

		workflowCmd :=
			commandFactory.Create(
				"bitrise",
				[]string{"run", wf, "--config", wfConfigPath},
				&command.Opts{
					Dir: config.WorkDir,
					Env: []string{
						fmt.Sprintf("STEP_DIR=%s", config.WorkDir),
						fmt.Sprintf("SKIP_STEP_YML_VALIDATION=%t", config.SkipStepYMLValidation),
						fmt.Sprintf("SKIP_GO_CHECKS=%t", config.SkipGoChecks),
						fmt.Sprintf("%s=%s", golangciLintVersionEnvKey, config.GolangciLintVersion),
						fmt.Sprintf("%s=%s", golangciLintConfigFileEnvKey, config.GolangciLintConfig),
						fmt.Sprintf("%s=%s", yamlfmtExcludeEnvKey, config.YamlfmtExclude),
					},

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
