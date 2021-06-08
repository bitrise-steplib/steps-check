package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
)

const (
	generatedE2EWorkflowName  = "e2e_test_executor_workflow"
	defaultBitriseSecretsName = ".bitrise.secrets.yml"
)

func runE2E(workDir string) error {
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

	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %v", err)
	}

	targetConfig := filepath.Join(tempDir, "bitrise_e2e.yml")

	if err := generateE2EWorkflow(e2eBitriseYMLPath, targetConfig); err != nil {
		return err
	}

	e2eCmdArgs := []string{"run", "--config", targetConfig}
	if secrets != "" {
		e2eCmdArgs = append(e2eCmdArgs, "--inventory", secrets)
	}
	e2eCmdArgs = append(e2eCmdArgs, generatedE2EWorkflowName)

	e2eCmd := command.NewWithStandardOuts("bitrise", e2eCmdArgs...)
	e2eCmd.SetDir(workDir)

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

func generateE2EWorkflow(sourceConfig, targetConfig string) error {
	bitriseConfig, warnings, err := parseBitriseConfigFromFile(sourceConfig)
	if err != nil {
		return fmt.Errorf("failed to parse e2e test config (%s): %v", sourceConfig, err)
	}

	for _, warning := range warnings {
		log.Warnf(warning)
	}

	appendE2EExecutorWorkflow(&bitriseConfig, targetConfig)

	if err = writeOutBitriseConfig(targetConfig, bitriseConfig); err != nil {
		return fmt.Errorf("failed to write e2e test workflow to path (%s): %v", targetConfig, err)
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
