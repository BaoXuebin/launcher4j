"""Ring buffer for log entries."""

from collections import defaultdict, deque
from dataclasses import dataclass, field
from typing import List


@dataclass
class LogEntry:
    timestamp: str
    level: str  # info, warn, error, debug, build
    message: str


class LogBuffer:
    """Thread-safe log buffer with per-project ring buffers."""

    def __init__(self, max_entries: int = 5000):
        self._max = max_entries
        self._buffers: dict[str, deque[LogEntry]] = defaultdict(
            lambda: deque(maxlen=max_entries)
        )

    def append(self, project_id: str, entry: LogEntry):
        self._buffers[project_id].append(entry)

    def get(self, project_id: str) -> List[LogEntry]:
        return list(self._buffers.get(project_id, []))

    def clear(self, project_id: str):
        self._buffers.pop(project_id, None)

    def remove(self, project_id: str):
        self._buffers.pop(project_id, None)
