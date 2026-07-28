"""Theme management for the application."""

import os
from PySide6.QtGui import QPalette, QColor
from PySide6.QtWidgets import QApplication


def load_style(app: QApplication) -> str:
    """Load and apply the QSS stylesheet.

    Returns the stylesheet string for further manipulation if needed.
    """
    qss_path = os.path.join(os.path.dirname(__file__), "..", "resources", "style.qss")
    qss_path = os.path.normpath(qss_path)

    try:
        with open(qss_path, "r", encoding="utf-8") as f:
            style = f.read()
    except FileNotFoundError:
        style = ""

    app.setStyleSheet(style)
    return style


def apply_dark_palette(app: QApplication):
    """Set the dark color palette for Qt widgets that don't use QSS."""
    palette = QPalette()

    palette.setColor(QPalette.Window, QColor("#1a1d28"))
    palette.setColor(QPalette.WindowText, QColor("#e2e6f0"))
    palette.setColor(QPalette.Base, QColor("#2e3345"))
    palette.setColor(QPalette.AlternateBase, QColor("#252937"))
    palette.setColor(QPalette.ToolTipBase, QColor("#252937"))
    palette.setColor(QPalette.ToolTipText, QColor("#e2e6f0"))
    palette.setColor(QPalette.Text, QColor("#e2e6f0"))
    palette.setColor(QPalette.Button, QColor("#3d4355"))
    palette.setColor(QPalette.ButtonText, QColor("#e2e6f0"))
    palette.setColor(QPalette.BrightText, QColor("#ef4444"))
    palette.setColor(QPalette.Link, QColor("#2391f7"))
    palette.setColor(QPalette.Highlight, QColor("#2391f7"))
    palette.setColor(QPalette.HighlightedText, QColor("#ffffff"))

    app.setPalette(palette)
