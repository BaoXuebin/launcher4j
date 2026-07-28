"""Java process management."""

import os
import signal
import subprocess
import threading
import time
from enum import Enum
from typing import Callable, Optional


class ProcessStatus(str, Enum):
    STOPPED = "stopped"
    STARTING = "starting"
    RUNNING = "running"
    STOPPING = "stopping"
    ERROR = "error"


class ManagedProcess:
    """Manages a single Java process."""

    def __init__(self, project_id: str, name: str):
        self.project_id = project_id
        self.name = name
        self.process: Optional[subprocess.Popen] = None
        self.status = ProcessStatus.STOPPED
        self.pid: Optional[int] = None
        self.port: Optional[int] = None

    def is_running(self) -> bool:
        if self.process and self.process.poll() is None:
            return True
        return False


class ProcessManager:
    """Manages multiple Java processes across projects."""

    def __init__(self):
        self._processes: dict[str, ManagedProcess] = {}
        self._lock = threading.Lock()

    def start(
        self,
        project_id: str,
        name: str,
        jar_path: str,
        jdk_home: str,
        vm_args: str,
        on_log: Optional[Callable[[str, str, str], None]] = None,
        on_status: Optional[Callable[[str, str], None]] = None,
    ) -> ManagedProcess:
        with self._lock:
            if project_id in self._processes:
                existing = self._processes[project_id]
                if existing.is_running():
                    raise RuntimeError(f"Project '{name}' is already running")

            java_cmd = jdk_home if jdk_home and jdk_home != "java" else "java"
            if jdk_home and jdk_home != "java" and not jdk_home.endswith("java"):
                java_cmd = os.path.join(jdk_home, "bin", "java")

            args = [java_cmd]
            if vm_args:
                args.extend(vm_args.split())
            args.extend(["-jar", jar_path])

            proc = ManagedProcess(project_id, name)

            try:
                process = subprocess.Popen(
                    args,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    text=True,
                    bufsize=1,
                )
            except FileNotFoundError:
                raise RuntimeError(
                    f"Java command '{java_cmd}' not found. "
                    "Install JDK or configure the path."
                )

            proc.process = process
            proc.pid = process.pid
            proc.status = ProcessStatus.STARTING

            self._processes[project_id] = proc

        # Notify starting
        if on_status:
            on_status(project_id, "starting")

        # Read stdout in background
        def read_stdout():
            if not process.stdout:
                return
            for line in process.stdout:
                line = line.rstrip("\n\r")
                if on_log:
                    level = "error" if any(w in line.lower() for w in ["error", "exception", "traceback"]) else "info"
                    on_log(project_id, level, line)

                # Detect port
                if "Tomcat initialized with port" in line or "port" in line.lower():
                    import re
                    m = re.search(r"port\D*(\d+)", line, re.IGNORECASE)
                    if m:
                        proc.port = int(m.group(1))

                # Detect startup completion
                if "Started" in line and "seconds" in line:
                    with self._lock:
                        proc.status = ProcessStatus.RUNNING
                    if on_status:
                        on_status(project_id, "running")

            # Process exited
            with self._lock:
                proc.status = ProcessStatus.STOPPED
                proc.pid = None
                proc.port = None
            if on_status:
                on_status(project_id, "stopped")

        def read_stderr():
            if not process.stderr:
                return
            for line in process.stderr:
                line = line.rstrip("\n\r")
                if on_log:
                    on_log(project_id, "error", line)

        t1 = threading.Thread(target=read_stdout, daemon=True)
        t2 = threading.Thread(target=read_stderr, daemon=True)
        t1.start()
        t2.start()

        # Mark as running after a brief delay (if not already marked)
        def check_running():
            time.sleep(2)
            with self._lock:
                if proc.status == ProcessStatus.STARTING:
                    proc.status = ProcessStatus.RUNNING
                    if on_status:
                        on_status(project_id, "running")

        threading.Thread(target=check_running, daemon=True).start()

        return proc

    def stop(self, project_id: str) -> bool:
        with self._lock:
            proc = self._processes.get(project_id)
            if not proc or not proc.process:
                return False

            proc.status = ProcessStatus.STOPPING
            process = proc.process

        if os.name == "nt":
            # Use taskkill to ensure child processes are killed
            try:
                subprocess.run(
                    ["taskkill", "/F", "/T", "/PID", str(process.pid)],
                    capture_output=True, timeout=5,
                )
            except subprocess.TimeoutExpired:
                pass
        else:
            process.terminate()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.kill()

        with self._lock:
            proc.status = ProcessStatus.STOPPED
            proc.pid = None
            proc.port = None
            proc.process = None

        return True

    def status(self, project_id: str) -> ProcessStatus:
        with self._lock:
            proc = self._processes.get(project_id)
            if not proc:
                return ProcessStatus.STOPPED
            if proc.process and proc.process.poll() is not None and proc.status in (
                ProcessStatus.RUNNING, ProcessStatus.STARTING
            ):
                proc.status = ProcessStatus.STOPPED
                proc.pid = None
                proc.port = None
            return proc.status

    def get_process(self, project_id: str) -> Optional[ManagedProcess]:
        with self._lock:
            return self._processes.get(project_id)

    def get_all_processes(self) -> dict[str, ManagedProcess]:
        with self._lock:
            return dict(self._processes)

    def shutdown_all(self):
        """Kill all running processes."""
        with self._lock:
            ids = list(self._processes.keys())

        for pid in ids:
            self.stop(pid)
