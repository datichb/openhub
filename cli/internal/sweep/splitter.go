package sweep

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/datichb/openhub/cli/internal/llm"
	"github.com/datichb/openhub/cli/internal/task"
)

// SplitOpts configures how the splitter decomposes the goal into tasks.
type SplitOpts struct {
	Strategy     Strategy // Decomposition strategy (required)
	Goal         string   // High-level sweep goal (required)
	ManualTasks  []string // Explicit task list (for StrategyManual)
	IncludeGlobs []string // File patterns to include
	ExcludeGlobs []string // File patterns to exclude
	MaxTasks     int      // Hard cap (0 = use SweepConfig.MaxSplits)
	ProjectPath  string   // Project root directory
	BranchPrefix string   // Branch name prefix (default: "sweep/")
	Hints        string   // Extra context for StrategyLLM
}

// Splitter decomposes a high-level sweep goal into concrete tasks.
// It receives an llm.Completer for the LLM strategy; the other strategies
// are purely deterministic and do not require LLM access.
type Splitter struct {
	llm llm.Completer
}

// NewSplitter creates a splitter. Pass nil for completer if you only use
// deterministic strategies (manual, by-file, by-package).
func NewSplitter(completer llm.Completer) *Splitter {
	return &Splitter{llm: completer}
}

// Split decomposes the goal according to opts.Strategy.
func (s *Splitter) Split(ctx context.Context, opts SplitOpts) ([]task.Task, error) {
	if opts.Goal == "" {
		return nil, fmt.Errorf("sweep goal is required")
	}
	if opts.BranchPrefix == "" {
		opts.BranchPrefix = "sweep/"
	}

	var tasks []task.Task
	var err error

	switch opts.Strategy {
	case StrategyManual:
		tasks, err = s.splitManual(opts)
	case StrategyByFile:
		tasks, err = s.splitByFile(opts)
	case StrategyByPackage:
		tasks, err = s.splitByPackage(opts)
	case StrategyLLM:
		tasks, err = s.splitLLM(ctx, opts)
	default:
		return nil, fmt.Errorf("unknown sweep strategy: %q", opts.Strategy)
	}

	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("sweep decomposition produced zero tasks")
	}

	// Enforce MaxTasks cap
	if opts.MaxTasks > 0 && len(tasks) > opts.MaxTasks {
		tasks = tasks[:opts.MaxTasks]
	}

	return tasks, nil
}

// --- Strategy implementations ---

// splitManual parses an explicit task list provided by the user.
func (s *Splitter) splitManual(opts SplitOpts) ([]task.Task, error) {
	if len(opts.ManualTasks) == 0 {
		return nil, fmt.Errorf("--sweep-tasks is required with --sweep-strategy=manual")
	}

	tasks := make([]task.Task, 0, len(opts.ManualTasks))
	for _, item := range opts.ManualTasks {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		id := slugify("sweep-" + item)
		tasks = append(tasks, task.Task{
			ID:          id,
			Kind:        task.KindSweep,
			Label:       item,
			Description: fmt.Sprintf("Sweep sub-task for scope: %s\nGoal: %s", item, opts.Goal),
			BranchName:  opts.BranchPrefix + id,
			Metadata: map[string]string{
				"scope":    item,
				"strategy": string(StrategyManual),
				"goal":     opts.Goal,
			},
		})
	}

	if len(tasks) == 0 {
		return nil, fmt.Errorf("--sweep-tasks produced no valid tasks after parsing")
	}
	return tasks, nil
}

// splitByFile groups files matched by include globs into batches.
func (s *Splitter) splitByFile(opts SplitOpts) ([]task.Task, error) {
	if len(opts.IncludeGlobs) == 0 {
		return nil, fmt.Errorf("--sweep-include is required with --sweep-strategy=by-file")
	}

	// Collect matching files
	var matched []string
	for _, pattern := range opts.IncludeGlobs {
		// Make pattern relative to project path
		fullPattern := filepath.Join(opts.ProjectPath, pattern)
		files, err := filepath.Glob(fullPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		for _, f := range files {
			// Make path relative to project for readability
			rel, err := filepath.Rel(opts.ProjectPath, f)
			if err != nil {
				rel = f
			}
			if !isExcluded(rel, opts.ExcludeGlobs) {
				matched = append(matched, rel)
			}
		}
	}

	if len(matched) == 0 {
		return nil, fmt.Errorf("no files matched include globs %v (exclude: %v)", opts.IncludeGlobs, opts.ExcludeGlobs)
	}

	// Determine batch size
	maxTasks := opts.MaxTasks
	if maxTasks <= 0 {
		maxTasks = 10
	}
	batchSize := (len(matched) + maxTasks - 1) / maxTasks
	if batchSize < 1 {
		batchSize = 1
	}

	var tasks []task.Task
	for i := 0; i < len(matched); i += batchSize {
		end := i + batchSize
		if end > len(matched) {
			end = len(matched)
		}
		batch := matched[i:end]
		batchIdx := len(tasks) + 1
		id := fmt.Sprintf("sweep-batch-%d", batchIdx)
		scope := strings.Join(batch, ",")
		label := fmt.Sprintf("Batch %d (%d files)", batchIdx, len(batch))

		tasks = append(tasks, task.Task{
			ID:          id,
			Kind:        task.KindSweep,
			Label:       label,
			Description: fmt.Sprintf("Sweep batch %d: apply goal to files:\n%s\n\nGoal: %s", batchIdx, strings.Join(batch, "\n"), opts.Goal),
			BranchName:  opts.BranchPrefix + id,
			Metadata: map[string]string{
				"scope":    scope,
				"strategy": string(StrategyByFile),
				"goal":     opts.Goal,
			},
		})
	}

	return tasks, nil
}

// splitByPackage creates one task per Go package (or JS workspace).
func (s *Splitter) splitByPackage(opts SplitOpts) ([]task.Task, error) {
	packages, err := listGoPackages(opts.ProjectPath)
	if err != nil {
		return nil, fmt.Errorf("listing packages: %w", err)
	}

	// Filter by include/exclude globs
	var filtered []string
	for _, pkg := range packages {
		if len(opts.IncludeGlobs) > 0 && !matchesAny(pkg, opts.IncludeGlobs) {
			continue
		}
		if isExcluded(pkg, opts.ExcludeGlobs) {
			continue
		}
		filtered = append(filtered, pkg)
	}

	if len(filtered) == 0 {
		return nil, fmt.Errorf("no packages found after filtering (include: %v, exclude: %v)", opts.IncludeGlobs, opts.ExcludeGlobs)
	}

	tasks := make([]task.Task, 0, len(filtered))
	for _, pkg := range filtered {
		// Extract a short name from the package path
		shortName := pkg
		if idx := strings.LastIndex(pkg, "/"); idx >= 0 {
			shortName = pkg[idx+1:]
		}
		// Remove leading ./ if present
		shortName = strings.TrimPrefix(shortName, "./")

		id := slugify("sweep-" + shortName)
		tasks = append(tasks, task.Task{
			ID:          id,
			Kind:        task.KindSweep,
			Label:       shortName,
			Description: fmt.Sprintf("Sweep package %s\nGoal: %s", pkg, opts.Goal),
			BranchName:  opts.BranchPrefix + id,
			Metadata: map[string]string{
				"scope":    pkg,
				"strategy": string(StrategyByPackage),
				"goal":     opts.Goal,
			},
		})
	}

	return tasks, nil
}

// splitLLM delegates decomposition to an LLM planner.
func (s *Splitter) splitLLM(ctx context.Context, opts SplitOpts) ([]task.Task, error) {
	if s.llm == nil {
		return nil, fmt.Errorf("LLM completer is required for --sweep-strategy=llm")
	}

	maxTasks := opts.MaxTasks
	if maxTasks <= 0 {
		maxTasks = 10
	}

	prompt := buildPlannerPrompt(opts.Goal, opts.ProjectPath, maxTasks, opts.Hints)

	const maxRetries = 2
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		currentPrompt := prompt
		if attempt > 0 && lastErr != nil {
			currentPrompt = fmt.Sprintf(
				"%s\n\n[RETRY %d/%d] La réponse précédente n'était pas valide : %s\nRéponds UNIQUEMENT avec du JSON valide.",
				prompt, attempt, maxRetries, lastErr.Error(),
			)
		}

		resp, err := s.llm.Complete(ctx, llm.Request{
			Prompt:      currentPrompt,
			Format:      "json",
			ProjectPath: opts.ProjectPath,
		})
		if err != nil {
			lastErr = err
			continue
		}

		tasks, err := parseLLMResponse(resp.Content, opts)
		if err != nil {
			lastErr = err
			continue
		}

		return tasks, nil
	}

	return nil, fmt.Errorf("LLM decomposition failed after %d attempts: %w", maxRetries+1, lastErr)
}

// --- Helpers ---

// llmTaskJSON is the JSON shape returned by the LLM planner.
type llmTaskJSON struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Scope       string `json:"scope"`
	Description string `json:"description"`
}

type llmPlannerResponse struct {
	Tasks []llmTaskJSON `json:"tasks"`
}

// parseLLMResponse extracts tasks from the LLM's JSON response.
func parseLLMResponse(content string, opts SplitOpts) ([]task.Task, error) {
	// Try to extract JSON from potential markdown code fences
	content = extractJSON(content)

	var resp llmPlannerResponse
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		return nil, fmt.Errorf("invalid JSON from LLM: %w", err)
	}

	if len(resp.Tasks) == 0 {
		return nil, fmt.Errorf("LLM returned zero tasks")
	}

	// Validate and convert
	seen := make(map[string]bool)
	tasks := make([]task.Task, 0, len(resp.Tasks))
	for _, lt := range resp.Tasks {
		if lt.ID == "" {
			return nil, fmt.Errorf("LLM task missing 'id' field")
		}
		id := slugify(lt.ID)
		if seen[id] {
			return nil, fmt.Errorf("duplicate task ID from LLM: %q", id)
		}
		seen[id] = true

		label := lt.Label
		if label == "" {
			label = id
		}

		tasks = append(tasks, task.Task{
			ID:          id,
			Kind:        task.KindSweep,
			Label:       label,
			Description: lt.Description,
			BranchName:  opts.BranchPrefix + id,
			Metadata: map[string]string{
				"scope":    lt.Scope,
				"strategy": string(StrategyLLM),
				"goal":     opts.Goal,
			},
		})
	}

	return tasks, nil
}

// extractJSON strips markdown code fences if present.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	// Strip ```json ... ``` fences
	if strings.HasPrefix(s, "```") {
		lines := strings.SplitN(s, "\n", 2)
		if len(lines) == 2 {
			s = lines[1]
		}
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	}
	return strings.TrimSpace(s)
}

// listGoPackages runs `go list ./...` and returns relative package paths.
func listGoPackages(projectPath string) ([]string, error) {
	cmd := exec.Command("go", "list", "./...")
	cmd.Dir = projectPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list failed: %w", err)
	}

	var packages []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			packages = append(packages, line)
		}
	}
	return packages, nil
}

// slugify converts a string to a safe identifier (lowercase, alphanumeric + hyphens).
var slugRe = regexp.MustCompile(`[^a-z0-9-]+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	s = slugRe.ReplaceAllString(s, "")
	// Collapse multiple hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	return s
}

// isExcluded returns true if path matches any of the exclude globs.
func isExcluded(path string, excludeGlobs []string) bool {
	for _, pattern := range excludeGlobs {
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
		// Also try matching against the base name
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
	}
	return false
}

// matchesAny returns true if s matches at least one glob pattern.
func matchesAny(s string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, s); matched {
			return true
		}
		// Also try matching against the last path component
		if matched, _ := filepath.Match(pattern, filepath.Base(s)); matched {
			return true
		}
	}
	return false
}
