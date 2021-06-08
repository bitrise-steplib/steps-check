package main

import (
	"fmt"
	models2 "github.com/bitrise-io/envman/models"
	stepmodel "github.com/bitrise-io/stepman/models"
	"io/ioutil"
	"strings"

	"github.com/bitrise-io/bitrise/models"
	"gopkg.in/yaml.v2"
)

func parseBitriseConfigFromFile(configPath string) (models.BitriseDataModel, []string, error) {
	configString, err := ioutil.ReadFile(configPath)
	if err != nil {
		return models.BitriseDataModel{}, []string{}, fmt.Errorf("failed to open file, error: %s", err)
	}

	return parseBitriseConfigFromBytes(configString)
}

func parseBitriseConfigFromBytes(configBytes []byte) (models.BitriseDataModel, []string, error) {
	config, warnings, err := configModelFromYAMLBytes(configBytes)
	if err != nil {
		return models.BitriseDataModel{}, warnings, fmt.Errorf("failed to parse Bitrise config, error: %s", err)
	}

	return config, warnings, nil
}

func appendE2EExecutorWorkflow(bitriseConfig *models.BitriseDataModel, targetConfig string) *models.BitriseDataModel {
	e2eWorkflows := getE2EWorkflows(bitriseConfig.Workflows)
	executorWorkflow := createExecutorWorkflow(e2eWorkflows, targetConfig)
	bitriseConfig.Workflows[executorWorkflow.Title] = executorWorkflow
	return bitriseConfig
}

func writeOutBitriseConfig(fileName string, bitriseConfig models.BitriseDataModel) error {
	bitriseConfigBytes, err := yaml.Marshal(bitriseConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal Bitrise config, error: %s", err)
	}

	err = ioutil.WriteFile(fileName, bitriseConfigBytes, 0644)
	if err != nil {
		return fmt.Errorf("failed to write Bitrise config, error: %s", err)
	}

	return nil
}

func configModelFromYAMLBytes(configBytes []byte) (bitriseData models.BitriseDataModel, warnings []string, err error) {
	if err = yaml.Unmarshal(configBytes, &bitriseData); err != nil {
		return
	}

	warnings, err = normalizeValidateFillMissingDefaults(&bitriseData)
	if err != nil {
		return
	}

	return
}

func normalizeValidateFillMissingDefaults(bitriseData *models.BitriseDataModel) ([]string, error) {
	if err := bitriseData.Normalize(); err != nil {
		return []string{}, err
	}

	warnings, err := bitriseData.Validate()
	if err != nil {
		return warnings, err
	}

	return warnings, nil
}

func getE2EWorkflows(workflows map[string]models.WorkflowModel) (e2eTestWorkflows []string) {
	for workflow := range workflows {
		if strings.HasPrefix(workflow, "test_") {
			e2eTestWorkflows = append(e2eTestWorkflows, workflow)
		}
	}
	return
}

func createExecutorWorkflow(e2eWorkflows []string, targetConfig string) models.WorkflowModel {

	var itemModels []models.StepListItemModel
	for _, workflow := range e2eWorkflows {
		script := `#!/usr/bin/env bash
bitrise run ` + workflow + ` --config ` + targetConfig + `
`
		title := "Running " + workflow
		itemModels = append(itemModels, map[string]stepmodel.StepModel{"script@1": {
			Title: &title,
			Inputs: []models2.EnvironmentItemModel{
				map[string]interface{}{
					"content": script,
				},
			},
		}})
	}
	return models.WorkflowModel{
		Title: generatedE2EWorkflowName,
		Steps: itemModels,
	}
}
