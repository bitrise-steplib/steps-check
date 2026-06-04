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
```

#### Usage from a different GitHub org (compatibility mode)

The `include:` mechanism above only resolves for repos in the **same** GitHub org as `steps-check`. Repos in a different org (e.g. `bitrise-io`) can't pull in these `bitrise.yml`s via `include:`, so they run the same checks through the legacy Golang step instead.

The step embeds `yamlfmt.bitrise.yml` and `golang.bitrise.yml` and runs a dedicated compatibility workflow for each linter. Select them with the `workflow` input (it's multiline, so you can run several in one step):

```yaml
workflows:
  pr-validation:
    steps:
    # [...] other steps
    - git::https://github.com/bitrise-steplib/steps-check.git@master:
        inputs:
        - workflow: |-
            yamlfmt
            golangci-lint
        # Required by the golangci-lint workflow:
        - golangci_lint_version: 2.11.4
        # Optional: use the repo's own golangci-lint config instead of the shared one:
        - golangci_lint_config_file: .golangci.yml
        # Optional: override the yamlfmt exclude globs:
        - yamlfmt_exclude: _tmp/**,.git/**,.github/**,vendor/**
```

Available compatibility workflows:

- `yamlfmt` — runs `bundle::yamlfmt` (configurable via `yamlfmt_exclude`)
- `golangci-lint` — runs `bundle::golangci-lint` (requires `golangci_lint_version`; `golangci_lint_config_file` optionally overrides the shared config, otherwise the shared one is downloaded)
