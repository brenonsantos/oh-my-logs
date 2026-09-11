package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PathCompletionResult contains the results of an autocomplete lookup on a filesystem path.
type PathCompletionResult struct {
	Completed     string   // updated input string with completion applied
	Matches       []string // list of matching entry names (with trailing '/' for directories)
	CommonPrefix  string   // longest common prefix among all matches
	Dir           string   // the directory part that was read
	InputPrefix   string   // path prefix prepended before match names (e.g. "~/Documents/")
	IsExactSingle bool     // true if there was exactly one match
}

// CompletePath inspects a partial path input and returns matching filesystem entries and completions.
func CompletePath(input string, baseDir string) PathCompletionResult {
	res := PathCompletionResult{
		Completed: input,
	}

	trimmed := strings.TrimSpace(input)
	hasTilde := strings.HasPrefix(trimmed, "~/") || trimmed == "~"

	if trimmed == "~" {
		res.Completed = "~/"
		res.Matches = []string{"~/"}
		res.InputPrefix = ""
		res.IsExactSingle = true
		return res
	}

	var searchDir string
	var partial string
	var inputPrefix string

	if hasTilde {
		expanded := expandHomePath(trimmed)
		if strings.HasSuffix(trimmed, "/") {
			searchDir = expanded
			partial = ""
			inputPrefix = trimmed
		} else {
			searchDir = filepath.Dir(expanded)
			partial = filepath.Base(expanded)
			lastSlash := strings.LastIndex(trimmed, "/")
			if lastSlash >= 0 {
				inputPrefix = trimmed[:lastSlash+1]
			} else {
				inputPrefix = "~/"
			}
		}
	} else if filepath.IsAbs(trimmed) {
		if strings.HasSuffix(trimmed, "/") {
			searchDir = trimmed
			partial = ""
			inputPrefix = trimmed
		} else {
			searchDir = filepath.Dir(trimmed)
			partial = filepath.Base(trimmed)
			lastSlash := strings.LastIndex(trimmed, "/")
			if lastSlash == 0 {
				inputPrefix = "/"
			} else if lastSlash > 0 {
				inputPrefix = trimmed[:lastSlash+1]
			}
		}
	} else {
		// Relative path
		effectiveBase := baseDir
		if effectiveBase == "" {
			effectiveBase, _ = os.Getwd()
		}
		if trimmed == "" {
			searchDir = effectiveBase
			partial = ""
			inputPrefix = ""
		} else if strings.HasSuffix(trimmed, "/") {
			searchDir = filepath.Join(effectiveBase, trimmed)
			partial = ""
			inputPrefix = trimmed
		} else {
			searchDir = filepath.Join(effectiveBase, filepath.Dir(trimmed))
			partial = filepath.Base(trimmed)
			lastSlash := strings.LastIndex(trimmed, "/")
			if lastSlash >= 0 {
				inputPrefix = trimmed[:lastSlash+1]
			} else {
				inputPrefix = ""
			}
		}
	}

	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return res
	}

	lowerPartial := strings.ToLower(partial)
	var matches []string
	for _, e := range entries {
		name := e.Name()
		// Skip hidden files unless partial starts with '.'
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(partial, ".") {
			continue
		}
		if strings.HasPrefix(strings.ToLower(name), lowerPartial) {
			isDir := e.IsDir()
			if !isDir && (e.Type()&os.ModeSymlink != 0) {
				if fi, err := os.Stat(filepath.Join(searchDir, name)); err == nil && fi.IsDir() {
					isDir = true
				}
			}
			if isDir {
				matches = append(matches, name+"/")
			} else {
				matches = append(matches, name)
			}
		}
	}

	if len(matches) == 0 {
		return res
	}

	res.Matches = matches
	res.Dir = searchDir
	res.InputPrefix = inputPrefix

	if len(matches) == 1 {
		res.Completed = inputPrefix + matches[0]
		res.IsExactSingle = true
		return res
	}

	// Multiple matches: find longest common prefix
	lcp := longestCommonPrefix(matches)
	res.CommonPrefix = lcp

	if len(lcp) > len(partial) {
		res.Completed = inputPrefix + lcp
	}

	return res
}

func longestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	prefix := strs[0]
	for _, s := range strs[1:] {
		for !strings.HasPrefix(strings.ToLower(s), strings.ToLower(prefix)) {
			if len(prefix) == 0 {
				return ""
			}
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}

func formatCompletions(matches []string, activeIdx int, maxWidth int) string {
	if len(matches) == 0 {
		return ""
	}
	var parts []string
	curLen := 0
	for i, m := range matches {
		var item string
		if i == activeIdx {
			item = theme.Accent.Bold(true).Render("[" + m + "]")
		} else {
			item = theme.Muted.Render(m)
		}
		itemLen := len(m) + 3
		if curLen+itemLen > maxWidth-12 && len(parts) > 0 {
			remaining := len(matches) - i
			parts = append(parts, theme.Muted.Render(fmt.Sprintf("(+%d more)", remaining)))
			break
		}
		parts = append(parts, item)
		curLen += itemLen
	}
	return strings.Join(parts, "  ")
}
