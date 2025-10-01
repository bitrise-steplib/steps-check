# Step linter

[![Step changelog](https://shields.io/github/v/release/bitrise-steplib/steps-check.git?include_prereleases&label=changelog&color=blueviolet)](https://github.com/bitrise-steplib/steps-check.git/releases)

Runs step linters


<details>
<summary>Description</summary>

Runs step linters

</details>

## 🧩 Get started

Add this step directly to your workflow in the [Bitrise Workflow Editor](https://docs.bitrise.io/en/bitrise-ci/workflows-and-pipelines/steps/adding-steps-to-a-workflow.html).

You can also run this step directly with [Bitrise CLI](https://github.com/bitrise-io/bitrise).

## ⚙️ Configuration

<details>
<summary>Inputs</summary>

| Key | Description | Flags | Default |
| --- | --- | --- | --- |
| `step_dir` | Step diresctory path | required | `.` |
| `workflow` | Select the validation workflow to run | required | `lint unit_test` |
| `skip_step_yml_validation` | Skip step.yml and README validation |  | `no` |
| `skip_go_checks` | Skip golang related checks |  | `no` |
</details>

<details>
<summary>Outputs</summary>
There are no outputs defined in this step
</details>

## 🙋 Contributing

We welcome [pull requests](https://github.com/bitrise-steplib/steps-check.git/pulls) and [issues](https://github.com/bitrise-steplib/steps-check.git/issues) against this repository.

For pull requests, work on your changes in a forked repository and use the Bitrise CLI to [run step tests locally](https://docs.bitrise.io/en/bitrise-ci/bitrise-cli/running-your-first-local-build-with-the-cli.html).

Learn more about developing steps:

- [Create your own step](https://docs.bitrise.io/en/bitrise-ci/workflows-and-pipelines/developing-your-own-bitrise-step/developing-a-new-step.html)
