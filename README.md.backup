# Shared internal checks

This repo contains shared (internal) checks and workflows for first-party Bitrise steps and other Bitrise codebases.

Why? To avoid duplication in ~100 step repos and to have a single source of truth for the checks and workflows.

### Legacy checks

This repo started as a custom Golang step that got included in step repo `bitrise.yml`s. Essentially, it runs [./checks.bitrise.yml](./checks.bitrise.yml).

There are still repos using this legacy pattern, but it's not recommended to add new checks there, just migrate the repos to the modern shared workflows discussed below.

### Modern shared workflows

The modern way to share checks and workflows is to use the [bitrise.yml include feature](https://docs.bitrise.io/en/bitrise-ci/configure-builds/configuration-yaml/modular-yaml-configuration.html).

This repo defines a couple of reusable step bundles and workflows, such as:

- [./yamlfmt.bitrise.yml](./yamlfmt.bitrise.yml)
- [./golang.bitrise.yml](./golang.bitrise.yml)
- [./step-metadata.bitrise.yml](./step-metadata.bitrise.yml)

#### Usage in step repos:

```yaml

include:
- repository: steps-check
  branch: master
  path: steps.bitrise.yml
```

`bitrise run check` executes the `check` workflow defined in [steps.bitrise.yml](./steps.bitrise.yml), which includes all step-related shared checks.

#### Usage in non-step repos:

Include the relevant pieces individually, then add them to the desired workflow. For example:

```yaml
include:
- repository: steps-check
  branch: master
  path: yamlfmt.bitrise.yml
- repository: steps-check
  branch: master
  path: golang.bitrise.yml

workflows:
  pr-validation:
    steps:
    # [...] other steps
    - bundle::yamlfmt: {}
    - bundle::golangci-lint:
        inputs:
        - golangci_lint_version: 2.11.4
    - bundle::go-test: {}
