"""Main application window."""

import os
from datetime import datetime

from PySide6.QtCore import Qt, QTimer
from PySide6.QtGui import QAction, QIcon
from PySide6.QtWidgets import (
    QMainWindow, QWidget, QHBoxLayout, QVBoxLayout,
    QSplitter, QSystemTrayIcon, QMenu, QMessageBox, QApplication,
)

from launcher4j import __version__
from launcher4j.config.store import ConfigStore, ProjectConfig, AppSettings
from launcher4j.engine.log_buffer import LogBuffer, LogEntry
from launcher4j.engine.process_manager import ProcessManager, ProcessStatus
from launcher4j.engine.maven_builder import MavenBuilder
from launcher4j.engine.file_watcher import FileWatcher
from launcher4j.ui.widgets import Sidebar, LogOutput, LogToolbar, StatusBar
from launcher4j.ui.dialogs import AddProjectDialog, SettingsDialog


class MainWindow(QMainWindow):
    """Main application window for Launcher4j."""

    def __init__(self):
        super().__init__()
        self.setWindowTitle(f"Launcher4j - Java Project Launcher")
        self.setMinimumSize(900, 600)
        self.resize(1100, 750)

        # Core engine
        self.config_store = ConfigStore()
        self.log_buffer = LogBuffer()
        self.process_manager = ProcessManager()
        self.file_watcher = FileWatcher()

        settings = self.config_store.load_settings()
        self.maven_builder = MavenBuilder(
            settings.maven_path if settings.maven_path else None
        )
        self._settings = settings

        # State
        self._selected_project_id: str | None = None
        self._projects: dict[str, ProjectConfig] = {}
        self._compile_timers: dict[str, QTimer] = {}

        # Build UI
        self._setup_ui()
        self._setup_tray()
        self._load_projects()
        self._setup_auto_compile()

    def _setup_ui(self):
        """Create the main UI layout."""
        central = QWidget()
        self.setCentralWidget(central)

        main_layout = QVBoxLayout(central)
        main_layout.setContentsMargins(0, 0, 0, 0)
        main_layout.setSpacing(0)

        # Content: sidebar + main area
        content = QWidget()
        content_layout = QHBoxLayout(content)
        content_layout.setContentsMargins(0, 0, 0, 0)
        content_layout.setSpacing(0)

        # Sidebar
        self.sidebar = Sidebar()
        self.sidebar.project_selected.connect(self._on_project_selected)
        self.sidebar.add_project.connect(self._on_add_project)
        self.sidebar.open_settings.connect(self._on_open_settings)
        content_layout.addWidget(self.sidebar)

        # Right panel
        right_panel = QWidget()
        right_layout = QVBoxLayout(right_panel)
        right_layout.setContentsMargins(0, 0, 0, 0)
        right_layout.setSpacing(0)

        # Log toolbar
        self.log_toolbar = LogToolbar()
        self.log_toolbar.start_clicked.connect(self._on_start)
        self.log_toolbar.stop_clicked.connect(self._on_stop)
        self.log_toolbar.restart_clicked.connect(self._on_restart)
        self.log_toolbar.compile_clicked.connect(self._on_compile)
        self.log_toolbar.build_clicked.connect(self._on_build)
        self.log_toolbar.clear_clicked.connect(self._on_clear_logs)
        right_layout.addWidget(self.log_toolbar)

        # Log output
        self.log_output = LogOutput()
        self.log_output.setVisible(False)
        right_layout.addWidget(self.log_output, 1)

        # Welcome page (shown when no project selected)
        self.welcome = QWidget()
        welcome_layout = QVBoxLayout(self.welcome)
        welcome_layout.setAlignment(Qt.AlignmentFlag.AlignCenter)
        welcome_label = QLabel(
            "从左侧添加一个 Maven 项目开始使用\n\n"
            "支持自动编译、启动、停止、重启和打包"
        )
        welcome_label.setAlignment(Qt.AlignmentFlag.AlignCenter)
        welcome_label.setStyleSheet("color:#5c687f;font-size:14px;padding:40px")
        welcome_layout.addWidget(welcome_label)
        right_layout.addWidget(self.welcome, 1)

        content_layout.addWidget(right_panel, 1)

        main_layout.addWidget(content, 1)

        # Status bar
        self.status_bar = StatusBar()
        main_layout.addWidget(self.status_bar)

    def _setup_tray(self):
        """Create system tray icon."""
        self.tray_icon = QSystemTrayIcon(self)
        # Use a simple icon - for production, use a proper icon file
        icon = QIcon()
        self.tray_icon.setIcon(icon)
        self.tray_icon.setToolTip("Launcher4j - Java Project Launcher")

        tray_menu = QMenu()
        show_action = QAction("显示窗口", self)
        show_action.triggered.connect(self.show_and_raise)
        tray_menu.addAction(show_action)
        tray_menu.addSeparator()
        quit_action = QAction("退出", self)
        quit_action.triggered.connect(self._quit_app)
        tray_menu.addAction(quit_action)

        self.tray_icon.setContextMenu(tray_menu)
        self.tray_icon.activated.connect(self._on_tray_activated)

        # Don't show tray by default - user preference
        # self.tray_icon.show()

    def show_and_raise(self):
        self.show()
        self.raise_()
        self.activateWindow()

    def _on_tray_activated(self, reason):
        if reason == QSystemTrayIcon.ActivationReason.DoubleClick:
            self.show_and_raise()

    def _load_projects(self):
        """Load projects from config and populate sidebar."""
        projects = self.config_store.load_projects()
        for p in projects:
            self._projects[p.id] = p
            self.sidebar.add_item(p.id, p.name)
        self._update_status_bar()

    def _setup_auto_compile(self):
        """Start watchers for projects with auto_compile enabled."""
        for p in self._projects.values():
            if p.auto_compile:
                self._start_watcher(p.id, p.path)

    def _start_watcher(self, project_id: str, project_path: str):
        settings = self.config_store.load_settings()
        self.file_watcher.watch(
            project_id, project_path,
            debounce_ms=settings.auto_compile_debounce_ms,
            on_compile=self._on_auto_compile_trigger,
        )

    def _on_auto_compile_trigger(self, project_id: str):
        """Called by file watcher when Java files change."""
        project = self._projects.get(project_id)
        if not project:
            return

        self._append_log(project_id, "build", f"⚡ 检测到文件变更，开始自动编译...")

        def do_compile():
            try:
                result = self.maven_builder.compile(
                    project.path, project_id,
                    on_log=self._on_maven_log,
                )
                if result.success:
                    self._append_log(
                        project_id, "build",
                        f"✅ 自动编译成功 ({result.duration_ms / 1000:.1f}s)",
                    )
                else:
                    self._append_log(
                        project_id, "error",
                        f"❌ 自动编译失败 ({result.duration_ms / 1000:.1f}s)",
                    )
            except Exception as e:
                self._append_log(project_id, "error", f"自动编译错误: {e}")

        # Use a timer to avoid blocking UI
        QTimer.singleShot(0, do_compile)

    def _on_maven_log(self, project_id: str, level: str, message: str):
        self._append_log(project_id, level, message)

    def _on_project_selected(self, project_id: str):
        self._selected_project_id = project_id
        self.sidebar.select_item(project_id)

        # Show log output, hide welcome
        self.log_output.setVisible(True)
        self.welcome.setVisible(False)

        # Load existing logs
        self.log_output.clear()
        entries = self.log_buffer.get(project_id)
        for entry in entries:
            self.log_output.append_log(entry.timestamp, entry.level, entry.message)

        # Update toolbar
        project = self._projects.get(project_id)
        if project:
            status = self.process_manager.status(project_id)
            proc = self.process_manager.get_process(project_id)
            port = proc.port if proc else None
            self.log_toolbar.set_project_info(project.name, status, port)

    def _on_add_project(self):
        dialog = AddProjectDialog(self)
        if dialog.exec():
            data = dialog.result_data
            if data:
                config = ProjectConfig(**data)
                self.config_store.save_project(config)
                self._projects[config.id] = config
                self.sidebar.add_item(config.id, config.name)
                self.sidebar.select_item(config.id)

                if config.auto_compile:
                    self._start_watcher(config.id, config.path)

                self._on_project_selected(config.id)
                self._update_status_bar()

    def _on_open_settings(self):
        settings = self.config_store.load_settings()
        dialog = SettingsDialog(self, {
            "maven_path": settings.maven_path,
            "auto_compile_debounce_ms": settings.auto_compile_debounce_ms,
        })
        if dialog.exec():
            data = dialog.result_data
            if data:
                new_settings = AppSettings(
                    theme=settings.theme,
                    maven_path=data["maven_path"],
                    auto_compile_debounce_ms=data["auto_compile_debounce_ms"],
                )
                self.config_store.save_settings(new_settings)
                self.maven_builder.update_maven_path(
                    data["maven_path"] if data["maven_path"] else None
                )

    def _on_start(self):
        project_id = self._selected_project_id
        if not project_id:
            return
        project = self._projects.get(project_id)
        if not project:
            return

        # Find jar
        jar_path = self._find_jar(project.path)
        if not jar_path:
            QMessageBox.warning(
                self, "启动失败",
                f"在 {project.path}/target/ 下未找到可执行 jar。\n请先执行打包。"
            )
            return

        try:
            # Update status
            self.sidebar.update_item_status(project_id, ProcessStatus.STARTING)
            self.log_toolbar.set_project_info(project.name, ProcessStatus.STARTING)

            self.process_manager.start(
                project_id, project.name, jar_path,
                project.jdk_home, project.vm_args,
                on_log=self._on_process_log,
                on_status=self._on_process_status,
            )
        except Exception as e:
            self._append_log(project_id, "error", f"启动失败: {e}")
            self.sidebar.update_item_status(project_id, ProcessStatus.ERROR)
            self.log_toolbar.set_project_info(project.name, ProcessStatus.ERROR)

    def _on_stop(self):
        project_id = self._selected_project_id
        if not project_id:
            return

        try:
            self.process_manager.stop(project_id)
            self.sidebar.update_item_status(project_id, ProcessStatus.STOPPED)
            if project_id in self._projects:
                p = self._projects[project_id]
                self.log_toolbar.set_project_info(p.name, ProcessStatus.STOPPED)
            self._append_log(project_id, "info", "进程已停止")
            self._update_status_bar()
        except Exception as e:
            self._append_log(project_id, "error", f"停止失败: {e}")

    def _on_restart(self):
        self._on_stop()
        QTimer.singleShot(1000, self._on_start)

    def _on_compile(self):
        project_id = self._selected_project_id
        if not project_id:
            return
        project = self._projects.get(project_id)
        if not project:
            return

        self._append_log(project_id, "build", "▶ 开始编译...")

        def do_compile():
            try:
                result = self.maven_builder.compile(
                    project.path, project_id,
                    on_log=self._on_maven_log,
                )
                if result.success:
                    self._append_log(
                        project_id, "build",
                        f"✅ 编译成功 ({result.duration_ms / 1000:.1f}s)",
                    )
                else:
                    self._append_log(
                        project_id, "error",
                        f"❌ 编译失败 ({result.duration_ms / 1000:.1f}s)",
                    )
            except Exception as e:
                self._append_log(project_id, "error", f"编译错误: {e}")

        QTimer.singleShot(0, do_compile)

    def _on_build(self):
        project_id = self._selected_project_id
        if not project_id:
            return
        project = self._projects.get(project_id)
        if not project:
            return

        self._append_log(project_id, "build", "📦 开始打包...")

        def do_build():
            try:
                result = self.maven_builder.build(
                    project.path, project_id,
                    on_log=self._on_maven_log,
                )
                if result.success:
                    self._append_log(
                        project_id, "build",
                        f"✅ 打包成功 ({result.duration_ms / 1000:.1f}s)",
                    )
                else:
                    self._append_log(
                        project_id, "error",
                        f"❌ 打包失败 ({result.duration_ms / 1000:.1f}s)",
                    )
            except Exception as e:
                self._append_log(project_id, "error", f"打包错误: {e}")

        QTimer.singleShot(0, do_build)

    def _on_clear_logs(self):
        project_id = self._selected_project_id
        if project_id:
            self.log_buffer.clear(project_id)
            self.log_output.clear()

    def _on_process_log(self, project_id: str, level: str, message: str):
        self._append_log(project_id, level, message)

    def _on_process_status(self, project_id: str, status: str):
        """Callback from ProcessManager on status change."""
        try:
            ps = ProcessStatus(status)
        except ValueError:
            return

        self.sidebar.update_item_status(project_id, ps)

        # Update toolbar if this is the selected project
        if project_id == self._selected_project_id:
            project = self._projects.get(project_id)
            if project:
                proc = self.process_manager.get_process(project_id)
                port = proc.port if proc else None
                self.log_toolbar.set_project_info(project.name, ps, port)

        self._update_status_bar()

    def _append_log(self, project_id: str, level: str, message: str):
        now = datetime.now().strftime("%H:%M:%S.%f")[:12]
        entry = LogEntry(timestamp=now, level=level, message=message)
        self.log_buffer.append(project_id, entry)

        # Update log output if this is the selected project
        if project_id == self._selected_project_id:
            self.log_output.append_log(now, level, message)

    def _update_status_bar(self):
        total = len(self._projects)
        running = sum(
            1 for pid in self._projects
            if self.process_manager.status(pid) == ProcessStatus.RUNNING
        )
        self.status_bar.update_status(running, total)

    def _find_jar(self, project_path: str) -> str | None:
        """Find the executable jar in target/ directory."""
        target = os.path.join(project_path, "target")
        if not os.path.isdir(target):
            return None

        jars = [
            f for f in os.listdir(target)
            if f.endswith(".jar")
            and "sources" not in f.lower()
            and "javadoc" not in f.lower()
            and "original" not in f.lower()
        ]

        if not jars:
            return None

        # Return the most recently modified
        jars.sort(key=lambda f: os.path.getmtime(os.path.join(target, f)), reverse=True)
        return os.path.join(target, jars[0])

    def _quit_app(self):
        self.process_manager.shutdown_all()
        QApplication.instance().quit()

    def closeEvent(self, event):
        """Minimize to tray instead of closing."""
        event.ignore()
        self.hide()
        self.tray_icon.show()
        self.tray_icon.showMessage(
            "Launcher4j",
            "应用已最小化到系统托盘",
            QSystemTrayIcon.MessageIcon.Information,
            2000,
        )
