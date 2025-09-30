# Step linter

[![Step changelog](https://shields.io/github/v/release/bitrise-steplib/steps-check.git?include_prereleases&label=changelog&color=blueviolet)](https://github.com/bitrise-steplib/steps-check.git/releases)

Runs step linters


<details>
<summary>Description</summary>

Runs step linters

</details>

## 🧩 Get started

Add this step directly to your workflow in the [Bitrise Workflow Editor](https://devcenter.bitrise.io/steps-and-workflows/steps-and-workflows-index/).

You can also run this step directly with [Bitrise CLI](https://github.com/bitrise-io/bitrise).

## ⚙️ Configuration

<details>
<summary>Inputs</summary>

| Key | Description | Flags | Default |
| --- | --- | --- | --- |
| `step_dir` | Step directory path | required | `.` |
| `workflow` | Select the validation workflow to run | required | `lint unit_test` |
| `skip_step_yml_validation` | Skip step.yml validation |  | `no` |
| `skip_go_checks` | Skip golang related checks |  | `no` |
</details>

<details>
<summary>Outputs</summary>
There are no outputs defined in this step
</details>

## 🙋 Contributing

We welcome [pull requests](https://github.com/bitrise-steplib/steps-check.git/pulls) and [issues](https://github.com/bitrise-steplib/steps-check.git/issues) against this repository.

For pull requests, work on your changes in a forked repository and use the Bitrise CLI to [run step tests locally](https://devcenter.bitrise.io/bitrise-cli/run-your-first-build/).

Learn more about developing steps:

- [Create your own step](https://devcenter.bitrise.io/contributors/create-your-own-step/)
- [Testing your Step](https://devcenter.bitrise.io/contributors/testing-and-versioning-your-steps/)
