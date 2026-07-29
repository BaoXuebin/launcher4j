package engine

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileWatcher watches project src/ directories for .java file changes
// and triggers debounced compile callbacks.
type FileWatcher struct {
	mu        sync.Mutex
	watchers  map[string]*projectWatcher
}

type projectWatcher struct {
	watcher    *fsnotify.Watcher
	projectID  string
	projectPath string
	debounceMs int
	onChange   func(string)
	timer      *time.Timer
	mu         sync.Mutex
	done       chan struct{}
}

// NewFileWatcher creates a new FileWatcher.
func NewFileWatcher() *FileWatcher {
	return &FileWatcher{
		watchers: make(map[string]*projectWatcher),
	}
}

// Watch starts watching a project's src/ directory for .java file changes.
// Returns true if watching was started successfully.
func (fw *FileWatcher) Watch(projectID, projectPath string, debounceMs int, onChange func(string)) bool {
	srcDir := filepath.Join(projectPath, "src")
	info, err := os.Stat(srcDir)
	if err != nil || !info.IsDir() {
		return false
	}

	fw.Unwatch(projectID)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return false
	}

	pw := &projectWatcher{
		watcher:     watcher,
		projectID:   projectID,
		projectPath: projectPath,
		debounceMs:  debounceMs,
		onChange:    onChange,
		done:        make(chan struct{}),
	}

	fw.mu.Lock()
	fw.watchers[projectID] = pw
	fw.mu.Unlock()

	// Add all subdirectories recursively
	filepath.Walk(srcDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() {
			watcher.Add(path)
		}
		return nil
	})

	go pw.loop()

	return true
}

// Unwatch stops watching a project.
func (fw *FileWatcher) Unwatch(projectID string) {
	fw.mu.Lock()
	pw, ok := fw.watchers[projectID]
	if ok {
		delete(fw.watchers, projectID)
	}
	fw.mu.Unlock()

	if ok {
		close(pw.done)
		pw.watcher.Close()
	}
}

// UnwatchAll stops all watchers.
func (fw *FileWatcher) UnwatchAll() {
	fw.mu.Lock()
	ids := make([]string, 0, len(fw.watchers))
	for id := range fw.watchers {
		ids = append(ids, id)
	}
	fw.mu.Unlock()

	for _, id := range ids {
		fw.Unwatch(id)
	}
}

// UpdateDebounce updates the debounce interval for a project watcher.
func (fw *FileWatcher) UpdateDebounce(projectID string, debounceMs int) {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	if pw, ok := fw.watchers[projectID]; ok {
		pw.mu.Lock()
		pw.debounceMs = debounceMs
		pw.mu.Unlock()
	}
}

func (pw *projectWatcher) loop() {
	for {
		select {
		case event, ok := <-pw.watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Create|fsnotify.Write) == 0 {
				continue
			}
			if !strings.HasSuffix(event.Name, ".java") {
				continue
			}
			pw.debounceFire()

		case err, ok := <-pw.watcher.Errors:
			if !ok {
				return
			}
			_ = err // silently ignore watch errors

		case <-pw.done:
			return
		}
	}
}

func (pw *projectWatcher) debounceFire() {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	if pw.timer != nil {
		pw.timer.Stop()
	}

	pw.timer = time.AfterFunc(time.Duration(pw.debounceMs)*time.Millisecond, func() {
		if pw.onChange != nil {
			pw.onChange(pw.projectID)
		}
	})
}
