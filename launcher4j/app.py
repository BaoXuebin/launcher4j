"""Application entry point for GUI mode."""

import sys

from PySide6.QtCore import Qt
from PySide6.QtWidgets import QApplication

from launcher4j.ui.theme import load_style, apply_dark_palette
from launcher4j.ui.main_window import MainWindow


def run_gui():
    """Launch the GUI application."""
    # High DPI support
    QApplication.setHighDpiScaleFactorRoundingPolicy(
        Qt.HighDpiScaleFactorRoundingPolicy.PassThrough
    )

    app = QApplication(sys.argv)
    app.setApplicationName("Launcher4j")
    app.setOrganizationName("Launcher4j")
    app.setQuitOnLastWindowClosed(False)

    # Apply dark theme
    apply_dark_palette(app)
    load_style(app)

    # Create and show main window
    window = MainWindow()
    window.show()

    sys.exit(app.exec())
