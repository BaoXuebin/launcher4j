"""Command-line interface for Launcher4j."""

import argparse
import os
import sys
import time

from launcher4j import __version__
from launcher4j.config.store import ConfigStore, ProjectConfig
from launcher4j.engine.process_manager import ProcessManager, ProcessStatus
from launcher4j.engine.maven_builder import MavenBuilder


def create_parser() -> argparse.ArgumentParser:
    """Create the CLI argument parser."""
    parser = argparse.ArgumentParser(
        prog="launcher4j",
        description="Lightweight Java/Spring Boot Project Launcher",
    )
    parser.add_argument(
        "--version", "-V",
        action="version",
        version=f"Launcher4j v{__version__}",
    )

    sub = parser.add_subparsers(dest="command", help="命令")

    # run
    run_p = sub.add_parser("run", help="启动项目")
    run_p.add_argument("project", help="项目 ID、名称或路径")

    # stop
    stop_p = sub.add_parser("stop", help="停止项目")
    stop_p.add_argument("project", help="项目 ID、名称或路径")

    # restart
    restart_p = sub.add_parser("restart", help="重启项目")
    restart_p.add_argument("project", help="项目 ID、名称或路径")

    # compile
    compile_p = sub.add_parser("compile", help="编译项目")
    compile_p.add_argument("project", nargs="?", help="项目 ID、名称或路径（默认当前目录）")

    # build
    build_p = sub.add_parser("build", help="打包项目")
    build_p.add_argument("project", nargs="?", help="项目 ID、名称或路径（默认当前目录）")

    # status
    status_p = sub.add_parser("status", help="查看项目状态")
    status_p.add_argument("project", nargs="?", help="项目 ID、名称或路径")

    # list
    sub.add_parser("list", help="列出所有已配置的项目")

    return parser


def _find_project(
    store: ConfigStore, identifier: str,
) -> ProjectConfig | None:
    """Find a project by ID, name, or path."""
    projects = store.load_projects()

    # By ID
    for p in projects:
        if p.id == identifier:
            return p

    # By name
    for p in projects:
        if p.name == identifier:
            return p

    # By path
    norm = identifier.replace("\\", "/")
    for p in projects:
        if p.path.replace("\\", "/") == norm:
            return p

    # As directory path (not configured)
    pom = os.path.join(identifier, "pom.xml")
    if os.path.exists(pom):
        name = os.path.basename(identifier)
        import uuid
        from datetime import datetime
        return ProjectConfig(
            id=str(uuid.uuid4()),
            name=name,
            path=identifier,
            added_at=datetime.now().isoformat(),
        )

    return None


def _find_jar(project_path: str) -> str | None:
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

    jars.sort(key=lambda f: os.path.getmtime(os.path.join(target, f)), reverse=True)
    return os.path.join(target, jars[0])


def cmd_run(args, store: ConfigStore, pm: ProcessManager):
    """Start a project and wait for it to exit."""
    project = _find_project(store, args.project)
    if not project:
        print(f"❌ Project '{args.project}' not found")
        sys.exit(1)

    jar = _find_jar(project.path)
    if not jar:
        print(f"❌ No executable jar found in {project.path}/target/")
        print("   Run 'launcher4j build' first.")
        sys.exit(1)

    def on_log(pid: str, level: str, msg: str):
        prefix = {"error": "ERR ", "warn": "WARN", "build": "BUILD", "info": "    "}
        print(f"  [{prefix.get(level, '    ')}] {msg}")

    def on_status(pid: str, status: str):
        print(f"  [{status}] {project.name}")

    print(f"▶ Starting '{project.name}'...")
    try:
        pm.start(
            project.id, project.name, jar,
            project.jdk_home, project.vm_args,
            on_log=on_log, on_status=on_status,
        )
    except Exception as e:
        print(f"❌ Failed to start: {e}")
        sys.exit(1)

    proc = pm.get_process(project.id)
    if proc and proc.pid:
        print(f"✓ Started (PID: {proc.pid})")

    # Wait for process to exit
    try:
        while True:
            status = pm.status(project.id)
            if status in (ProcessStatus.STOPPED, ProcessStatus.ERROR):
                break
            time.sleep(1)
    except KeyboardInterrupt:
        print("\n⏹ Stopping...")
        pm.stop(project.id)


def cmd_stop(args, store: ConfigStore, pm: ProcessManager):
    """Stop a project."""
    project = _find_project(store, args.project)
    if not project:
        print(f"❌ Project '{args.project}' not found")
        sys.exit(1)

    pm.stop(project.id)
    print(f"✓ Stopped '{project.name}'")


def cmd_restart(args, store: ConfigStore, pm: ProcessManager):
    """Restart a project."""
    project = _find_project(store, args.project)
    if not project:
        print(f"❌ Project '{args.project}' not found")
        sys.exit(1)

    print(f"↻ Restarting '{project.name}'...")
    pm.stop(project.id)
    time.sleep(1)

    jar = _find_jar(project.path)
    if not jar:
        print(f"❌ No executable jar found")
        sys.exit(1)

    def on_log(pid, level, msg):
        prefix = {"error": "ERR ", "warn": "WARN", "build": "BUILD", "info": "    "}
        print(f"  [{prefix.get(level, '    ')}] {msg}")

    pm.start(project.id, project.name, jar, project.jdk_home, project.vm_args, on_log=on_log)
    proc = pm.get_process(project.id)
    if proc and proc.pid:
        print(f"✓ Restarted (PID: {proc.pid})")


def cmd_compile(args, store: ConfigStore, builder: MavenBuilder):
    """Compile a project."""
    if args.project:
        project = _find_project(store, args.project)
        if not project:
            path = args.project
            if not os.path.exists(os.path.join(path, "pom.xml")):
                print(f"❌ Project '{args.project}' not found")
                sys.exit(1)
        else:
            path = project.path
    else:
        path = os.getcwd()
        if not os.path.exists(os.path.join(path, "pom.xml")):
            print("❌ No pom.xml found in current directory")
            sys.exit(1)

    def on_log(pid, level, msg):
        print(f"  {msg}")

    print(f"▶ Compiling {os.path.basename(path)}...")
    result = builder.compile(path, "cli", on_log=on_log)
    if result.success:
        print(f"✅ Compile SUCCESS ({result.duration_ms / 1000:.1f}s)")
    else:
        print(f"❌ Compile FAILED ({result.duration_ms / 1000:.1f}s)")
        for err in result.errors[:10]:
            print(f"   {err}")
        sys.exit(1)


def cmd_build(args, store: ConfigStore, builder: MavenBuilder):
    """Build (package) a project."""
    if args.project:
        project = _find_project(store, args.project)
        if not project:
            path = args.project
            if not os.path.exists(os.path.join(path, "pom.xml")):
                print(f"❌ Project '{args.project}' not found")
                sys.exit(1)
        else:
            path = project.path
    else:
        path = os.getcwd()
        if not os.path.exists(os.path.join(path, "pom.xml")):
            print("❌ No pom.xml found in current directory")
            sys.exit(1)

    def on_log(pid, level, msg):
        print(f"  {msg}")

    print(f"📦 Building {os.path.basename(path)}...")
    result = builder.build(path, "cli", on_log=on_log)
    if result.success:
        print(f"✅ Build SUCCESS ({result.duration_ms / 1000:.1f}s)")
    else:
        print(f"❌ Build FAILED ({result.duration_ms / 1000:.1f}s)")
        for err in result.errors[:10]:
            print(f"   {err}")
        sys.exit(1)


def cmd_status(args, store: ConfigStore, pm: ProcessManager):
    """Show project status."""
    if args.project:
        project = _find_project(store, args.project)
        if not project:
            print(f"❌ Project '{args.project}' not found")
            sys.exit(1)

        status = pm.status(project.id)
        proc = pm.get_process(project.id)
        print(f"Project: {project.name}")
        print(f"  Path:   {project.path}")
        print(f"  Status: {status.value}")
        if proc and proc.pid:
            print(f"  PID:    {proc.pid}")
        if proc and proc.port:
            print(f"  Port:   {proc.port}")
    else:
        projects = store.load_projects()
        if not projects:
            print("No configured projects.")
            return
        for p in projects:
            status = pm.status(p.id)
            proc = pm.get_process(p.id)
            pid_str = f"PID:{proc.pid}" if proc and proc.pid else ""
            print(f"  {p.name:20s} {status.value:10s} {pid_str}")


def cmd_list(args, store: ConfigStore, pm: ProcessManager):
    """List all configured projects."""
    projects = store.load_projects()
    if not projects:
        print("No configured projects.")
        return

    print(f"{'Name':20s} {'Status':10s} {'Path'}")
    print("-" * 70)
    for p in projects:
        status = pm.status(p.id)
        print(f"{p.name:20s} {status.value:10s} {p.path}")


def main():
    """Run the CLI."""
    parser = create_parser()
    args = parser.parse_args()

    if not args.command:
        # No command -> launch GUI
        return False

    store = ConfigStore()
    settings = store.load_settings()
    builder = MavenBuilder(settings.maven_path if settings.maven_path else None)
    pm = ProcessManager()

    commands = {
        "run": cmd_run,
        "stop": cmd_stop,
        "restart": cmd_restart,
        "compile": cmd_compile,
        "build": cmd_build,
        "status": cmd_status,
        "list": cmd_list,
    }

    cmd_fn = commands.get(args.command)
    if cmd_fn:
        cmd_fn(args, store, pm if args.command in ("run", "stop", "restart", "status", "list") else builder)

    return True  # CLI mode
