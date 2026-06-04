package main

import (
	"strings"
	"testing"
)

func TestInlineIncludesMergesSharedKeyAndDropsInclude(t *testing.T) {
	sources := map[string]string{
		"main.bitrise.yml": `format_version: "11"

include:
- path: ./common.bitrise.yml

step_bundles:
  build:
    steps:
    - script@1: {}
`,
		"common.bitrise.yml": `step_bundles:
  download-file:
    steps:
    - script@1: {}
`,
	}

	out, err := inlineIncludes("main.bitrise.yml", sources)
	if err != nil {
		t.Fatalf("inlineIncludes: %v", err)
	}

	if strings.Contains(out, "include:") {
		t.Errorf("include directive was not dropped:\n%s", out)
	}
	if strings.Count(out, "step_bundles:") != 1 {
		t.Errorf("expected a single merged step_bundles key, got:\n%s", out)
	}
	for _, want := range []string{"build:", "download-file:", `format_version: "11"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in merged output:\n%s", want, out)
		}
	}
}

func TestInlineIncludesAppendsIncludedOnlyKeyAndResolvesRecursively(t *testing.T) {
	sources := map[string]string{
		"main.bitrise.yml": `include:
- path: a.bitrise.yml

workflows:
  ci: {}
`,
		"a.bitrise.yml": `include:
- path: b.bitrise.yml

step_bundles:
  a-bundle: {}
`,
		"b.bitrise.yml": `step_bundles:
  b-bundle: {}
`,
	}

	out, err := inlineIncludes("main.bitrise.yml", sources)
	if err != nil {
		t.Fatalf("inlineIncludes: %v", err)
	}

	// step_bundles only exists in the included files, so it is appended once with
	// both nested-include bundles merged under it.
	if strings.Count(out, "step_bundles:") != 1 {
		t.Errorf("expected a single step_bundles key, got:\n%s", out)
	}
	for _, want := range []string{"ci:", "a-bundle:", "b-bundle:"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in merged output:\n%s", want, out)
		}
	}
}

func TestInlineIncludesErrorsOnMissingSource(t *testing.T) {
	sources := map[string]string{
		"main.bitrise.yml": "include:\n- path: ./missing.bitrise.yml\n",
	}
	if _, err := inlineIncludes("main.bitrise.yml", sources); err == nil {
		t.Fatal("expected an error for a missing embedded include source")
	}
}
