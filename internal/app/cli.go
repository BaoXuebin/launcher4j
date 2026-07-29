// Package app provides the CLI subcommands for Launcher4j.
package app

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/baoxuebin/launcher4j/internal/config"
	"github.com/baoxuebin/launcher4j/internal/engine"
)

// CLI handles command-line interface commands.
type CLI struct {
	store   *config.ConfigStore
	builder *engine.MavenBuilder
	pm      *engine.ProcessManager
}

// NewCLI creates a new CLI handler.
func NewCLI() *CLI {
	store := config.NewConfigStore()
	settings, _ := store.LoadSettings()
	return &CLI{
		store:   store,
		builder: engine.NewMavenBuilder(settings.MavenPath),
		pm:      engine.NewProcessManager(),
	}
}

// FindProject finds a project by ID, name, or path.
func (c *CLI) FindProject(identifier string) *config.ProjectConfig {
	projects, err := c.store.LoadProjects()
	if err != nil {
		return nil
	}

	for i := range projects {
		if projects[i].ID == identifier {
			return &projects[i]
		}
	}

	for i := range projects {
		if projects[i].Name == identifier {
			return &projects[i]
		}
	}

	norm := strings.ReplaceAll(identifier, "\\", "/")
	for i := range projects {
		p := strings.ReplaceAll(projects[i].Path, "\\", "/")
		if p == norm {
			return &projects[i]
		}
	}

	pom := filepath.Join(identifier, "pom.xml")
	if _, err := os.Stat(pom); err == nil {
		proj := config.DefaultProjectConfig(identifier, filepath.Base(identifier))
		return &proj
	}

	return nil
}

// FindJar locates the executable JAR in a project's target directory.
func FindJar(projectPath string) string {
	mainMod := FindMainModule(projectPath)
	if jar := findJarInDir(mainMod); jar != "" {
		return jar
	}
	return findJarInDir(projectPath)
}

func findJarInDir(path string) string {
	target := filepath.Join(path, "target")
	entries, err := os.ReadDir(target)
	if err != nil {
		return ""
	}

	var jars []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jar") {
			continue
		}
		lower := strings.ToLower(e.Name())
		if strings.Contains(lower, "sources") ||
			strings.Contains(lower, "javadoc") ||
			strings.Contains(lower, "original") {
			continue
		}
		jars = append(jars, filepath.Join(target, e.Name()))
	}

	if len(jars) == 0 {
		return ""
	}

	sort.Slice(jars, func(i, j int) bool {
		fi, _ := os.Stat(jars[i])
		fj, _ := os.Stat(jars[j])
		return fi.ModTime().After(fj.ModTime())
	})

	return jars[0]
}

// FindMainModule finds the main module containing @SpringBootApplication
// in multi-module Maven projects.
func FindMainModule(projectPath string) string {
	pom := filepath.Join(projectPath, "pom.xml")
	if _, err := os.Stat(pom); os.IsNotExist(err) {
		return projectPath
	}

	data, err := os.ReadFile(pom)
	if err != nil {
		return projectPath
	}

	content := string(data)
	if !strings.Contains(content, "<module>") {
		return projectPath
	}

	modules := extractModuleNames(content)
	if len(modules) == 0 {
		return projectPath
	}

	for _, modName := range modules {
		modPath := filepath.Join(projectPath, modName)
		srcDir := filepath.Join(modPath, "src", "main", "java")
		if info, err := os.Stat(srcDir); err == nil && info.IsDir() {
			found := false
			filepath.Walk(srcDir, func(path string, fi os.FileInfo, err error) error {
				if err != nil || fi.IsDir() {
					return nil
				}
				if strings.HasSuffix(path, ".java") {
					if d, err := os.ReadFile(path); err == nil {
						if strings.Contains(string(d), "@SpringBootApplication") {
							found = true
							return filepath.SkipAll
						}
					}
				}
				return nil
			})
			if found {
				return modPath
			}
		}
	}

	return filepath.Join(projectPath, modules[0])
}

func extractModuleNames(content string) []string {
	var modules []string
	idx := 0
	for {
		start := strings.Index(content[idx:], "<module>")
		if start < 0 {
			break
		}
		start += idx + len("<module>")
		end := strings.Index(content[start:], "</module>")
		if end < 0 {
			break
		}
		mod := strings.TrimSpace(content[start : start+end])
		if mod != "" {
			modules = append(modules, mod)
		}
		idx = start + end + len("</module>")
	}
	return modules
}

// ── Command Handlers ─────────────────────────────────

// CmdRun starts a project, auto-building first if needed, and waits for it to exit.
func (c *CLI) CmdRun(projectID string) error {
	project := c.FindProject(projectID)
	if project == nil {
		return fmt.Errorf("project '%s' not found", projectID)
	}

	jar := FindJar(project.Path)
	if jar == "" {
		fmt.Println("  未找到 JAR，自动构建...")
		result, err := c.builder.Build(project.Path, project.ID, func(pid, level, msg string) {
			fmt.Printf("  %s\n", msg)
		})
		if err != nil {
			return fmt.Errorf("auto-build failed: %w", err)
		}
		if !result.Success {
			return fmt.Errorf("auto-build failed, cannot start")
		}
		jar = FindJar(project.Path)
		if jar == "" {
			return fmt.Errorf("build succeeded but no jar found in %s/target/", project.Path)
		}
	}

	fmt.Printf("▶ Starting '%s'...\n", project.Name)

	onLog := func(pid, level, msg string) {
		prefix := map[string]string{
			"error": "ERR", "warn": "WRN",
			"build": "BLD", "info": "INF",
			"debug": "DBG",
		}
		fmt.Printf("  [%s] %s\n", prefix[level], msg)
	}

	_, err := c.pm.Start(
		project.ID, project.Name, jar,
		project.JDKHome, project.VMArgs, project.EnvVars,
		onLog,
		func(pid, status string) {
			fmt.Printf("  [%s] %s\n", status, project.Name)
		},
	)
	if err != nil {
		return fmt.Errorf("failed to start: %w", err)
	}

	proc := c.pm.GetProcess(project.ID)
	if proc != nil && proc.PID > 0 {
		fmt.Printf("✓ Started (PID: %d)\n", proc.PID)
	}

	fmt.Println("  Press Ctrl+C to stop...")
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	c.pm.Stop(project.ID)
	fmt.Println("\n⏹ Stopped")
	return nil
}

// CmdStop stops a project.
func (c *CLI) CmdStop(projectID string) error {
	project := c.FindProject(projectID)
	if project == nil {
		return fmt.Errorf("project '%s' not found", projectID)
	}
	c.pm.Stop(project.ID)
	fmt.Printf("✓ Stopped '%s'\n", project.Name)
	return nil
}

// CmdRestart restarts a project.
func (c *CLI) CmdRestart(projectID string) error {
	project := c.FindProject(projectID)
	if project == nil {
		return fmt.Errorf("project '%s' not found", projectID)
	}

	fmt.Printf("↻ Restarting '%s'...\n", project.Name)
	c.pm.Stop(project.ID)
	time.Sleep(1 * time.Second)

	jar := FindJar(project.Path)
	if jar == "" {
		return fmt.Errorf("no executable jar found")
	}

	onLog := func(pid, level, msg string) {
		prefix := map[string]string{
			"error": "ERR", "warn": "WRN",
			"build": "BLD", "info": "INF",
			"debug": "DBG",
		}
		fmt.Printf("  [%s] %s\n", prefix[level], msg)
	}

	_, err := c.pm.Start(
		project.ID, project.Name, jar,
		project.JDKHome, project.VMArgs, project.EnvVars,
		onLog, nil,
	)
	if err != nil {
		return fmt.Errorf("failed to restart: %w", err)
	}

	proc := c.pm.GetProcess(project.ID)
	if proc != nil && proc.PID > 0 {
		fmt.Printf("✓ Restarted (PID: %d)\n", proc.PID)
	}
	return nil
}

// CmdCompile compiles a project.
func (c *CLI) CmdCompile(identifier string) error {
	path, projectID, err := c.resolvePath(identifier)
	if err != nil {
		return err
	}

	fmt.Printf("▶ Compiling %s...\n", filepath.Base(path))
	result, err := c.builder.Compile(path, projectID, func(pid, level, msg string) {
		fmt.Printf("  %s\n", msg)
	})
	if err != nil {
		return err
	}
	if result.Success {
		fmt.Printf("✅ Compile SUCCESS (%.1fs)\n", float64(result.DurationMs)/1000)
	} else {
		fmt.Printf("❌ Compile FAILED (%.1fs)\n", float64(result.DurationMs)/1000)
		for _, e := range result.Errors {
			if len(e) > 0 {
				fmt.Printf("   %s\n", e)
			}
		}
		return errors.New("compile failed")
	}
	return nil
}

// CmdBuild packages a project.
func (c *CLI) CmdBuild(identifier string) error {
	path, projectID, err := c.resolvePath(identifier)
	if err != nil {
		return err
	}

	fmt.Printf("📦 Building %s...\n", filepath.Base(path))
	result, err := c.builder.Build(path, projectID, func(pid, level, msg string) {
		fmt.Printf("  %s\n", msg)
	})
	if err != nil {
		return err
	}
	if result.Success {
		fmt.Printf("✅ Build SUCCESS (%.1fs)\n", float64(result.DurationMs)/1000)
	} else {
		fmt.Printf("❌ Build FAILED (%.1fs)\n", float64(result.DurationMs)/1000)
		for _, e := range result.Errors {
			if len(e) > 0 {
				fmt.Printf("   %s\n", e)
			}
		}
		return errors.New("build failed")
	}
	return nil
}

// CmdClean cleans a project.
func (c *CLI) CmdClean(identifier string) error {
	path, projectID, err := c.resolvePath(identifier)
	if err != nil {
		return err
	}

	fmt.Printf("🧹 Cleaning %s...\n", filepath.Base(path))
	result, err := c.builder.Clean(path, projectID, func(pid, level, msg string) {
		fmt.Printf("  %s\n", msg)
	})
	if err != nil {
		return err
	}
	if result.Success {
		fmt.Printf("✅ Clean SUCCESS (%.1fs)\n", float64(result.DurationMs)/1000)
	} else {
		fmt.Printf("❌ Clean FAILED (%.1fs)\n", float64(result.DurationMs)/1000)
		for _, e := range result.Errors {
			if len(e) > 0 {
				fmt.Printf("   %s\n", e)
			}
		}
		return errors.New("clean failed")
	}
	return nil
}

// CmdStatus shows project status and full configuration.
func (c *CLI) CmdStatus(identifier string) error {
	if identifier == "" {
		projects, err := c.store.LoadProjects()
		if err != nil {
			return err
		}
		if len(projects) == 0 {
			fmt.Println("No configured projects.")
			return nil
		}
		fmt.Printf("  %-20s %-10s %s\n", "Name", "Status", "Path")
		fmt.Println(strings.Repeat("-", 70))
		for _, p := range projects {
			status := c.pm.Status(p.ID)
			fmt.Printf("  %-20s %-10s %s\n", p.Name, status.String(), p.Path)
		}
	} else {
		project := c.FindProject(identifier)
		if project == nil {
			return fmt.Errorf("project '%s' not found", identifier)
		}
		status := c.pm.Status(project.ID)
		proc := c.pm.GetProcess(project.ID)
		fmt.Printf("Project: %s\n", project.Name)
		fmt.Printf("  Path:         %s\n", project.Path)
		fmt.Printf("  Status:       %s\n", status.String())
		fmt.Printf("  JDK:          %s\n", project.JDKHome)
		fmt.Printf("  Auto-Compile: %t\n", project.AutoCompile)
		if proc != nil && proc.PID > 0 {
			fmt.Printf("  PID:          %d\n", proc.PID)
		}
		if proc != nil && proc.Port > 0 {
			fmt.Printf("  Port:         %d\n", proc.Port)
		}
		if project.VMArgs != "" {
			fmt.Printf("  VM Args:\n")
			for _, arg := range strings.Fields(project.VMArgs) {
				fmt.Printf("    • %s\n", arg)
			}
		}
		if project.EnvVars != "" {
			fmt.Printf("  Env Vars:\n")
			for _, line := range strings.Split(project.EnvVars, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					fmt.Printf("    • %s\n", line)
				}
			}
		}
	}
	return nil
}

// CmdList lists all configured projects.
func (c *CLI) CmdList() error {
	projects, err := c.store.LoadProjects()
	if err != nil {
		return err
	}
	if len(projects) == 0 {
		fmt.Println("No configured projects.")
		return nil
	}

	fmt.Printf("%-20s %-10s %s\n", "Name", "Status", "Path")
	fmt.Println(strings.Repeat("-", 70))
	for _, p := range projects {
		status := c.pm.Status(p.ID)
		fmt.Printf("%-20s %-10s %s\n", p.Name, status.String(), p.Path)
	}
	return nil
}

// CmdAdd adds a new project to configuration.
func (c *CLI) CmdAdd(path, name, jdkHome, vmArgs, envVars string, autoCompile bool) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	pom := filepath.Join(absPath, "pom.xml")
	if _, err := os.Stat(pom); os.IsNotExist(err) {
		return fmt.Errorf("pom.xml not found in %s", absPath)
	}
	if name == "" {
		name = filepath.Base(absPath)
	}
	proj := config.DefaultProjectConfig(absPath, name)
	proj.JDKHome = jdkHome
	proj.VMArgs = vmArgs
	proj.EnvVars = envVars
	proj.AutoCompile = autoCompile
	if err := c.store.SaveProject(proj); err != nil {
		return err
	}
	fmt.Printf("✓ Added project '%s' (%s)\n", proj.Name, proj.Path)
	return nil
}

// CmdConfig updates an existing project's configuration.
// Empty strings mean "don't change". For removal, use the addVM/removeVM/setEnv/unsetEnv params.
func (c *CLI) CmdConfig(
	identifier string,
	name, jdkHome string,
	addVM, removeVM []string,
	setEnv, unsetEnv []string,
	autoCompile *bool,
) error {
	project := c.FindProject(identifier)
	if project == nil {
		return fmt.Errorf("project '%s' not found", identifier)
	}

	changed := false

	if name != "" {
		project.Name = name
		changed = true
	}
	if jdkHome != "" {
		project.JDKHome = jdkHome
		changed = true
	}

	// VM args: add individual items
	if len(addVM) > 0 {
		existing := strings.Fields(project.VMArgs)
		existingSet := make(map[string]bool, len(existing))
		for _, a := range existing {
			existingSet[a] = true
		}
		for _, a := range addVM {
			if !existingSet[a] {
				existing = append(existing, a)
				existingSet[a] = true
			}
		}
		project.VMArgs = strings.Join(existing, " ")
		changed = true
	}

	// VM args: remove individual items (by prefix match)
	if len(removeVM) > 0 {
		existing := strings.Fields(project.VMArgs)
		var kept []string
		for _, a := range existing {
			remove := false
			for _, r := range removeVM {
				if strings.HasPrefix(a, r) {
					remove = true
					break
				}
			}
			if !remove {
				kept = append(kept, a)
			}
		}
		project.VMArgs = strings.Join(kept, " ")
		changed = true
	}

	// Env vars: set individual entries
	if len(setEnv) > 0 {
		envMap := make(map[string]string)
		for _, line := range strings.Split(project.EnvVars, "\n") {
			line = strings.TrimSpace(line)
			if idx := strings.Index(line, "="); idx > 0 {
				key := strings.TrimSpace(line[:idx])
				val := strings.TrimSpace(line[idx+1:])
				envMap[key] = val
			}
		}
		for _, kv := range setEnv {
			if idx := strings.Index(kv, "="); idx > 0 {
				key := strings.TrimSpace(kv[:idx])
				val := strings.TrimSpace(kv[idx+1:])
				envMap[key] = val
			}
		}
		var lines []string
		for k, v := range envMap {
			lines = append(lines, k+"="+v)
		}
		sort.Strings(lines)
		project.EnvVars = strings.Join(lines, "\n")
		changed = true
	}

	// Env vars: unset individual entries
	if len(unsetEnv) > 0 {
		unsetSet := make(map[string]bool)
		for _, k := range unsetEnv {
			unsetSet[strings.TrimSpace(k)] = true
		}
		var kept []string
		for _, line := range strings.Split(project.EnvVars, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if idx := strings.Index(line, "="); idx > 0 {
				key := strings.TrimSpace(line[:idx])
				if !unsetSet[key] {
					kept = append(kept, line)
				}
			} else if !unsetSet[line] {
				kept = append(kept, line)
			}
		}
		project.EnvVars = strings.Join(kept, "\n")
		changed = true
	}

	if autoCompile != nil {
		project.AutoCompile = *autoCompile
		changed = true
	}

	if !changed {
		fmt.Println("No changes specified. Current configuration:")
		return c.CmdStatus(identifier)
	}

	if err := c.store.SaveProject(*project); err != nil {
		return err
	}
	fmt.Printf("✓ Updated project '%s'\n", project.Name)
	return nil
}

// CmdRemove removes a project from configuration.
func (c *CLI) CmdRemove(identifier string) error {
	project := c.FindProject(identifier)
	if project == nil {
		return fmt.Errorf("project '%s' not found", identifier)
	}
	if err := c.store.RemoveProject(project.ID); err != nil {
		return err
	}
	c.pm.Stop(project.ID)
	fmt.Printf("✓ Removed project '%s'\n", project.Name)
	return nil
}

// resolvePath resolves an identifier to a project path and ID.
func (c *CLI) resolvePath(identifier string) (path, projectID string, err error) {
	if identifier == "" {
		wd, _ := os.Getwd()
		pom := filepath.Join(wd, "pom.xml")
		if _, err := os.Stat(pom); os.IsNotExist(err) {
			return "", "", fmt.Errorf("no pom.xml found in current directory")
		}
		return wd, "cwd", nil
	}

	project := c.FindProject(identifier)
	if project != nil {
		return project.Path, project.ID, nil
	}

	pom := filepath.Join(identifier, "pom.xml")
	if _, err := os.Stat(pom); err == nil {
		return identifier, "path", nil
	}

	return "", "", fmt.Errorf("project '%s' not found", identifier)
}
