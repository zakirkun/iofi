// Package skills loads markdown skill files and enforces one consistent shape.
package skills

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"iofi/internal/store"
)

// Every skill must carry all of these headings. Enforced at load so an agent
// never receives a half-written procedure.
var RequiredSections = []string{
	"Purpose", "When To Use", "Procedure", "Constraints", "Output Contract", "Examples",
}

type Frontmatter struct {
	Slug          string   `yaml:"slug" json:"slug"`
	Version       int      `yaml:"version" json:"version"`
	AppliesTo     []string `yaml:"applies_to" json:"applies_to"`
	ToolsRequired []string `yaml:"tools_required" json:"tools_required"`
	Priority      int      `yaml:"priority" json:"priority"`
	Description   string   `yaml:"description" json:"description"`
}

type Skill struct {
	Frontmatter Frontmatter
	Body        string
	Sections    map[string]string
	Path        string
}

var (
	fmRe      = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---\r?\n`)
	headingRe = regexp.MustCompile(`(?m)^##\s+(.+?)\s*$`)
)

// Parse splits frontmatter from body and validates the section set.
func Parse(name string, raw []byte) (*Skill, error) {
	text := string(raw)
	m := fmRe.FindStringSubmatch(text)
	if m == nil {
		return nil, fmt.Errorf("%s: missing YAML frontmatter", name)
	}
	var fmData Frontmatter
	if err := yaml.Unmarshal([]byte(m[1]), &fmData); err != nil {
		return nil, fmt.Errorf("%s: frontmatter: %w", name, err)
	}
	if fmData.Slug == "" {
		fmData.Slug = strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	}
	if fmData.Version == 0 {
		fmData.Version = 1
	}
	if fmData.Priority == 0 {
		fmData.Priority = 100
	}

	body := strings.TrimPrefix(text, m[0])
	sections := splitSections(body)

	// A heading with nothing under it passes a presence check and still hands
	// the agent an empty procedure, so emptiness is treated as missing.
	var missing, blank []string
	for _, want := range RequiredSections {
		body, ok := sections[want]
		switch {
		case !ok:
			missing = append(missing, want)
		case strings.TrimSpace(body) == "":
			blank = append(blank, want)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("%s: missing section(s): %s", name, strings.Join(missing, ", "))
	}
	if len(blank) > 0 {
		return nil, fmt.Errorf("%s: empty section(s): %s", name, strings.Join(blank, ", "))
	}
	return &Skill{Frontmatter: fmData, Body: strings.TrimSpace(body), Sections: sections, Path: name}, nil
}

func splitSections(body string) map[string]string {
	out := map[string]string{}
	idx := headingRe.FindAllStringSubmatchIndex(body, -1)
	for i, loc := range idx {
		title := strings.TrimSpace(body[loc[2]:loc[3]])
		end := len(body)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		out[title] = strings.TrimSpace(body[loc[1]:end])
	}
	return out
}

// LoadDir parses every .md under root, upserts it, and wires the role links
// declared in `applies_to`. A malformed file is reported and skipped — one bad
// skill must not stop the rest from loading.
func LoadDir(ctx context.Context, st *store.Store, root string) (int, []error) {
	var loaded int
	var errs []error

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, err)
			return nil
		}
		sk, err := Parse(path, raw)
		if err != nil {
			errs = append(errs, err)
			return nil
		}
		if err := sk.Save(ctx, st); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", sk.Frontmatter.Slug, err))
			return nil
		}
		for _, w := range sk.Validate(ctx, st) {
			errs = append(errs, errors.New(w))
		}
		loaded++
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, fs.ErrNotExist) {
		errs = append(errs, walkErr)
	}
	return loaded, errs
}

// Validate reports mismatches between what the skill assumes and how the roles
// are actually configured. These are warnings, not load failures: the skill is
// still usable, but an agent following a procedure that calls a tool its role
// cannot reach will fail halfway through with no explanation.
func (s *Skill) Validate(ctx context.Context, st *store.Store) []string {
	var warns []string
	for _, roleSlug := range s.Frontmatter.AppliesTo {
		role, err := st.GetRoleBySlug(ctx, roleSlug)
		if err != nil {
			warns = append(warns, fmt.Sprintf(
				"skill %q applies_to role %q, which does not exist", s.Frontmatter.Slug, roleSlug))
			continue
		}
		for _, tool := range s.Frontmatter.ToolsRequired {
			if !role.ToolPolicy.Permits(tool) {
				warns = append(warns, fmt.Sprintf(
					"skill %q needs tool %q but role %q does not allow it",
					s.Frontmatter.Slug, tool, roleSlug))
			}
		}
	}
	return warns
}

// Save upserts the skill and (re)links it to the roles in applies_to.
func (s *Skill) Save(ctx context.Context, st *store.Store) error {
	front := store.JSONMap{
		"slug":           s.Frontmatter.Slug,
		"version":        s.Frontmatter.Version,
		"applies_to":     s.Frontmatter.AppliesTo,
		"tools_required": s.Frontmatter.ToolsRequired,
		"priority":       s.Frontmatter.Priority,
		"description":    s.Frontmatter.Description,
	}
	if _, err := st.UpsertSkill(ctx, store.Skill{
		Slug:        s.Frontmatter.Slug,
		Version:     s.Frontmatter.Version,
		Body:        s.Body,
		Frontmatter: front,
		SourcePath:  s.Path,
	}); err != nil {
		return err
	}
	for _, roleSlug := range s.Frontmatter.AppliesTo {
		if err := st.AttachSkillBySlugs(ctx, roleSlug, s.Frontmatter.Slug, s.Frontmatter.Priority); err != nil {
			return err
		}
	}
	return nil
}
