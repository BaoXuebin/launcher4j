package engine

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// BuildResult holds the outcome of a Maven build.
type BuildResult struct {
	Success    bool
	DurationMs int64
	Output     string
	Errors     []string
}

// MavenBuilder runs Maven commands (compile, package, clean) and captures output.
type MavenBuilder struct {
	mavenPath string
}

// NewMavenBuilder creates a new MavenBuilder.
func NewMavenBuilder(mavenPath string) *MavenBuilder {
	if mavenPath == "" {
		mavenPath = detectMavenCmd()
	}
	return &MavenBuilder{mavenPath: mavenPath}
}

// UpdateMavenPath updates the Maven executable path.
func (b *MavenBuilder) UpdateMavenPath(path string) {
	b.mavenPath = path
	if b.mavenPath == "" {
		b.mavenPath = detectMavenCmd()
	}
}

// GetMvnCmd returns the current Maven command.
func (b *MavenBuilder) GetMvnCmd() string {
	return b.mavenPath
}

// Compile runs `mvn compile` in the given project directory.
func (b *MavenBuilder) Compile(projectPath, projectID string, onLog LogCallback) (*BuildResult, error) {
	return b.runMaven(projectPath, []string{"compile"}, projectID, onLog)
}

// Build runs `mvn package -DskipTests` in the given project directory.
func (b *MavenBuilder) Build(projectPath, projectID string, onLog LogCallback) (*BuildResult, error) {
	return b.runMaven(projectPath, []string{"package", "-DskipTests"}, projectID, onLog)
}

// Clean runs `mvn clean` in the given project directory.
func (b *MavenBuilder) Clean(projectPath, projectID string, onLog LogCallback) (*BuildResult, error) {
	return b.runMaven(projectPath, []string{"clean"}, projectID, onLog)
}

func (b *MavenBuilder) runMaven(
	projectPath string,
	goals []string,
	projectID string,
	onLog LogCallback,
) (*BuildResult, error) {
	// Check pom.xml exists
	pom := projectPath + "/pom.xml"
	if _, err := os.Stat(pom); os.IsNotExist(err) {
		return nil, fmt.Errorf("pom.xml not found in %s", projectPath)
	}

	if onLog != nil {
		onLog(projectID, "build", fmt.Sprintf("▶ Running: mvn %s ...", strings.Join(goals, " ")))
	}

	start := time.Now()

	// Build command
	cmd := exec.Command(b.mavenPath, goals...)
	cmd.Dir = projectPath

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf(
			"Maven command '%s' not found. Install Maven or configure the path in settings.",
			b.mavenPath,
		)
	}

	var outputLines []string
	var errors []string

	// Read stdout
	stdoutScanner := bufio.NewScanner(stdout)
	go func() {
		for stdoutScanner.Scan() {
			line := stdoutScanner.Text()
			outputLines = append(outputLines, line)

			level := "info"
			if strings.Contains(line, "ERROR") || strings.Contains(line, "FAILURE") {
				level = "error"
				errors = append(errors, line)
			} else if strings.Contains(line, "WARNING") {
				level = "warn"
			}

			if onLog != nil {
				onLog(projectID, level, line)
			}
		}
	}()

	// Read stderr
	stderrScanner := bufio.NewScanner(stderr)
	for stderrScanner.Scan() {
		line := stderrScanner.Text()
		outputLines = append(outputLines, "[ERR] "+line)
		errors = append(errors, line)
		if onLog != nil {
			onLog(projectID, "error", line)
		}
	}

	// Wait for stdout goroutine to finish
	cmd.Wait()
	duration := time.Since(start).Milliseconds()
	success := cmd.ProcessState.Success()

	if onLog != nil {
		status := "SUCCESS"
		if !success {
			status = "FAILED"
		}
		icon := "✓"
		if !success {
			icon = "✗"
		}
		level := "info"
		if !success {
			level = "error"
		}
		onLog(projectID, level,
			fmt.Sprintf("%s Build %s in %.1fs", icon, status, float64(duration)/1000))
	}

	return &BuildResult{
		Success:    success,
		DurationMs: duration,
		Output:     strings.Join(outputLines, "\n"),
		Errors:     errors,
	}, nil
}

// detectMavenCmd returns the platform-appropriate Maven command.
func detectMavenCmd() string {
	if runtime.GOOS == "windows" {
		return "mvn.cmd"
	}
	return "mvn"
}
