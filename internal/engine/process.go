package engine

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// ProcessStatus represents the lifecycle status of a managed Java process.
type ProcessStatus int

const (
	StatusStopped  ProcessStatus = iota
	StatusStarting
	StatusRunning
	StatusStopping
	StatusError
)

func (s ProcessStatus) String() string {
	switch s {
	case StatusStopped:
		return "stopped"
	case StatusStarting:
		return "starting"
	case StatusRunning:
		return "running"
	case StatusStopping:
		return "stopping"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// ParseStatus parses a status string into a ProcessStatus.
func ParseStatus(s string) (ProcessStatus, error) {
	switch s {
	case "stopped":
		return StatusStopped, nil
	case "starting":
		return StatusStarting, nil
	case "running":
		return StatusRunning, nil
	case "stopping":
		return StatusStopping, nil
	case "error":
		return StatusError, nil
	default:
		return StatusStopped, fmt.Errorf("unknown status: %s", s)
	}
}

// ManagedProcess holds state for a single Java process.
type ManagedProcess struct {
	ProjectID string
	Name      string
	Cmd       *exec.Cmd
	PID       int
	Port      int
	status    ProcessStatus
	mu        sync.RWMutex
}

// NewManagedProcess creates a new ManagedProcess.
func NewManagedProcess(projectID, name string) *ManagedProcess {
	return &ManagedProcess{
		ProjectID: projectID,
		Name:      name,
		status:    StatusStopped,
	}
}

// SetStatus safely updates the process status.
func (p *ManagedProcess) SetStatus(s ProcessStatus) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status = s
}

// GetStatus safely returns the process status.
func (p *ManagedProcess) GetStatus() ProcessStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

// LogCallback is called when a log line is received from a managed process.
type LogCallback func(projectID, level, message string)

// StatusCallback is called when the process status changes.
type StatusCallback func(projectID, status string)

// ProcessManager manages multiple Java processes across projects.
type ProcessManager struct {
	mu        sync.Mutex
	processes map[string]*ManagedProcess
}

// NewProcessManager creates a new ProcessManager.
func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		processes: make(map[string]*ManagedProcess),
	}
}

// Start launches a Java JAR process for the given project.
func (pm *ProcessManager) Start(
	projectID, name, jarPath, jdkHome, vmArgs, envVars string,
	onLog LogCallback,
	onStatus StatusCallback,
) (*ManagedProcess, error) {
	pm.mu.Lock()
	if existing, ok := pm.processes[projectID]; ok {
		if existing.IsRunning() {
			pm.mu.Unlock()
			return nil, fmt.Errorf("project '%s' is already running", name)
		}
	}
	pm.mu.Unlock()

	// Build java command
	javaCmd := resolveJavaCmd(jdkHome)
	args := []string{javaCmd}
	if vmArgs != "" {
		args = append(args, strings.Fields(vmArgs)...)
	}
	args = append(args, "-jar", jarPath)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = filepathFromJar(jarPath)

	// Set up environment
	cmd.Env = os.Environ()
	if envVars != "" {
		for _, line := range strings.Split(envVars, "\n") {
			line = strings.TrimSpace(line)
			if idx := strings.Index(line, "="); idx > 0 {
				key := strings.TrimSpace(line[:idx])
				val := strings.TrimSpace(line[idx+1:])
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, val))
			}
		}
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf(
			"Java command '%s' not found. Install JDK or configure the path.", javaCmd)
	}

	proc := NewManagedProcess(projectID, name)
	proc.Cmd = cmd
	proc.PID = cmd.Process.Pid
	proc.SetStatus(StatusStarting)

	pm.mu.Lock()
	pm.processes[projectID] = proc
	pm.mu.Unlock()

	if onStatus != nil {
		onStatus(projectID, "starting")
	}

	// Read stdout
	go readOutput(projectID, stdout, onLog, proc, onStatus, false)
	// Read stderr
	go readOutput(projectID, stderr, onLog, proc, onStatus, true)

	// Fallback: mark as running after 2 seconds if not already detected
	go func() {
		time.Sleep(2 * time.Second)
		if proc.GetStatus() == StatusStarting {
			proc.SetStatus(StatusRunning)
			if onStatus != nil {
				onStatus(projectID, "running")
			}
		}
	}()

	return proc, nil
}

// Stop terminates a Java process.
func (pm *ProcessManager) Stop(projectID string) bool {
	pm.mu.Lock()
	proc, ok := pm.processes[projectID]
	if !ok || proc.Cmd == nil || proc.Cmd.Process == nil {
		pm.mu.Unlock()
		return false
	}
	proc.SetStatus(StatusStopping)
	pm.mu.Unlock()

	pid := proc.PID

	if runtime.GOOS == "windows" {
		kill := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid))
		kill.Run()
	} else {
		proc.Cmd.Process.Signal(os.Interrupt)
		done := make(chan struct{}, 1)
		go func() {
			proc.Cmd.Wait()
			done <- struct{}{}
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			proc.Cmd.Process.Kill()
		}
	}

	proc.SetStatus(StatusStopped)
	proc.PID = 0
	proc.Port = 0
	return true
}

// Status returns the current process status.
func (pm *ProcessManager) Status(projectID string) ProcessStatus {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	proc, ok := pm.processes[projectID]
	if !ok {
		return StatusStopped
	}
	// Check if process exited unexpectedly
	if proc.Cmd != nil && proc.Cmd.Process != nil &&
		proc.Cmd.Process.Signal(os.Signal(nil)) != nil &&
		(proc.GetStatus() == StatusRunning || proc.GetStatus() == StatusStarting) {
		proc.SetStatus(StatusStopped)
		proc.PID = 0
		proc.Port = 0
	}
	return proc.GetStatus()
}

// GetProcess returns the managed process for a project.
func (pm *ProcessManager) GetProcess(projectID string) *ManagedProcess {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.processes[projectID]
}

// GetAllProcesses returns all managed processes.
func (pm *ProcessManager) GetAllProcesses() map[string]*ManagedProcess {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	result := make(map[string]*ManagedProcess, len(pm.processes))
	for k, v := range pm.processes {
		result[k] = v
	}
	return result
}

// ShutdownAll terminates all running processes.
func (pm *ProcessManager) ShutdownAll() {
	pm.mu.Lock()
	ids := make([]string, 0, len(pm.processes))
	for id := range pm.processes {
		ids = append(ids, id)
	}
	pm.mu.Unlock()

	for _, id := range ids {
		pm.Stop(id)
	}
}

// readOutput reads lines from a process output pipe.
func readOutput(
	projectID string,
	rc interface {
		Read([]byte) (int, error)
		Close() error
	},
	onLog LogCallback,
	proc *ManagedProcess,
	onStatus StatusCallback,
	isStderr bool,
) {
	ansiRE := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	scanner := bufio.NewScanner(rc)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		// Decode raw bytes: try UTF-8 first, fall back to GBK
		raw := scanner.Bytes()
		line := decodeToUTF8(raw)

		// Strip ANSI codes
		plain := ansiRE.ReplaceAllString(line, "")

		// Determine level
		level := "info"
		if isStderr {
			level = "error"
		} else {
			level = detectLogLevel(plain)
		}

		if onLog != nil {
			onLog(projectID, level, plain)
		}

		// Detect port
		lower := strings.ToLower(plain)
		if strings.Contains(lower, "port") {
			re := regexp.MustCompile(`port\D*(\d+)`)
			if m := re.FindStringSubmatch(plain); len(m) > 1 {
				fmt.Sscanf(m[1], "%d", &proc.Port)
			}
		}

		// Detect Spring Boot startup
		if strings.Contains(plain, "Started") && strings.Contains(plain, "seconds") {
			proc.SetStatus(StatusRunning)
			if onStatus != nil {
				onStatus(projectID, "running")
			}
		}
	}

	// Process exited
	proc.SetStatus(StatusStopped)
	proc.PID = 0
	proc.Port = 0
	if onStatus != nil {
		onStatus(projectID, "stopped")
	}
}

// decodeToUTF8 converts raw bytes to a UTF-8 string.
// It tries UTF-8 first; if invalid, falls back to GBK decoding.
func decodeToUTF8(raw []byte) string {
	if utf8.Valid(raw) {
		return string(raw)
	}
	// Try GBK (common on Chinese Windows for Java output)
	result, _, err := transform.String(simplifiedchinese.GBK.NewDecoder(), string(raw))
	if err == nil {
		return result
	}
	// Last resort: force UTF-8 with replacement
	decoder := simplifiedchinese.GBK.NewDecoder()
	decoded, _, err := transform.Bytes(decoder, raw)
	if err != nil {
		// Return original with invalid UTF-8 replaced
		return string(raw)
	}
	return string(decoded)
}

// detectLogLevel determines the log level from a line of text.
func detectLogLevel(line string) string {
	upper := " " + strings.ToUpper(line) + " "
	switch {
	case strings.Contains(upper, " ERROR ") || strings.Contains(upper, " ERR "):
		return "error"
	case strings.Contains(upper, " WARN ") || strings.Contains(upper, " WARNING "):
		return "warn"
	case strings.Contains(upper, " DEBUG ") || strings.Contains(upper, " TRACE "):
		return "debug"
	case strings.Contains(upper, " INFO "):
		return "info"
	default:
		lower := strings.ToLower(line)
		if strings.Contains(lower, "exception") || strings.Contains(lower, "traceback") {
			return "error"
		}
		return "info"
	}
}

// resolveJavaCmd returns the appropriate java command based on jdkHome.
func resolveJavaCmd(jdkHome string) string {
	if jdkHome == "" || jdkHome == "java" {
		return "java"
	}
	if strings.HasSuffix(jdkHome, "java") || strings.HasSuffix(jdkHome, "java.exe") {
		return jdkHome
	}
	return jdkHome + "/bin/java"
}

// IsRunning checks if the underlying OS process is still alive.
func (p *ManagedProcess) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.Cmd == nil || p.Cmd.Process == nil {
		return false
	}
	return p.Cmd.Process.Signal(os.Signal(nil)) == nil
}

// filepathFromJar returns the directory containing the JAR file.
func filepathFromJar(jarPath string) string {
	idx := strings.LastIndex(jarPath, "/")
	if idx == -1 {
		idx = strings.LastIndex(jarPath, "\\")
	}
	if idx >= 0 {
		return jarPath[:idx]
	}
	return "."
}
