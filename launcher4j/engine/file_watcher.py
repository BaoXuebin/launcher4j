"""File system watcher for auto-compilation."""

import os
import threading
import time
from typing import Callable, Optional

from watchdog.events import FileSystemEventHandler
from watchdog.observers import Observer


class JavaFileHandler(FileSystemEventHandler):
    """Watchdog handler that triggers on .java file changes."""

    def __init__(
        self,
        project_id: str,
        project_path: str,
        debounce_ms: int,
        on_change: Callable[[str], None],
    ):
        self.project_id = project_id
        self.project_path = project_path
        self.debounce_ms = debounce_ms
        self.on_change = on_change
        self._timer: Optional[threading.Timer] = None
        self._lock = threading.Lock()

    def on_modified(self, event):
        if event.is_directory:
            return
        if not event.src_path.endswith(".java"):
            return

        # Debounce: reset timer on each change
        with self._lock:
            if self._timer and self._timer.is_alive():
                self._timer.cancel()
            self._timer = threading.Timer(
                self.debounce_ms / 1000.0,
                self._fire,
            )
            self._timer.daemon = True
            self._timer.start()

    def _fire(self):
        self.on_change(self.project_id)


class FileWatcher:
    """Watches project src/ directories for .java file changes."""

    def __init__(self):
        self._observers: dict[str, tuple[Observer, JavaFileHandler]] = {}
        self._lock = threading.Lock()

    def watch(
        self,
        project_id: str,
        project_path: str,
        debounce_ms: int = 500,
        on_compile: Optional[Callable[[str], None]] = None,
    ) -> bool:
        src_dir = os.path.join(project_path, "src")
        if not os.path.isdir(src_dir):
            return False

        self.unwatch(project_id)

        handler = JavaFileHandler(project_id, project_path, debounce_ms,
                                   lambda pid: on_compile(pid) if on_compile else None)
        observer = Observer()
        observer.schedule(handler, src_dir, recursive=True)
        observer.daemon = True
        observer.start()

        with self._lock:
            self._observers[project_id] = (observer, handler)

        return True

    def unwatch(self, project_id: str):
        with self._lock:
            if project_id in self._observers:
                observer, handler = self._observers.pop(project_id)
                observer.stop()
                observer.join(timeout=3)

    def unwatch_all(self):
        with self._lock:
            for project_id in list(self._observers.keys()):
                self.unwatch(project_id)

    def update_debounce(self, project_id: str, debounce_ms: int):
        with self._lock:
            if project_id in self._observers:
                _, handler = self._observers[project_id]
                handler.debounce_ms = debounce_ms
