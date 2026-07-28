"""Dialog windows for Launcher4j."""

import os
import uuid
from datetime import datetime

from PySide6.QtCore import Qt
from PySide6.QtWidgets import (
    QDialog, QVBoxLayout, QHBoxLayout, QLabel, QPushButton,
    QLineEdit, QCheckBox, QWidget, QFileDialog,
)


class AddProjectDialog(QDialog):
    """Dialog to add a new Maven project."""

    def __init__(self, parent=None):
        super().__init__(parent)
        self.setWindowTitle("添加项目")
        self.setFixedSize(500, 420)
        self.setObjectName("dialog")
        self.setModal(True)

        # Result data
        self.result_data = None

        layout = QVBoxLayout(self)
        layout.setContentsMargins(0, 0, 0, 0)
        layout.setSpacing(0)

        # Title
        title = QLabel("添加项目")
        title.setObjectName("dialog-title")
        layout.addWidget(title)

        # Content
        content = QWidget()
        content.setObjectName("dialog-content")
        content_layout = QVBoxLayout(content)
        content_layout.setSpacing(14)

        # Path
        path_label = QLabel("项目目录 *")
        path_label.setObjectName("label")
        content_layout.addWidget(path_label)

        path_row = QHBoxLayout()
        self.path_input = QLineEdit()
        self.path_input.setPlaceholderText("选择 Maven 项目根目录...")
        self.path_input.setReadOnly(True)
        path_row.addWidget(self.path_input)

        browse_btn = QPushButton("📂 浏览")
        browse_btn.setObjectName("btn-secondary")
        browse_btn.clicked.connect(self._browse)
        path_row.addWidget(browse_btn)
        content_layout.addLayout(path_row)

        # Name
        name_label = QLabel("项目名称")
        name_label.setObjectName("label")
        content_layout.addWidget(name_label)
        self.name_input = QLineEdit()
        self.name_input.setPlaceholderText("my-spring-app")
        content_layout.addWidget(self.name_input)

        # JDK
        jdk_label = QLabel("JDK 路径")
        jdk_label.setObjectName("label")
        content_layout.addWidget(jdk_label)
        self.jdk_input = QLineEdit("java")
        self.jdk_input.setPlaceholderText("java 或 JDK 绝对路径")
        content_layout.addWidget(self.jdk_input)
        help_label = QLabel("默认使用系统 PATH 中的 java")
        help_label.setObjectName("label-help")
        content_layout.addWidget(help_label)

        # VM Args
        vm_label = QLabel("VM 参数")
        vm_label.setObjectName("label")
        content_layout.addWidget(vm_label)
        self.vm_input = QLineEdit()
        self.vm_input.setPlaceholderText("-Xmx512m -Dspring.profiles.active=dev")
        content_layout.addWidget(self.vm_input)

        # Auto compile
        self.auto_compile_cb = QCheckBox("启用自动编译（文件变更时自动编译）")
        self.auto_compile_cb.setChecked(True)
        content_layout.addWidget(self.auto_compile_cb)

        layout.addWidget(content, 1)

        # Footer
        footer = QWidget()
        footer.setObjectName("dialog-footer")
        footer_layout = QHBoxLayout(footer)
        footer_layout.setContentsMargins(0, 0, 0, 0)

        cancel_btn = QPushButton("取消")
        cancel_btn.setObjectName("btn-secondary")
        cancel_btn.clicked.connect(self.reject)
        footer_layout.addWidget(cancel_btn)

        footer_layout.addStretch()

        self.add_btn = QPushButton("添加")
        self.add_btn.setObjectName("btn-primary")
        self.add_btn.clicked.connect(self._accept)
        footer_layout.addWidget(self.add_btn)

        layout.addWidget(footer)

    def _browse(self):
        path = QFileDialog.getExistingDirectory(
            self, "选择 Maven 项目目录"
        )
        if path:
            self.path_input.setText(path)
            if not self.name_input.text():
                name = os.path.basename(path)
                self.name_input.setText(name)

    def _accept(self):
        path = self.path_input.text().strip()
        name = self.name_input.text().strip()

        if not path:
            self.path_input.setFocus()
            return
        if not name:
            name = os.path.basename(path)

        self.result_data = {
            "id": str(uuid.uuid4()),
            "name": name,
            "path": path,
            "jdk_home": self.jdk_input.text().strip() or "java",
            "vm_args": self.vm_input.text().strip(),
            "auto_compile": self.auto_compile_cb.isChecked(),
            "added_at": datetime.now().isoformat(),
        }
        self.accept()


class SettingsDialog(QDialog):
    """Application settings dialog."""

    def __init__(self, parent=None, settings: dict = None):
        super().__init__(parent)
        self.setWindowTitle("设置")
        self.setFixedSize(420, 280)
        self.setObjectName("dialog")
        self.setModal(True)

        self.result_data = None

        layout = QVBoxLayout(self)
        layout.setContentsMargins(0, 0, 0, 0)
        layout.setSpacing(0)

        title = QLabel("设置")
        title.setObjectName("dialog-title")
        layout.addWidget(title)

        content = QWidget()
        content.setObjectName("dialog-content")
        content_layout = QVBoxLayout(content)
        content_layout.setSpacing(14)

        # Maven path
        mvn_label = QLabel("Maven 路径（可选）")
        mvn_label.setObjectName("label")
        content_layout.addWidget(mvn_label)
        mvn_row = QHBoxLayout()
        self.mvn_input = QLineEdit()
        self.mvn_input.setPlaceholderText("留空则使用系统 PATH 中的 mvn")
        if settings:
            self.mvn_input.setText(settings.get("maven_path", ""))
        mvn_row.addWidget(self.mvn_input)

        mvn_browse = QPushButton("📂")
        mvn_browse.setObjectName("btn-secondary")
        mvn_browse.setFixedWidth(36)
        mvn_browse.clicked.connect(lambda: self._browse_mvn())
        mvn_row.addWidget(mvn_browse)
        content_layout.addLayout(mvn_row)

        # Debounce
        debounce_label = QLabel("自动编译去抖时间 (ms)")
        debounce_label.setObjectName("label")
        content_layout.addWidget(debounce_label)
        self.debounce_input = QLineEdit()
        self.debounce_input.setPlaceholderText("500")
        if settings:
            self.debounce_input.setText(str(settings.get("auto_compile_debounce_ms", 500)))
        content_layout.addWidget(self.debounce_input)
        help_label = QLabel("文件变更后等待此时间再触发编译，避免频繁编译")
        help_label.setObjectName("label-help")
        content_layout.addWidget(help_label)

        layout.addWidget(content, 1)

        footer = QWidget()
        footer.setObjectName("dialog-footer")
        footer_layout = QHBoxLayout(footer)

        cancel_btn = QPushButton("取消")
        cancel_btn.setObjectName("btn-secondary")
        cancel_btn.clicked.connect(self.reject)
        footer_layout.addWidget(cancel_btn)
        footer_layout.addStretch()

        save_btn = QPushButton("保存设置")
        save_btn.setObjectName("btn-primary")
        save_btn.clicked.connect(self._accept)
        footer_layout.addWidget(save_btn)

        layout.addWidget(footer)

    def _browse_mvn(self):
        path, _ = QFileDialog.getOpenFileName(
            self, "选择 Maven 可执行文件", "",
            "Maven (mvn mvn.cmd);;所有文件 (*)"
        )
        if path:
            self.mvn_input.setText(path)

    def _accept(self):
        try:
            debounce = int(self.debounce_input.text().strip())
        except ValueError:
            debounce = 500

        self.result_data = {
            "maven_path": self.mvn_input.text().strip(),
            "auto_compile_debounce_ms": debounce,
        }
        self.accept()
