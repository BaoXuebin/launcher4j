// Launcher4j - Lightweight Java/Spring Boot Project Launcher.
//
// Usage:
//
//	launcher4j                        # List projects (default)
//	launcher4j run <project>          # Start project
//	launcher4j stop <project>         # Stop project
//	launcher4j restart <project>      # Restart project
//	launcher4j compile [project]      # Compile project (default: current dir)
//	launcher4j build [project]        # Build project
//	launcher4j clean [project]        # Clean project
//	launcher4j status [project]       # Show project status and config
//	launcher4j list                   # List all configured projects
//	launcher4j add <path> [options]   # Add a new project
//	launcher4j config <project> [opt] # Update project configuration
//	launcher4j remove <project>       # Remove a project
//	launcher4j version                # Show version
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/baoxuebin/launcher4j/internal/app"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "version", "--version", "-V":
		fmt.Printf("Launcher4j v%s\n", version)
		return
	case "help", "--help", "-h":
		printUsage()
		return
	}

	cli := app.NewCLI()

	switch cmd {
	case "run":
		if len(os.Args) < 3 {
			fmt.Println("Usage: launcher4j run <project>")
			os.Exit(1)
		}
		if err := cli.CmdRun(os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}

	case "stop":
		if len(os.Args) < 3 {
			fmt.Println("Usage: launcher4j stop <project>")
			os.Exit(1)
		}
		if err := cli.CmdStop(os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}

	case "restart":
		if len(os.Args) < 3 {
			fmt.Println("Usage: launcher4j restart <project>")
			os.Exit(1)
		}
		if err := cli.CmdRestart(os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}

	case "compile":
		project := ""
		if len(os.Args) >= 3 {
			project = os.Args[2]
		}
		if err := cli.CmdCompile(project); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}

	case "build":
		project := ""
		if len(os.Args) >= 3 {
			project = os.Args[2]
		}
		if err := cli.CmdBuild(project); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}

	case "clean":
		project := ""
		if len(os.Args) >= 3 {
			project = os.Args[2]
		}
		if err := cli.CmdClean(project); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}

	case "status":
		project := ""
		if len(os.Args) >= 3 {
			project = os.Args[2]
		}
		if err := cli.CmdStatus(project); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}

	case "list":
		if err := cli.CmdList(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}

	case "add":
		if len(os.Args) < 3 {
			fmt.Println("Usage: launcher4j add <path> [--name=<name>] [--jdk=<jdk>] [--vm-args=<args>] [--env=<vars>] [--no-auto-compile]")
			os.Exit(1)
		}
		path := os.Args[2]
		name := ""
		jdkHome := "java"
		vmArgs := ""
		envVars := ""
		autoCompile := true
		for _, arg := range os.Args[3:] {
			switch {
			case strings.HasPrefix(arg, "--name="):
				name = arg[len("--name="):]
			case strings.HasPrefix(arg, "--jdk="):
				jdkHome = arg[len("--jdk="):]
			case strings.HasPrefix(arg, "--vm-args="):
				vmArgs = arg[len("--vm-args="):]
			case strings.HasPrefix(arg, "--env="):
				envVars = arg[len("--env="):]
			case arg == "--no-auto-compile":
				autoCompile = false
			}
		}
		if err := cli.CmdAdd(path, name, jdkHome, vmArgs, envVars, autoCompile); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}

	case "config":
		if len(os.Args) < 3 {
			fmt.Println("Usage: launcher4j config <project> [--name=<name>] [--jdk=<jdk>] [--add-vm=<arg>]... [--remove-vm=<prefix>]... [--set-env=<KEY=VALUE>]... [--unset-env=<KEY>]... [--auto-compile] [--no-auto-compile]")
			os.Exit(1)
		}
		project := os.Args[2]
		name := ""
		jdkHome := ""
		var addVM, removeVM, setEnv, unsetEnv []string
		var autoCompile *bool
		for _, arg := range os.Args[3:] {
			switch {
			case strings.HasPrefix(arg, "--name="):
				name = arg[len("--name="):]
			case strings.HasPrefix(arg, "--jdk="):
				jdkHome = arg[len("--jdk="):]
			case strings.HasPrefix(arg, "--add-vm="):
				addVM = append(addVM, arg[len("--add-vm="):])
			case strings.HasPrefix(arg, "--remove-vm="):
				removeVM = append(removeVM, arg[len("--remove-vm="):])
			case strings.HasPrefix(arg, "--set-env="):
				setEnv = append(setEnv, arg[len("--set-env="):])
			case strings.HasPrefix(arg, "--unset-env="):
				unsetEnv = append(unsetEnv, arg[len("--unset-env="):])
			case arg == "--no-auto-compile":
				v := false
				autoCompile = &v
			case arg == "--auto-compile":
				v := true
				autoCompile = &v
			}
		}
		if err := cli.CmdConfig(project, name, jdkHome, addVM, removeVM, setEnv, unsetEnv, autoCompile); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}

	case "remove":
		if len(os.Args) < 3 {
			fmt.Println("Usage: launcher4j remove <project>")
			os.Exit(1)
		}
		if err := cli.CmdRemove(os.Args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Launcher4j - Lightweight Java/Spring Boot Project Launcher

Usage:
  launcher4j                         List all configured projects (alias: list)
  launcher4j run <project>           Start a project
  launcher4j stop <project>          Stop a project
  launcher4j restart <project>       Restart a project
  launcher4j compile [project]       Compile a Maven project
  launcher4j build [project]         Build (package) a project
  launcher4j clean [project]         Clean a project
  launcher4j status [project]        Show project status and configuration
  launcher4j list                    List all configured projects
  launcher4j add <path> [options]    Add a new project
  launcher4j config <project> [opt]  Update project configuration
  launcher4j remove <project>        Remove a project
  launcher4j version                 Show version information

Options for 'add':
  --name=<name>           Project display name
  --jdk=<path>            JDK path (default: "java" from PATH)
  --vm-args=<args>        JVM arguments (space-separated, e.g. "-Xmx512m -Dkey=val")
  --env=<vars>            Environment variables (KEY=VALUE|KEY=VALUE...)
  --no-auto-compile       Disable auto-compile on file change

Options for 'config':
  --name=<name>           New display name
  --jdk=<path>            JDK path
  --add-vm=<arg>          Add a VM argument (can be used multiple times)
  --remove-vm=<prefix>    Remove VM arguments matching prefix (can be used multiple times)
  --set-env=<KEY=VAL>     Set an environment variable (can be used multiple times)
  --unset-env=<KEY>       Remove an environment variable (can be used multiple times)
  --auto-compile          Enable auto-compile
  --no-auto-compile       Disable auto-compile

Examples:
  launcher4j run my-app
  launcher4j build ./projects/my-app
  launcher4j add /path/to/project --name=my-app --vm-args="-Xmx512m"
  launcher4j config my-app --add-vm=-Xmx1g --set-env=SPRING_PROFILES_ACTIVE=prod
  launcher4j config my-app --remove-vm=-Xmx --unset-env=SPRING_PROFILES_ACTIVE
  launcher4j config my-app --auto-compile
  launcher4j status my-app`)
}
