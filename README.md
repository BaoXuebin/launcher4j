# Launcher4j ⚡

> 轻量级 Java/Spring Boot 项目启动器 — 替代臃肿的 IDE 运行环境

![Go](https://img.shields.io/badge/go-1.23+-blue?logo=go)
![License](https://img.shields.io/badge/license-MIT-green)

## 简介

Launcher4j 是一个轻量级命令行工具，专门用于管理 Java/Spring Boot 项目的启停、编译和打包。
告别 IntelliJ IDEA 的高内存占用，用极简的方式运行你的项目。

### 特性

- **⌨️ CLI 命令行** — 所有操作都可通过命令行完成，脚本友好
- **⚡ 自动编译** — 监听 Java 文件变更，自动触发 `mvn compile`
- **📦 项目管理** — 多项目支持，独立配置 JDK 和 VM 参数
- **🔄 进程管理** — 一键启动/停止/重启 Spring Boot 应用
- **📋 实时日志** — 彩色终端风格日志输出
- **🔌 零依赖** — 单文件二进制，无需 Python/JRE 以外的任何运行时

## 安装

### 方式一：下载预编译二进制

从 [Releases](../../releases) 页面下载对应平台的二进制文件，加入 PATH 即可。

### 方式二：从源码编译

```bash
git clone https://github.com/baoxuebin/launcher4j.git
cd launcher4j
go build -o launcher4j ./cmd/launcher4j
```

### 方式三：Go 安装

```bash
go install github.com/baoxuebin/launcher4j/cmd/launcher4j@latest
```

## CLI 命令

| 命令 | 说明 | 示例 |
|------|------|------|
| `run <project>` | 启动项目（按 Ctrl+C 停止） | `launcher4j run my-app` |
| `stop <project>` | 停止项目 | `launcher4j stop my-app` |
| `restart <project>` | 重启项目 | `launcher4j restart my-app` |
| `compile [project]` | 编译（默认当前目录） | `launcher4j compile` |
| `build [project]` | 打包 | `launcher4j build ./my-app` |
| `clean [project]` | 清理 | `launcher4j clean` |
| `status [project]` | 查看状态（可查看单个或全部） | `launcher4j status` |
| `list` | 列出所有已配置项目 | `launcher4j list` |
| `add <path>` | 添加新项目 | `launcher4j add /path/to/project` |
| `remove <project>` | 删除项目 | `launcher4j remove my-app` |
| `version` | 版本信息 | `launcher4j version` |

### add 命令选项

```
launcher4j add <path> \
  --name=<name>           项目显示名称（默认使用目录名）
  --jdk=<path>            JDK 路径（默认使用 PATH 中的 java）
  --vm-args=<args>        JVM 参数（如 "-Xmx512m -Dspring.profiles.active=dev"）
  --env=<vars>            环境变量（KEY=VALUE|KEY=VALUE...）
  --no-auto-compile       禁用自动编译
```

### 使用示例

```bash
# 添加一个 Spring Boot 项目
launcher4j add /workspace/my-app --name=my-app --vm-args="-Xmx512m"

# 编译
launcher4j compile my-app

# 打包
launcher4j build my-app

# 启动
launcher4j run my-app

# 查看状态
launcher4j status

# 停止
launcher4j stop my-app
```

## 配置

配置文件保存在系统用户目录：

- **Windows**: `%APPDATA%/launcher4j/launcher4j.json`
- **Linux/macOS**: `~/.config/launcher4j/launcher4j.json`

## 项目结构

```
launcher4j/
├── cmd/launcher4j/
│   └── main.go                 # 入口：命令分发 + 参数解析
├── internal/
│   ├── app/
│   │   └── cli.go              # CLI 命令处理
│   ├── config/
│   │   └── store.go            # JSON 配置持久化
│   └── engine/
│       ├── builder.go          # Maven 构建器
│       ├── filewatcher.go      # 文件变更监听
│       ├── logbuffer.go        # 日志环形缓冲区
│       └── process.go          # Java 进程管理
├── go.mod / go.sum
└── launcher4j.exe              # 编译产物
```

## 许可证

[MIT](LICENSE)
