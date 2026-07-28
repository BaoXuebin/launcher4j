#!/usr/bin/env python3
"""Launcher4j - Lightweight Java/Spring Boot Project Launcher.

Usage:
  launcher4j                   # Launch GUI
  launcher4j run <project>     # Start project
  launcher4j stop <project>    # Stop project
  launcher4j compile [path]    # Compile project
  launcher4j build [path]      # Build project
  launcher4j status [project]  # Show status
  launcher4j list              # List projects
"""

import sys
import os

# Ensure launcher4j package is importable
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from launcher4j.cli import main as cli_main


def main():
    # Try CLI first; if it returns False, launch GUI
    if not cli_main():
        from launcher4j.app import run_gui
        run_gui()


if __name__ == "__main__":
    main()
