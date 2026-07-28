# Launcher4j ⚡

> 轻量级 Java/Spring Boot 项目启动器 — 替代臃肿的 IDE 运行环境

![Python](https://img.shields.io/badge/python-3.10+-blue?logo=python)
![PySide6](https://img.shields.io/badge/PySide6-6.6+-green?logo=qt)
![License](https://img.shields.io/badge/license-MIT-green)

## 简介

Launcher4j 是一个轻量级的桌面应用，专门用于管理 Java/Spring Boot 项目的启停、编译和打包。
告别 IntelliJ IDEA 的高内存占用，用极简的方式运行你的项目。

### 特性

- **🖥️ 图形界面** — 基于 PySide6 (Qt) 的精美暗色主题界面
- **⌨️ CLI 命令行** — 所有操作都可通过命令行完成
- **⚡ 自动编译** — 监听 Java 文件变更，自动触发 `mvn compile`
- **📦 项目管理** — 多项目支持，独立配置 JDK 和 VM 参数
- **🔄 进程管理** — 一键启动/停止/重启 Spring Boot 应用
- **📋 实时日志** — 彩色终端风格日志输出
- **🔔 系统托盘** — 最小化到托盘，后台运行

## 安装

### 依赖

```bash
pip install PySide6 watchdog
```

### 启动

```bash
# GUI 模式
python main.py

# CLI 模式
python main.py list
python main.py compile
python main.py build
python main.py run my-app
```

### 打包为 exe

```bash
pip install pyinstaller
pyinstaller --onefile --windowed --add-data "launcher4j/resources/style.qss;launcher4j/resources" main.py
```

## CLI 命令

| 命令 | 说明 | 示例 |
|------|------|------|
| `run <project>` | 启动项目 | `launcher4j run my-app` |
| `stop <project>` | 停止项目 | `launcher4j stop my-app` |
| `restart <project>` | 重启项目 | `launcher4j restart my-app` |
| `compile [path]` | 编译（默认当前目录） | `launcher4j compile` |
| `build [path]` | 打包 | `launcher4j build ./my-app` |
| `status [project]` | 查看状态 | `launcher4j status my-app` |
| `list` | 列出所有项目 | `launcher4j list` |

## 项目结构

```
launcher4j/
├── main.py                    # 入口
├── requirements.txt           # 依赖
├── launcher4j/
│   ├── app.py                 # GUI 启动
│   ├── cli.py                 # CLI 入口
│   ├── config/store.py        # JSON 配置持久化
│   ├── engine/
│   │   ├── process_manager.py # Java 进程管理
│   │   ├── maven_builder.py   # Maven 编译/打包
│   │   ├── file_watcher.py    # 文件变更监听
│   │   └── log_buffer.py      # 日志缓冲区
│   ├── ui/
│   │   ├── main_window.py     # 主窗口
│   │   ├── widgets.py         # UI 组件(侧边栏、日志、状态栏)
│   │   ├── dialogs.py         # 对话框(添加项目、设置)
│   │   └── theme.py           # 主题管理
│   └── resources/style.qss    # Qt 暗色主题样式
└── setup.py
```

## 许可证

[MIT](LICENSE)
