"""Configuration storage using JSON."""

import json
import os
from dataclasses import dataclass, field, asdict
from typing import List, Optional
from pathlib import Path


@dataclass
class ProjectConfig:
    id: str
    name: str
    path: str
    jdk_home: str = "java"
    vm_args: str = ""
    auto_compile: bool = True
    added_at: str = ""


@dataclass
class AppSettings:
    theme: str = "dark"
    maven_path: str = ""
    auto_compile_debounce_ms: int = 500


@dataclass
class AppConfig:
    projects: List[ProjectConfig] = field(default_factory=list)
    settings: AppSettings = field(default_factory=AppSettings)


class ConfigStore:
    """Manages persistent configuration stored as JSON."""

    def __init__(self, config_dir: Optional[str] = None):
        if config_dir:
            self.config_dir = Path(config_dir)
        else:
            if os.name == "nt":
                base = Path(os.environ.get("APPDATA", Path.home() / "AppData" / "Roaming"))
            else:
                base = Path.home() / ".config"
            self.config_dir = base / "launcher4j"

        self.config_dir.mkdir(parents=True, exist_ok=True)
        self.config_path = self.config_dir / "launcher4j.json"

    def _load_raw(self) -> dict:
        try:
            with open(self.config_path, "r", encoding="utf-8") as f:
                return json.load(f)
        except (FileNotFoundError, json.JSONDecodeError):
            return {}

    def _save_raw(self, data: dict):
        with open(self.config_path, "w", encoding="utf-8") as f:
            json.dump(data, f, indent=2, ensure_ascii=False)

    def load_projects(self) -> List[ProjectConfig]:
        data = self._load_raw()
        projects_data = data.get("projects", [])
        return [ProjectConfig(**p) for p in projects_data]

    def save_project(self, project: ProjectConfig):
        data = self._load_raw()
        projects = data.get("projects", [])
        # Update or append
        for i, p in enumerate(projects):
            if p["id"] == project.id:
                projects[i] = asdict(project)
                break
        else:
            projects.append(asdict(project))
        data["projects"] = projects
        self._save_raw(data)

    def remove_project(self, project_id: str):
        data = self._load_raw()
        projects = data.get("projects", [])
        data["projects"] = [p for p in projects if p["id"] != project_id]
        self._save_raw(data)

    def load_settings(self) -> AppSettings:
        data = self._load_raw()
        settings_data = data.get("settings", {})
        return AppSettings(**settings_data)

    def save_settings(self, settings: AppSettings):
        data = self._load_raw()
        data["settings"] = asdict(settings)
        self._save_raw(data)
