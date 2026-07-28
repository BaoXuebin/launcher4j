"""Custom UI widgets for Launcher4j."""

from PySide6.QtCore import Qt, Signal, QSize
from PySide6.QtGui import QFont, QColor, QPainter, QBrush, QPen
from PySide6.QtWidgets import (
    QWidget, QVBoxLayout, QHBoxLayout, QLabel, QPushButton,
    QLineEdit, QFrame, QScrollArea, QPlainTextEdit, QSizePolicy,
)

from launcher4j.engine.process_manager import ProcessStatus


class StatusDot(QWidget):
    """Small colored circle indicating process status."""

    def __init__(self, status: ProcessStatus = ProcessStatus.STOPPED, parent=None):
        super().__init__(parent)
        self._status = status
        self.setFixedSize(10, 10)

    def set_status(self, status: ProcessStatus):
        self._status = status
        self.update()

    def paintEvent(self, event):
        p = QPainter(self)
        p.setRenderHint(QPainter.RenderHint.Antialiasing)
        p.setPen(Qt.PenStyle.NoPen)

        colors = {
            ProcessStatus.RUNNING: QColor("#22c55e"),
            ProcessStatus.STARTING: QColor("#f59e0b"),
            ProcessStatus.STOPPING: QColor("#f59e0b"),
            ProcessStatus.ERROR: QColor("#ef4444"),
            ProcessStatus.STOPPED: QColor("#5c687f"),
        }
        color = colors.get(self._status, QColor("#5c687f"))
        p.setBrush(QBrush(color))
        p.drawEllipse(0, 0, 10, 10)

        # Glow effect for running
        if self._status == ProcessStatus.RUNNING:
            p.setBrush(QBrush(QColor(34, 197, 94, 40)))
            p.drawEllipse(-1, -1, 12, 12)


class ProjectListItem(QFrame):
    """A clickable project item in the sidebar."""

    clicked = Signal(str)
    remove_clicked = Signal(str)

    def __init__(self, project_id: str, name: str, parent=None):
        super().__init__(parent)
        self.project_id = project_id
        self._selected = False

        self.setObjectName("project-item")
        self.setCursor(Qt.CursorShape.PointingHandCursor)

        layout = QHBoxLayout(self)
        layout.setContentsMargins(8, 8, 8, 8)
        layout.setSpacing(8)

        self.dot = StatusDot(ProcessStatus.STOPPED)
        layout.addWidget(self.dot)

        self.name_label = QLabel(name)
        self.name_label.setObjectName("project-name")
        layout.addWidget(self.name_label, 1)

        self.status_label = QLabel("已停止")
        self.status_label.setObjectName("project-status-text")
        layout.addWidget(self.status_label)

        layout.addSpacing(4)

    def mousePressEvent(self, event):
        if event.button() == Qt.MouseButton.LeftButton:
            self.clicked.emit(self.project_id)
        super().mousePressEvent(event)

    def set_selected(self, selected: bool):
        self._selected = selected
        if selected:
            self.setProperty("selected", True)
            self.name_label.setObjectName("project-name-selected")
        else:
            self.setProperty("selected", False)
            self.name_label.setObjectName("project-name")
        self.style().unpolish(self)
        self.style().polish(self)

    def update_status(self, status: ProcessStatus, status_text: str):
        self.dot.set_status(status)
        self.status_label.setText(status_text)


class Sidebar(QWidget):
    """Left sidebar with project list."""

    project_selected = Signal(str)
    add_project = Signal()
    open_settings = Signal()

    def __init__(self, parent=None):
        super().__init__(parent)
        self.setObjectName("sidebar")
        self.setFixedWidth(240)

        layout = QVBoxLayout(self)
        layout.setContentsMargins(0, 0, 0, 0)
        layout.setSpacing(0)

        # Header
        header = QWidget()
        header.setObjectName("sidebar-header")
        header_layout = QHBoxLayout(header)
        header_layout.setContentsMargins(14, 12, 14, 12)

        title = QLabel("⚡ Launcher4j")
        title.setObjectName("sidebar-title")
        header_layout.addWidget(title)

        settings_btn = QPushButton("⚙")
        settings_btn.setObjectName("btn-ghost")
        settings_btn.setFixedSize(28, 28)
        settings_btn.clicked.connect(self.open_settings.emit)
        header_layout.addWidget(settings_btn)

        layout.addWidget(header)

        # Search
        search_container = QWidget()
        search_container.setContentsMargins(10, 8, 10, 4)
        search_layout = QVBoxLayout(search_container)
        search_layout.setContentsMargins(0, 0, 0, 0)

        self.search_input = QLineEdit()
        self.search_input.setObjectName("search-input")
        self.search_input.setPlaceholderText("🔍 搜索项目...")
        self.search_input.textChanged.connect(self._filter)
        search_layout.addWidget(self.search_input)

        layout.addWidget(search_container)

        # Project list (scrollable)
        scroll = QScrollArea()
        scroll.setWidgetResizable(True)
        scroll.setHorizontalScrollBarPolicy(Qt.ScrollBarPolicy.ScrollBarAlwaysOff)
        scroll.setFrameShape(QFrame.Shape.NoFrame)

        self.list_widget = QWidget()
        self.list_layout = QVBoxLayout(self.list_widget)
        self.list_layout.setContentsMargins(4, 4, 4, 4)
        self.list_layout.setSpacing(1)
        self.list_layout.addStretch()

        scroll.setWidget(self.list_widget)
        layout.addWidget(scroll, 1)

        # Add button
        btn_container = QWidget()
        btn_container.setContentsMargins(10, 8, 10, 10)
        btn_layout = QVBoxLayout(btn_container)
        btn_layout.setContentsMargins(0, 0, 0, 0)

        add_btn = QPushButton("+ 添加项目")
        add_btn.setObjectName("btn-primary")
        add_btn.clicked.connect(self.add_project.emit)
        btn_layout.addWidget(add_btn)

        layout.addWidget(btn_container)

        self.items: dict[str, ProjectListItem] = {}
        self._filter_text = ""

    def _filter(self, text: str):
        self._filter_text = text.lower()
        for pid, item in self.items.items():
            item.setVisible(
                not text or text.lower() in item.name_label.text().lower()
            )

    def add_item(self, project_id: str, name: str):
        item = ProjectListItem(project_id, name)
        item.clicked.connect(self.project_selected.emit)

        # Insert before the stretch
        self.list_layout.insertWidget(self.list_layout.count() - 1, item)
        self.items[project_id] = item
        item.setVisible(
            not self._filter_text
            or self._filter_text in name.lower()
        )
        return item

    def remove_item(self, project_id: str):
        if project_id in self.items:
            item = self.items.pop(project_id)
            self.list_layout.removeWidget(item)
            item.deleteLater()

    def select_item(self, project_id: str):
        for pid, item in self.items.items():
            item.set_selected(pid == project_id)

    def update_item_status(self, project_id: str, status: ProcessStatus):
        if project_id in self.items:
            status_texts = {
                ProcessStatus.RUNNING: "运行中",
                ProcessStatus.STARTING: "启动中",
                ProcessStatus.STOPPING: "停止中",
                ProcessStatus.ERROR: "异常",
                ProcessStatus.STOPPED: "已停止",
            }
            self.items[project_id].update_status(
                status, status_texts.get(status, "未知")
            )


class LogOutput(QPlainTextEdit):
    """Monospace log output widget with ANSI color support."""

    def __init__(self, parent=None):
        super().__init__(parent)
        self.setObjectName("log-output")
        self.setReadOnly(True)
        self.setMaximumBlockCount(10000)
        self.setLineWrapMode(QPlainTextEdit.LineWrapMode.NoWrap)
        self.setVerticalScrollBarPolicy(Qt.ScrollBarPolicy.ScrollBarAsNeeded)
        self.setHorizontalScrollBarPolicy(Qt.ScrollBarPolicy.ScrollBarAsNeeded)

    def append_log(self, timestamp: str, level: str, message: str):
        color_map = {
            "error": "#ef4444",
            "warn": "#f59e0b",
            "build": "#4cb2ff",
            "debug": "#5c687f",
            "info": "#c8cfe0",
        }
        color = color_map.get(level, "#c8cfe0")

        prefix = ""
        if level == "error":
            prefix = "ERR "
        elif level == "warn":
            prefix = "WARN"
        elif level == "build":
            prefix = "BUILD"

        # Format: "HH:MM:SS.mmm LEVEL message"
        line = f"<span style='color:#5c687f'>{timestamp}</span> "
        line += f"<span style='color:{color};font-weight:600'>{prefix}</span> "
        line += f"<span style='color:{color}'>{self._escape(message)}</span>"

        self.appendHtml(line)

        # Auto-scroll
        sb = self.verticalScrollBar()
        if sb:
            sb.setValue(sb.maximum())

    def _escape(self, text: str) -> str:
        return (text.replace("&", "&amp;")
                    .replace("<", "&lt;")
                    .replace(">", "&gt;")
                    .replace("\n", "<br>"))


class LogToolbar(QFrame):
    """Toolbar above the log panel."""

    start_clicked = Signal()
    stop_clicked = Signal()
    restart_clicked = Signal()
    compile_clicked = Signal()
    build_clicked = Signal()
    clear_clicked = Signal()

    def __init__(self, parent=None):
        super().__init__(parent)
        self.setObjectName("log-toolbar")

        layout = QHBoxLayout(self)
        layout.setContentsMargins(8, 4, 8, 4)
        layout.setSpacing(4)

        # Project info area
        self.info_container = QWidget()
        info_layout = QHBoxLayout(self.info_container)
        info_layout.setContentsMargins(0, 0, 0, 0)
        info_layout.setSpacing(8)

        self.project_name = QLabel("选择项目")
        self.project_name.setStyleSheet("font-weight:700;font-size:13px;color:#e2e6f0")
        info_layout.addWidget(self.project_name)

        self.badge = QLabel()
        self.badge.setObjectName("badge-neutral")
        info_layout.addWidget(self.badge)

        self.port_label = QLabel()
        self.port_label.setObjectName("badge-info")
        info_layout.addWidget(self.port_label)

        layout.addWidget(self.info_container, 1)

        # Spacer
        layout.addStretch()

        # Buttons (right side)
        self.compile_btn = QPushButton("⚡ 编译")
        self.compile_btn.setObjectName("btn-ghost")
        self.compile_btn.clicked.connect(self.compile_clicked.emit)
        layout.addWidget(self.compile_btn)

        self.build_btn = QPushButton("📦 打包")
        self.build_btn.setObjectName("btn-ghost")
        self.build_btn.clicked.connect(self.build_clicked.emit)
        layout.addWidget(self.build_btn)

        sep = QFrame()
        sep.setFrameShape(QFrame.Shape.VLine)
        sep.setStyleSheet("background:#2e3345;max-width:1px;margin:4px 2px")
        layout.addWidget(sep)

        self.start_btn = QPushButton("▶ 启动")
        self.start_btn.setObjectName("btn-success")
        self.start_btn.clicked.connect(self.start_clicked.emit)
        layout.addWidget(self.start_btn)

        self.stop_btn = QPushButton("⏹ 停止")
        self.stop_btn.setObjectName("btn-danger")
        self.stop_btn.clicked.connect(self.stop_clicked.emit)
        self.stop_btn.setVisible(False)
        layout.addWidget(self.stop_btn)

        self.restart_btn = QPushButton("↻ 重启")
        self.restart_btn.setObjectName("btn-ghost")
        self.restart_btn.clicked.connect(self.restart_clicked.emit)
        layout.addWidget(self.restart_btn)

        sep2 = QFrame()
        sep2.setFrameShape(QFrame.Shape.VLine)
        sep2.setStyleSheet("background:#2e3345;max-width:1px;margin:4px 2px")
        layout.addWidget(sep2)

        clear_btn = QPushButton("🗑")
        clear_btn.setObjectName("btn-ghost")
        clear_btn.setToolTip("清空日志")
        clear_btn.clicked.connect(self.clear_clicked.emit)
        layout.addWidget(clear_btn)

    def set_project_info(self, name: str, status: ProcessStatus, port: int = None):
        self.project_name.setText(name)

        badge_map = {
            ProcessStatus.RUNNING: ("badge-success", "● 运行中"),
            ProcessStatus.STARTING: ("badge-warning", "◌ 启动中"),
            ProcessStatus.STOPPING: ("badge-warning", "◌ 停止中"),
            ProcessStatus.ERROR: ("badge-error", "✕ 异常"),
            ProcessStatus.STOPPED: ("badge-neutral", "○ 已停止"),
        }
        obj_name, text = badge_map.get(status, ("badge-neutral", "○ 未知"))
        self.badge.setObjectName(obj_name)
        self.badge.setText(text)
        self.badge.style().unpolish(self.badge)
        self.badge.style().polish(self.badge)

        if port:
            self.port_label.setText(f": {port}")
            self.port_label.setVisible(True)
        else:
            self.port_label.setVisible(False)

        is_running = status in (ProcessStatus.RUNNING, ProcessStatus.STARTING)
        self.start_btn.setVisible(not is_running)
        self.stop_btn.setVisible(is_running)


class StatusBar(QFrame):
    """Bottom status bar."""

    def __init__(self, parent=None):
        super().__init__(parent)
        self.setObjectName("statusbar")

        layout = QHBoxLayout(self)
        layout.setContentsMargins(12, 3, 12, 3)

        self.status_left = QLabel("● 0/0 项目运行中")
        self.status_left.setStyleSheet("color:#5c687f;font-size:11px")
        layout.addWidget(self.status_left)

        layout.addStretch()

        self.status_right = QLabel("Launcher4j v0.1.0")
        self.status_right.setStyleSheet("color:#5c687f;font-size:11px")
        layout.addWidget(self.status_right)

    def update_status(self, running: int, total: int):
        self.status_left.setText(f"● {running}/{total} 项目运行中")
