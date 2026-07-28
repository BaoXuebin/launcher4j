"""Setup script for Launcher4j."""

from setuptools import setup, find_packages

setup(
    name="launcher4j",
    version="0.1.0",
    description="Lightweight Java/Spring Boot Project Launcher",
    packages=find_packages(),
    include_package_data=True,
    package_data={
        "launcher4j": ["resources/*.qss"],
    },
    entry_points={
        "console_scripts": [
            "launcher4j=main:main",
        ],
    },
    python_requires=">=3.10",
    install_requires=[
        "PySide6>=6.6",
        "watchdog>=4.0",
    ],
)
