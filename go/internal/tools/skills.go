package tools

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type SkillSource struct {
	Scope     string `json:"scope"`
	Namespace string `json:"namespace"`
	Path      string `json:"path"`
}

type SkillSummary struct {
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	PreferredPath string        `json:"preferred_path"`
	Sources       []SkillSource `json:"sources"`
}

type ScannedSkillRoot struct {
	Scope     string `json:"scope"`
	Namespace string `json:"namespace"`
	Path      string `json:"path"`
	Exists    bool   `json:"exists"`
}

type ListSkillsResult struct {
	Success       bool               `json:"success"`
	WorkspaceRoot string             `json:"workspace_root"`
	ScannedRoots  []ScannedSkillRoot `json:"scanned_roots"`
	Skills        []SkillSummary     `json:"skills"`
	Filters       map[string]any     `json:"filters"`
}

func ListSkills(workspaceRoot string, namespace string, namePattern string, descriptionMaxLength int, includeProject bool, includeGlobal bool) ListSkillsResult {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		homeDir = "."
	}

	type skillRoot struct {
		scope     string
		namespace string
		path      string
	}

	roots := make([]skillRoot, 0, 5)
	if includeProject {
		roots = append(roots,
			skillRoot{scope: "project", namespace: "agents", path: filepath.Join(workspaceRoot, ".agents", "skills")},
			skillRoot{scope: "project", namespace: "codex", path: filepath.Join(workspaceRoot, ".codex", "skills")},
		)
	}
	if includeGlobal {
		roots = append(roots,
			skillRoot{scope: "global", namespace: "agents", path: filepath.Join(homeDir, ".agents", "skills")},
			skillRoot{scope: "global", namespace: "codex", path: filepath.Join(homeDir, ".codex", "skills")},
			skillRoot{scope: "global", namespace: "claude", path: filepath.Join(homeDir, ".claude", "skills")},
		)
	}

	summaryByName := make(map[string]*SkillSummary)
	scanned := make([]ScannedSkillRoot, 0, len(roots))

	for _, root := range roots {
		if namespace != "" && root.namespace != namespace {
			continue
		}

		info, err := os.Stat(root.path)
		exists := err == nil && info.IsDir()
		scanned = append(scanned, ScannedSkillRoot{
			Scope:     root.scope,
			Namespace: root.namespace,
			Path:      root.path,
			Exists:    exists,
		})
		if !exists {
			continue
		}

		_ = filepath.Walk(root.path, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil || info == nil || info.IsDir() || info.Name() != "SKILL.md" {
				return nil
			}
			name, description := readSkillSummary(path)
			if namePattern != "" && !globMatch(namePattern, name) {
				return nil
			}
			if descriptionMaxLength > 0 && len(description) > descriptionMaxLength {
				description = strings.TrimSpace(description[:descriptionMaxLength]) + "..."
			}
			source := SkillSource{
				Scope:     root.scope,
				Namespace: root.namespace,
				Path:      path,
			}

			if existing, ok := summaryByName[name]; ok {
				existing.Sources = append(existing.Sources, source)
				return nil
			}
			summaryByName[name] = &SkillSummary{
				Name:          name,
				Description:   description,
				PreferredPath: path,
				Sources:       []SkillSource{source},
			}
			return nil
		})
	}

	names := make([]string, 0, len(summaryByName))
	for name := range summaryByName {
		names = append(names, name)
	}
	sort.Strings(names)

	skills := make([]SkillSummary, 0, len(names))
	for _, name := range names {
		skills = append(skills, *summaryByName[name])
	}

	return ListSkillsResult{
		Success:       true,
		WorkspaceRoot: workspaceRoot,
		ScannedRoots:  scanned,
		Skills:        skills,
		Filters: map[string]any{
			"namespace":              emptyToNil(namespace),
			"name_pattern":           emptyToNil(namePattern),
			"description_max_length": maxOrNil(descriptionMaxLength),
			"include_project":        includeProject,
			"include_global":         includeGlobal,
		},
	}
}

func readSkillSummary(path string) (string, string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return filepath.Base(filepath.Dir(path)), ""
	}
	lines := strings.Split(string(raw), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return filepath.Base(filepath.Dir(path)), ""
	}

	name := filepath.Base(filepath.Dir(path))
	description := ""
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "---" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		switch key {
		case "name":
			if value != "" {
				name = value
			}
		case "description":
			description = value
		}
	}
	return name, description
}

func globMatch(pattern string, value string) bool {
	matched, err := filepath.Match(pattern, value)
	return err == nil && matched
}

func emptyToNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func maxOrNil(value int) any {
	if value <= 0 {
		return nil
	}
	return value
}
