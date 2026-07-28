"""Maven build process management."""

import os
import subprocess
import time
from typing import Callable, List, Optional
from dataclasses import dataclass


@dataclass
class BuildResult:
    success: bool
    duration_ms: int
    output: str
    errors: List[str]


class MavenBuilder:
    """Runs Maven commands (compile, package, clean) and captures output."""

    def __init__(self, maven_path: Optional[str] = None):
        self._maven_path = maven_path

    def update_maven_path(self, path: Optional[str]):
        self._maven_path = path

    def _get_mvn_cmd(self) -> str:
        if self._maven_path:
            return self._maven_path
        if os.name == "nt":
            return "mvn.cmd"
        return "mvn"

    def compile(
        self,
        project_path: str,
        project_id: str,
        on_log: Optional[Callable[[str, str, str], None]] = None,
    ) -> BuildResult:
        return self._run_maven(project_path, ["compile"], project_id, on_log)

    def build(
        self,
        project_path: str,
        project_id: str,
        on_log: Optional[Callable[[str, str, str], None]] = None,
    ) -> BuildResult:
        return self._run_maven(project_path, ["package", "-DskipTests"], project_id, on_log)

    def clean(
        self,
        project_path: str,
        project_id: str,
        on_log: Optional[Callable[[str, str, str], None]] = None,
    ) -> BuildResult:
        return self._run_maven(project_path, ["clean"], project_id, on_log)

    def _run_maven(
        self,
        project_path: str,
        goals: List[str],
        project_id: str,
        on_log: Optional[Callable[[str, str, str], None]] = None,
    ) -> BuildResult:
        # Check pom.xml exists
        pom = os.path.join(project_path, "pom.xml")
        if not os.path.exists(pom):
            raise FileNotFoundError(f"pom.xml not found in {project_path}")

        if on_log:
            on_log(project_id, "build", f"▶ Running: mvn {' '.join(goals)} ...")

        start = time.time()
        cmd = [self._get_mvn_cmd()] + goals

        try:
            proc = subprocess.Popen(
                cmd,
                cwd=project_path,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
                bufsize=1,
            )
        except FileNotFoundError:
            raise RuntimeError(
                f"Maven command '{cmd[0]}' not found. "
                "Install Maven or configure the path in settings."
            )

        output_lines: List[str] = []
        errors: List[str] = []

        def read_stream(stream, is_error: bool):
            for line in stream:
                line = line.rstrip("\n\r")
                output_lines.append(line)

                level = "error" if is_error or "ERROR" in line or "FAILURE" in line else (
                    "warn" if "WARNING" in line else "info"
                )
                if is_error or "ERROR" in line or "FAILURE" in line:
                    errors.append(line)

                if on_log:
                    on_log(project_id, level, line)

        # Read stdout
        if proc.stdout:
            read_stream(proc.stdout, False)

        # Read stderr
        if proc.stderr:
            read_stream(proc.stderr, True)

        proc.wait()
        duration = int((time.time() - start) * 1000)
        success = proc.returncode == 0

        # Try to read remaining output
        try:
            remaining_out, remaining_err = proc.communicate(timeout=2)
            if remaining_out:
                for line in remaining_out.splitlines():
                    line = line.rstrip("\n\r")
                    output_lines.append(line)
                    if on_log:
                        on_log(project_id, "info", line)
            if remaining_err:
                for line in remaining_err.splitlines():
                    line = line.rstrip("\n\r")
                    output_lines.append(line)
                    errors.append(line)
                    if on_log:
                        on_log(project_id, "error", line)
        except subprocess.TimeoutExpired:
            pass

        if on_log:
            status = "SUCCESS" if success else "FAILED"
            on_log(project_id, "info" if success else "error",
                   f"{'✓' if success else '✗'} Build {status} in {duration / 1000:.1f}s")

        return BuildResult(
            success=success,
            duration_ms=duration,
            output="\n".join(output_lines),
            errors=errors,
        )
