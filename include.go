package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// topLevelKeyRE matches a top-level YAML mapping key (no leading indentation),
// e.g. `workflows:` or `format_version: "11"`.
var topLevelKeyRE = regexp.MustCompile(`^([A-Za-z0-9_.\-]+):`)

// pathLineRE extracts the `path:` value of an `include:` list entry.
var pathLineRE = regexp.MustCompile(`^\s*-?\s*path:\s*(\S+)`)

// section is a top-level YAML key together with its raw lines (the header line
// plus every following line until the next top-level key). Keeping the lines
// verbatim preserves comments, formatting and embedded shell scripts.
type section struct {
	key   string
	lines []string
}

// inlineIncludes resolves the `include:` directives of an embedded compatibility
// config by dumbly splicing the referenced configs' content into it. The config
// is treated as a template: the `include:` block is dropped and, for every
// top-level key the included file shares with the template (in practice
// `step_bundles`), the included file's entries are appended under the template's
// key. Keys that only the included file has are appended as new sections.
//
// We do this in-process instead of letting bitrise resolve the includes, because
// bitrise resolves relative `include: ./common.bitrise.yml` paths against the
// config file's directory, which is awkward for our embedded, temp-dir-written
// configs.
func inlineIncludes(name string, sources map[string]string) (string, error) {
	content, ok := sources[name]
	if !ok {
		return "", fmt.Errorf("config %q is not embedded", name)
	}

	var includeTargets []string
	var kept []section
	for _, sec := range parseSections(content) {
		if sec.key == "include" {
			includeTargets = append(includeTargets, includePaths(sec)...)
			continue
		}
		kept = append(kept, sec)
	}

	for _, target := range includeTargets {
		// Resolve the included file's own includes first.
		resolved, err := inlineIncludes(target, sources)
		if err != nil {
			return "", err
		}
		kept = mergeSections(kept, parseSections(resolved))
	}

	return renderSections(kept), nil
}

// parseSections splits content into top-level sections, attaching blank/comment
// and indented lines to the section above them.
func parseSections(content string) []section {
	var sections []section
	cur := -1
	for _, line := range strings.Split(content, "\n") {
		if m := topLevelKeyRE.FindStringSubmatch(line); m != nil {
			sections = append(sections, section{key: m[1], lines: []string{line}})
			cur = len(sections) - 1
			continue
		}
		if cur == -1 {
			// Leading blank/comment lines before any key.
			sections = append(sections, section{lines: []string{line}})
			cur = 0
			continue
		}
		sections[cur].lines = append(sections[cur].lines, line)
	}
	return sections
}

// includePaths returns the embedded file names referenced by an `include:` section.
func includePaths(sec section) []string {
	var paths []string
	for _, line := range sec.lines {
		if m := pathLineRE.FindStringSubmatch(line); m != nil {
			paths = append(paths, filepath.Base(m[1]))
		}
	}
	return paths
}

// mergeSections appends each src section into dst: shared top-level keys get the
// src section's child lines appended; new keys are appended as whole sections.
func mergeSections(dst, src []section) []section {
	for _, s := range src {
		if s.key == "" {
			continue // leading blank/comment block of the included file
		}
		if i := indexOfSection(dst, s.key); i >= 0 {
			// Append the included entries (everything after the header line)
			// under the existing key.
			dst[i].lines = append(dst[i].lines, s.lines[1:]...)
			continue
		}
		dst = append(dst, s)
	}
	return dst
}

func indexOfSection(sections []section, key string) int {
	for i, s := range sections {
		if s.key == key {
			return i
		}
	}
	return -1
}

func renderSections(sections []section) string {
	var b strings.Builder
	for _, s := range sections {
		for _, line := range s.lines {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}
