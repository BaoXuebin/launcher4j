package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/baoxuebin/launcher4j/internal/app"
	"github.com/baoxuebin/launcher4j/internal/config"
	"github.com/baoxuebin/launcher4j/internal/engine"
)

// Tool definition for MCP tools/list response
type toolDef struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	InputSchema inputSchema `json:"inputSchema"`
}

type inputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]property `json:"properties"`
	Required   []string           `json:"required,omitempty"`
}

type property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type callResult struct {
	Content []contentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Handler processes MCP tool calls and manages application state.
type Handler struct {
	store   *config.ConfigStore
	builder *engine.MavenBuilder
	pm      *engine.ProcessManager
}

// NewHandler creates a new tool handler.
func NewHandler() *Handler {
	store := config.NewConfigStore()
	settings, _ := store.LoadSettings()
	return &Handler{
		store:   store,
		builder: engine.NewMavenBuilder(settings.MavenPath),
		pm:      engine.NewProcessManager(),
	}
}

// listTools returns the list of all available tools.
func (h *Handler) listTools() (any, *rpcError) {
	tools := []toolDef{
		{
			Name:        "run",
			Description: "Start a configured Java/Spring Boot project. Auto-builds if no JAR found. The project must have been previously added with the 'add' tool.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"project": {Type: "string", Description: "Project name, ID, or path"},
				},
				Required: []string{"project"},
			},
		},
		{
			Name:        "stop",
			Description: "Stop a running Java project by name, ID, or path.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"project": {Type: "string", Description: "Project name, ID, or path"},
				},
				Required: []string{"project"},
			},
		},
		{
			Name:        "restart",
			Description: "Restart a running Java project (stops it first, then starts).",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"project": {Type: "string", Description: "Project name, ID, or path"},
				},
				Required: []string{"project"},
			},
		},
		{
			Name:        "compile",
			Description: "Run 'mvn compile' on a Maven project. Can target a configured project or the current directory.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"project": {Type: "string", Description: "Project name, ID, or path (optional, defaults to current directory)"},
				},
			},
		},
		{
			Name:        "build",
			Description: "Run 'mvn package -DskipTests' to build a JAR. Can target a configured project or the current directory.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"project": {Type: "string", Description: "Project name, ID, or path (optional, defaults to current directory)"},
				},
			},
		},
		{
			Name:        "clean",
			Description: "Run 'mvn clean' to clean build artifacts.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"project": {Type: "string", Description: "Project name, ID, or path (optional, defaults to current directory)"},
				},
			},
		},
		{
			Name:        "status",
			Description: "Show detailed status and configuration of a project. If no project specified, shows all projects.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"project": {Type: "string", Description: "Project name, ID, or path (optional)"},
				},
			},
		},
		{
			Name:        "list",
			Description: "List all configured projects with their status.",
			InputSchema: inputSchema{
				Type:       "object",
				Properties: map[string]property{},
			},
		},
		{
			Name:        "add",
			Description: "Add a new Maven/Spring Boot project to the launcher configuration.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"path":       {Type: "string", Description: "Absolute path to the project root (must contain pom.xml)"},
					"name":       {Type: "string", Description: "Display name (defaults to directory name)"},
					"jdk_home":   {Type: "string", Description: "JDK installation path (default: 'java' from PATH)"},
					"vm_args":    {Type: "string", Description: "JVM arguments, e.g. '-Xmx512m -Dspring.profiles.active=dev'"},
					"env_vars":   {Type: "string", Description: "Environment variables, one KEY=VALUE per line"},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "remove",
			Description: "Remove a project from the launcher configuration.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"project": {Type: "string", Description: "Project name, ID, or path to remove"},
				},
				Required: []string{"project"},
			},
		},
		{
			Name:        "config_vm",
			Description: "Add or remove individual JVM arguments for a project. Use 'add' to append args and 'remove' to delete args by prefix match.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"project":    {Type: "string", Description: "Project name, ID, or path"},
					"add_vm":     {Type: "array", Description: "JVM args to add, e.g. ['-Xmx1g', '-Dserver.port=8080']"},
					"remove_vm":  {Type: "array", Description: "Prefixes of JVM args to remove, e.g. ['-Xmx', '-Dspring.profiles']"},
				},
				Required: []string{"project"},
			},
		},
		{
			Name:        "config_env",
			Description: "Set or unset environment variables for a project.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"project":   {Type: "string", Description: "Project name, ID, or path"},
					"set_env":   {Type: "array", Description: "Env vars to set, e.g. ['SPRING_PROFILES_ACTIVE=dev', 'JAVA_HOME=/path/to/jdk']"},
					"unset_env": {Type: "array", Description: "Env var keys to remove, e.g. ['SPRING_PROFILES_ACTIVE']"},
				},
				Required: []string{"project"},
			},
		},
		{
			Name:        "config",
			Description: "View or update project configuration settings (name, JDK path, auto-compile). Pass no options to view current config.",
			InputSchema: inputSchema{
				Type: "object",
				Properties: map[string]property{
					"project":      {Type: "string", Description: "Project name, ID, or path"},
					"name":         {Type: "string", Description: "New display name for the project"},
					"jdk_home":     {Type: "string", Description: "JDK installation path"},
					"auto_compile": {Type: "boolean", Description: "Enable/disable auto-compile on file change"},
				},
				Required: []string{"project"},
			},
		},
	}

	return map[string]any{"tools": tools}, nil
}

// callTool executes a tool by name with the given arguments.
func (h *Handler) callTool(params json.RawMessage) (any, *rpcError) {
	var p callParams
	if err := json.Unmarshal(params, &p); err != nil {
		return callResult{
			Content: []contentItem{{Type: "text", Text: fmt.Sprintf("Invalid parameters: %v", err)}},
			IsError: true,
		}, nil
	}

	args, err := parseArgs(p.Arguments)
	if err != nil {
		return callResult{
			Content: []contentItem{{Type: "text", Text: fmt.Sprintf("Invalid arguments: %v", err)}},
			IsError: true,
		}, nil
	}

	var result string
	switch p.Name {
	case "run":
		result = h.toolRun(args)
	case "stop":
		result = h.toolStop(args)
	case "restart":
		result = h.toolRestart(args)
	case "compile":
		result = h.toolCompile(args)
	case "build":
		result = h.toolBuild(args)
	case "clean":
		result = h.toolClean(args)
	case "status":
		result = h.toolStatus(args)
	case "list":
		result = h.toolList()
	case "add":
		result = h.toolAdd(args)
	case "remove":
		result = h.toolRemove(args)
	case "config_vm":
		result = h.toolConfigVM(args)
	case "config_env":
		result = h.toolConfigEnv(args)
	case "config":
		result = h.toolConfig(args)
	default:
		result = fmt.Sprintf("Unknown tool: %s", p.Name)
	}

	return callResult{
		Content: []contentItem{{Type: "text", Text: result}},
	}, nil
}

func parseArgs(raw json.RawMessage) (map[string]any, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args == nil {
		args = make(map[string]any)
	}
	return args, nil
}

func getString(args map[string]any, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getStringSlice(args map[string]any, key string) []string {
	if v, ok := args[key]; ok {
		if arr, ok := v.([]interface{}); ok {
			result := make([]string, 0, len(arr))
			for _, item := range arr {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		}
	}
	return nil
}

func getBool(args map[string]any, key string) *bool {
	if v, ok := args[key]; ok {
		if b, ok := v.(bool); ok {
			return &b
		}
	}
	return nil
}

// ── Tool implementations ────────────────────────────

func (h *Handler) toolRun(args map[string]any) string {
	proj := h.findProject(getString(args, "project"))
	if proj == nil {
		return fmt.Sprintf("❌ Project '%s' not found", getString(args, "project"))
	}
	if h.pm.Status(proj.ID) == engine.StatusRunning {
		return fmt.Sprintf("⚠️ Project '%s' is already running", proj.Name)
	}

	// Auto-build if no JAR
	jar := app.FindJar(proj.Path)
	if jar == "" {
		result, err := h.builder.Build(proj.Path, proj.ID, nil)
		if err != nil {
			return fmt.Sprintf("❌ Auto-build failed: %v", err)
		}
		if !result.Success {
			return fmt.Sprintf("❌ Build failed:\n%s", strings.Join(result.Errors, "\n"))
		}
		jar = app.FindJar(proj.Path)
		if jar == "" {
			return "❌ Build succeeded but no JAR found"
		}
	}

	// Start
	proc, err := h.pm.Start(proj.ID, proj.Name, jar, proj.JDKHome, proj.VMArgs, proj.EnvVars, nil, nil)
	if err != nil {
		return fmt.Sprintf("❌ Start failed: %v", err)
	}
	return fmt.Sprintf("✓ Started '%s' (PID: %d, jar: %s)", proj.Name, proc.PID, filepath.Base(jar))
}

func (h *Handler) toolStop(args map[string]any) string {
	proj := h.findProject(getString(args, "project"))
	if proj == nil {
		return fmt.Sprintf("❌ Project '%s' not found", getString(args, "project"))
	}
	if h.pm.Stop(proj.ID) {
		return fmt.Sprintf("✓ Stopped '%s'", proj.Name)
	}
	return fmt.Sprintf("⚠️ Project '%s' was not running", proj.Name)
}

func (h *Handler) toolRestart(args map[string]any) string {
	proj := h.findProject(getString(args, "project"))
	if proj == nil {
		return fmt.Sprintf("❌ Project '%s' not found", getString(args, "project"))
	}

	h.pm.Stop(proj.ID)
	time.Sleep(500 * time.Millisecond)

	jar := app.FindJar(proj.Path)
	if jar == "" {
		return "❌ No JAR found, please build first"
	}

	proc, err := h.pm.Start(proj.ID, proj.Name, jar, proj.JDKHome, proj.VMArgs, proj.EnvVars, nil, nil)
	if err != nil {
		return fmt.Sprintf("❌ Restart failed: %v", err)
	}
	return fmt.Sprintf("✓ Restarted '%s' (PID: %d)", proj.Name, proc.PID)
}

func (h *Handler) toolCompile(args map[string]any) string {
	path, pid, err := h.resolvePath(getString(args, "project"))
	if err != nil {
		return fmt.Sprintf("❌ %v", err)
	}
	result, err := h.builder.Compile(path, pid, nil)
	if err != nil {
		return fmt.Sprintf("❌ Compile error: %v", err)
	}
	if result.Success {
		return fmt.Sprintf("✅ Compile SUCCESS (%.1fs) - %s", float64(result.DurationMs)/1000, filepath.Base(path))
	}
	return fmt.Sprintf("❌ Compile FAILED:\n%s", strings.Join(result.Errors, "\n"))
}

func (h *Handler) toolBuild(args map[string]any) string {
	path, pid, err := h.resolvePath(getString(args, "project"))
	if err != nil {
		return fmt.Sprintf("❌ %v", err)
	}
	result, err := h.builder.Build(path, pid, nil)
	if err != nil {
		return fmt.Sprintf("❌ Build error: %v", err)
	}
	if result.Success {
		return fmt.Sprintf("✅ Build SUCCESS (%.1fs) - %s", float64(result.DurationMs)/1000, filepath.Base(path))
	}
	return fmt.Sprintf("❌ Build FAILED:\n%s", strings.Join(result.Errors, "\n"))
}

func (h *Handler) toolClean(args map[string]any) string {
	path, pid, err := h.resolvePath(getString(args, "project"))
	if err != nil {
		return fmt.Sprintf("❌ %v", err)
	}
	result, err := h.builder.Clean(path, pid, nil)
	if err != nil {
		return fmt.Sprintf("❌ Clean error: %v", err)
	}
	if result.Success {
		return fmt.Sprintf("✅ Clean SUCCESS (%.1fs) - %s", float64(result.DurationMs)/1000, filepath.Base(path))
	}
	return fmt.Sprintf("❌ Clean FAILED:\n%s", strings.Join(result.Errors, "\n"))
}

func (h *Handler) toolStatus(args map[string]any) string {
	identifier := getString(args, "project")
	if identifier == "" {
		projects, err := h.store.LoadProjects()
		if err != nil {
			return fmt.Sprintf("❌ Load error: %v", err)
		}
		if len(projects) == 0 {
			return "No configured projects."
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%-20s %-10s %s\n", "Name", "Status", "Path"))
		sb.WriteString(strings.Repeat("-", 60) + "\n")
		for _, p := range projects {
			sb.WriteString(fmt.Sprintf("%-20s %-10s %s\n", p.Name, h.pm.Status(p.ID).String(), p.Path))
		}
		return sb.String()
	}

	proj := h.findProject(identifier)
	if proj == nil {
		return fmt.Sprintf("❌ Project '%s' not found", identifier)
	}
	status := h.pm.Status(proj.ID)
	pinfo := h.pm.GetProcess(proj.ID)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Project: %s\n", proj.Name))
	sb.WriteString(fmt.Sprintf("  Path:         %s\n", proj.Path))
	sb.WriteString(fmt.Sprintf("  Status:       %s\n", status.String()))
	sb.WriteString(fmt.Sprintf("  JDK:          %s\n", proj.JDKHome))
	sb.WriteString(fmt.Sprintf("  Auto-Compile: %v\n", proj.AutoCompile))
	if pinfo != nil && pinfo.PID > 0 {
		sb.WriteString(fmt.Sprintf("  PID:          %d\n", pinfo.PID))
	}
	if pinfo != nil && pinfo.Port > 0 {
		sb.WriteString(fmt.Sprintf("  Port:         %d\n", pinfo.Port))
	}
	if proj.VMArgs != "" {
		sb.WriteString("  VM Args:\n")
		for _, a := range strings.Fields(proj.VMArgs) {
			sb.WriteString(fmt.Sprintf("    %s\n", a))
		}
	}
	if proj.EnvVars != "" {
		sb.WriteString("  Env Vars:\n")
		for _, line := range strings.Split(proj.EnvVars, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				sb.WriteString(fmt.Sprintf("    %s\n", line))
			}
		}
	}
	return sb.String()
}

func (h *Handler) toolList() string {
	projects, err := h.store.LoadProjects()
	if err != nil {
		return fmt.Sprintf("❌ %v", err)
	}
	if len(projects) == 0 {
		return "No configured projects."
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-20s %-10s %s\n", "Name", "Status", "Path"))
	sb.WriteString(strings.Repeat("-", 60) + "\n")
	for _, p := range projects {
		sb.WriteString(fmt.Sprintf("%-20s %-10s %s\n", p.Name, h.pm.Status(p.ID).String(), p.Path))
	}
	return sb.String()
}

func (h *Handler) toolAdd(args map[string]any) string {
	path := getString(args, "path")
	name := getString(args, "name")
	jdkHome := getString(args, "jdk_home")
	vmArgs := getString(args, "vm_args")
	envVars := getString(args, "env_vars")

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Sprintf("❌ Invalid path: %v", err)
	}
	pom := filepath.Join(absPath, "pom.xml")
	if _, err := os.Stat(pom); err != nil {
		return fmt.Sprintf("❌ No pom.xml found in %s", absPath)
	}
	if name == "" {
		name = filepath.Base(absPath)
	}
	if jdkHome == "" {
		jdkHome = "java"
	}

	proj := config.DefaultProjectConfig(absPath, name)
	proj.JDKHome = jdkHome
	proj.VMArgs = vmArgs
	proj.EnvVars = envVars
	if err := h.store.SaveProject(proj); err != nil {
		return fmt.Sprintf("❌ Save error: %v", err)
	}
	return fmt.Sprintf("✓ Added project '%s' (%s)", proj.Name, proj.Path)
}

func (h *Handler) toolRemove(args map[string]any) string {
	proj := h.findProject(getString(args, "project"))
	if proj == nil {
		return fmt.Sprintf("❌ Project '%s' not found", getString(args, "project"))
	}
	h.pm.Stop(proj.ID)
	if err := h.store.RemoveProject(proj.ID); err != nil {
		return fmt.Sprintf("❌ Remove error: %v", err)
	}
	return fmt.Sprintf("✓ Removed project '%s'", proj.Name)
}

func (h *Handler) toolConfigVM(args map[string]any) string {
	proj := h.findProject(getString(args, "project"))
	if proj == nil {
		return fmt.Sprintf("❌ Project '%s' not found", getString(args, "project"))
	}

	addVM := getStringSlice(args, "add_vm")
	removeVM := getStringSlice(args, "remove_vm")

	if len(addVM) > 0 {
		existing := strings.Fields(proj.VMArgs)
		existingSet := make(map[string]bool)
		for _, a := range existing {
			existingSet[a] = true
		}
		for _, a := range addVM {
			if !existingSet[a] {
				existing = append(existing, a)
				existingSet[a] = true
			}
		}
		proj.VMArgs = strings.Join(existing, " ")
	}
	if len(removeVM) > 0 {
		existing := strings.Fields(proj.VMArgs)
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
		proj.VMArgs = strings.Join(kept, " ")
	}

	if err := h.store.SaveProject(*proj); err != nil {
		return fmt.Sprintf("❌ Save error: %v", err)
	}
	return fmt.Sprintf("✓ Updated VM args for '%s'\n  %s", proj.Name, proj.VMArgs)
}

func (h *Handler) toolConfigEnv(args map[string]any) string {
	proj := h.findProject(getString(args, "project"))
	if proj == nil {
		return fmt.Sprintf("❌ Project '%s' not found", getString(args, "project"))
	}

	setEnv := getStringSlice(args, "set_env")
	unsetEnv := getStringSlice(args, "unset_env")

	envMap := make(map[string]string)
	for _, line := range strings.Split(proj.EnvVars, "\n") {
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, "="); idx > 0 {
			envMap[strings.TrimSpace(line[:idx])] = strings.TrimSpace(line[idx+1:])
		}
	}
	for _, kv := range setEnv {
		if idx := strings.Index(kv, "="); idx > 0 {
			envMap[strings.TrimSpace(kv[:idx])] = strings.TrimSpace(kv[idx+1:])
		}
	}
	for _, key := range unsetEnv {
		delete(envMap, strings.TrimSpace(key))
	}

	var lines []string
	for k, v := range envMap {
		lines = append(lines, k+"="+v)
	}
	proj.EnvVars = strings.Join(lines, "\n")

	if err := h.store.SaveProject(*proj); err != nil {
		return fmt.Sprintf("❌ Save error: %v", err)
	}
	return fmt.Sprintf("✓ Updated env vars for '%s'\n  %s", proj.Name, strings.ReplaceAll(proj.EnvVars, "\n", "\n  "))
}

func (h *Handler) toolConfig(args map[string]any) string {
	proj := h.findProject(getString(args, "project"))
	if proj == nil {
		return fmt.Sprintf("❌ Project '%s' not found", getString(args, "project"))
	}

	changed := false
	if name := getString(args, "name"); name != "" {
		proj.Name = name
		changed = true
	}
	if jdk := getString(args, "jdk_home"); jdk != "" {
		proj.JDKHome = jdk
		changed = true
	}
	if ac := getBool(args, "auto_compile"); ac != nil {
		proj.AutoCompile = *ac
		changed = true
	}

	if !changed {
		// View mode: return current status
		return h.toolStatus(map[string]any{"project": proj.ID})
	}

	if err := h.store.SaveProject(*proj); err != nil {
		return fmt.Sprintf("❌ Save error: %v", err)
	}
	return fmt.Sprintf("✓ Updated project '%s'", proj.Name)
}

// ── Helpers ─────────────────────────────────────────

func (h *Handler) findProject(identifier string) *config.ProjectConfig {
	// Use the same logic as CLI
	projects, _ := h.store.LoadProjects()
	for i := range projects {
		if projects[i].ID == identifier || projects[i].Name == identifier {
			return &projects[i]
		}
	}
	norm := strings.ReplaceAll(identifier, "\\", "/")
	for i := range projects {
		if strings.ReplaceAll(projects[i].Path, "\\", "/") == norm {
			return &projects[i]
		}
	}
	pom := filepath.Join(identifier, "pom.xml")
	if _, err := os.Stat(pom); err == nil {
		p := config.DefaultProjectConfig(identifier, filepath.Base(identifier))
		return &p
	}
	return nil
}

func (h *Handler) resolvePath(identifier string) (path, pid string, err error) {
	if identifier == "" {
		wd, _ := os.Getwd()
		if _, err := os.Stat(filepath.Join(wd, "pom.xml")); err != nil {
			return "", "", fmt.Errorf("no pom.xml in current directory")
		}
		return wd, "cwd", nil
	}
	proj := h.findProject(identifier)
	if proj != nil {
		return proj.Path, proj.ID, nil
	}
	if _, err := os.Stat(filepath.Join(identifier, "pom.xml")); err == nil {
		return identifier, "path", nil
	}
	return "", "", fmt.Errorf("project '%s' not found", identifier)
}
