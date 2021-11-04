package main

import (
	"fmt"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/segmentio/analytics-go"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

const unifiedCiAppID = "48fa8fbee698622c"

const (
	defaultBitriseSecretsName = ".bitrise.secrets.yml"
)

type partialBitriseModel struct {
	Workflows yaml.MapSlice `json:"workflows,omitempty" yaml:"workflows,omitempty"`
}

func runE2E(commandFactory command.Factory, workDir string, segmentKey string, parentURL string) error {
	e2eBitriseYMLPath := filepath.Join(workDir, "e2e", "bitrise.yml")
	if exists, err := pathutil.IsPathExists(e2eBitriseYMLPath); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("looking for bitrise.yml in e2e directory, path (%s) does not exists", e2eBitriseYMLPath)
	}

	log.Infof("Using bitrise.yml from: %s", e2eBitriseYMLPath)

	secrets, err := lookupSecrets(workDir)
	if err != nil {
		return err
	}

	if secrets == "" {
		log.Errorf("No %s found", defaultBitriseSecretsName)
	} else {
		log.Infof("Using secrets from: %s", secrets)
	}

	workflows, err := readE2EWorkflows(e2eBitriseYMLPath)
	if err != nil {
		return err
	}

	client := analytics.New(segmentKey)
	defer client.Close()
	for _, workflow := range workflows {
		err = runE2EWorkflow(commandFactory, workDir, e2eBitriseYMLPath, secrets, workflow)
		if parentURL != "" {
			if err := sendAnalytics(client, workflow, err == nil, parentURL); err != nil {
				return err
			}
		}
		if err != nil {
			return fmt.Errorf("failed to run workflow %s: %w", workflow, err)
		}
	}
	return nil
}

func sendAnalytics(client analytics.Client, workflow string, success bool, parentURL string) error {
	var status string
	if success {
		status = "success"
	} else {
		status = "error"
	}
	if err := client.Enqueue(analytics.Track{
		UserId: unifiedCiAppID,
		Event:  "ci_finished",
		Properties: map[string]interface{}{
			"workflow":   workflow,
			"status":     status,
			"parent_url": parentURL,
		},
	}); err != nil {
		return err
	}
	return nil
}

func readE2EWorkflows(configPath string) ([]string, error) {
	configBytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	return readE2EWorkflowsFromBytes(configBytes)
}

func readE2EWorkflowsFromBytes(configBytes []byte) ([]string, error) {
	model := partialBitriseModel{}
	if err := yaml.Unmarshal(configBytes, &model); err != nil {
		return nil, err
	}
	var result []string
	for _, workflow := range model.Workflows {
		key, ok := workflow.Key.(string)
		if !ok {
			return nil, fmt.Errorf("failed to cast workflow name to string")
		}
		if strings.HasPrefix(key, "test_") {
			result = append(result, key)
		}
	}
	return result, nil
}

func runE2EWorkflow(commandFactory command.Factory, workDir string, configPath string, secretsPath string, workflow string) error {
	e2eCmdArgs := []string{"run", "--config", configPath}
	if secretsPath != "" {
		e2eCmdArgs = append(e2eCmdArgs, "--inventory", secretsPath)
	}
	e2eCmdArgs = append(e2eCmdArgs, workflow)

	e2eCmd := commandFactory.Create(
		"bitrise",
		e2eCmdArgs,
		&command.Opts{
			Dir:    workDir,
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
		})
	fmt.Println()
	log.Donef("$ %s", e2eCmd.PrintableCommandArgs())

	if err := e2eCmd.Run(); err != nil {
		if errorutil.IsExitStatusError(err) {
			return err
		}

		return fmt.Errorf("failed to run command: %v", err)
	}
	return nil
}

func lookupSecrets(workDir string) (string, error) {
	secretLookupPaths := []string{
		filepath.Join(workDir, "e2e", defaultBitriseSecretsName),
		filepath.Join(workDir, defaultBitriseSecretsName),
	}
	for _, secretPath := range secretLookupPaths {
		if exists, err := pathutil.IsPathExists(secretPath); err != nil {
			return "", err
		} else if exists {
			return secretPath, nil
		}
	}

	return "", nil
}
