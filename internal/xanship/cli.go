package xanship

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
)

var version = "dev"

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		printUsage(stdout)
		return nil
	}
	runner := commandRunner{stdout: stdout, stderr: stderr}
	switch args[0] {
	case "assess":
		return runAssess(ctx, runner, args[1:], stdout)
	case "commands":
		return runCommands(args[1:], stdout)
	case "dry-run":
		return runDryRun(ctx, runner, args[1:], stdout)
	case "load-images":
		return runLoadImages(ctx, runner, args[1:])
	case "copy-volumes":
		return runCopyVolumes(ctx, runner, args[1:])
	case "stop-docker":
		return runStopDocker(ctx, runner, args[1:])
	case "start-apple":
		return runStartApple(ctx, runner, args[1:])
	case "migrate":
		return runMigrate(ctx, runner, args[1:], stdout)
	case "version":
		fmt.Fprintln(stdout, version)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runAssess(ctx context.Context, runner commandRunner, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("assess", flag.ContinueOnError)
	fs.SetOutput(stdout)
	var containers stringList
	fs.Var(&containers, "container", "Docker container name or ID to assess; repeatable")
	composeProject := fs.String("compose-project", "", "Docker Compose project label to assess")
	all := fs.Bool("all", false, "assess all running Docker containers")
	planPath := fs.String("plan", "xanship-plan.json", "path to write the migration plan")
	targetPrefix := fs.String("target-prefix", "xanship-", "prefix for Apple Container names, volumes, and networks")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := RequireExecutable(ctx, runner, "docker"); err != nil {
		return err
	}
	plan, err := BuildPlan(ctx, runner, AssessOptions{
		Containers: containers, ComposeProject: *composeProject, All: *all, TargetPrefix: *targetPrefix,
	})
	if err != nil {
		return err
	}
	if err := SavePlan(*planPath, plan); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote %s: %d container(s), %d volume(s), %d network(s), %d warning(s)\n", *planPath, len(plan.Containers), len(plan.Volumes), len(plan.Networks), len(plan.Warnings))
	return nil
}

func runCommands(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("commands", flag.ContinueOnError)
	fs.SetOutput(stdout)
	planPath := fs.String("plan", "xanship-plan.json", "migration plan path")
	dryRun := fs.Bool("dry-run", false, "print container create/delete commands instead of run commands")
	if err := fs.Parse(args); err != nil {
		return err
	}
	plan, err := LoadPlan(*planPath)
	if err != nil {
		return err
	}
	PrintAppleCommands(stdout, plan, *dryRun)
	return nil
}

func runDryRun(ctx context.Context, runner commandRunner, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("dry-run", flag.ContinueOnError)
	fs.SetOutput(stdout)
	planPath := fs.String("plan", "xanship-plan.json", "migration plan path")
	apply := fs.Bool("apply", false, "actually create and delete Apple containers for validation")
	if err := fs.Parse(args); err != nil {
		return err
	}
	plan, err := LoadPlan(*planPath)
	if err != nil {
		return err
	}
	if !*apply {
		PrintAppleCommands(stdout, plan, true)
		return nil
	}
	if err := RequireExecutable(ctx, runner, "container"); err != nil {
		return err
	}
	return DryRunApple(ctx, runner, plan)
}

func runLoadImages(ctx context.Context, runner commandRunner, args []string) error {
	fs := flag.NewFlagSet("load-images", flag.ContinueOnError)
	planPath := fs.String("plan", "xanship-plan.json", "migration plan path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	plan, err := LoadPlan(*planPath)
	if err != nil {
		return err
	}
	return LoadImages(ctx, runner, plan)
}

func runCopyVolumes(ctx context.Context, runner commandRunner, args []string) error {
	fs := flag.NewFlagSet("copy-volumes", flag.ContinueOnError)
	planPath := fs.String("plan", "xanship-plan.json", "migration plan path")
	image := fs.String("copy-image", "docker.io/library/busybox:latest", "image used to stream volume tar data")
	if err := fs.Parse(args); err != nil {
		return err
	}
	plan, err := LoadPlan(*planPath)
	if err != nil {
		return err
	}
	return CopyVolumes(ctx, runner, plan, *image)
}

func runStopDocker(ctx context.Context, runner commandRunner, args []string) error {
	fs := flag.NewFlagSet("stop-docker", flag.ContinueOnError)
	planPath := fs.String("plan", "xanship-plan.json", "migration plan path")
	timeout := fs.String("timeout", "10", "docker stop timeout seconds")
	if err := fs.Parse(args); err != nil {
		return err
	}
	plan, err := LoadPlan(*planPath)
	if err != nil {
		return err
	}
	return StopDocker(ctx, runner, plan, *timeout)
}

func runStartApple(ctx context.Context, runner commandRunner, args []string) error {
	fs := flag.NewFlagSet("start-apple", flag.ContinueOnError)
	planPath := fs.String("plan", "xanship-plan.json", "migration plan path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	plan, err := LoadPlan(*planPath)
	if err != nil {
		return err
	}
	return StartApple(ctx, runner, plan)
}

func runMigrate(ctx context.Context, runner commandRunner, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(stdout)
	var containers stringList
	fs.Var(&containers, "container", "Docker container name or ID to migrate; repeatable")
	composeProject := fs.String("compose-project", "", "Docker Compose project label to migrate")
	all := fs.Bool("all", false, "migrate all running Docker containers")
	planPath := fs.String("plan", "xanship-plan.json", "migration plan path")
	targetPrefix := fs.String("target-prefix", "xanship-", "prefix for Apple Container names, volumes, and networks")
	copyImage := fs.String("copy-image", "docker.io/library/busybox:latest", "image used to stream volume tar data")
	skipImageLoad := fs.Bool("skip-image-load", false, "do not docker save | container image load images")
	skipDryRun := fs.Bool("skip-dry-run", false, "skip Apple Container create/delete validation")
	skipCopy := fs.Bool("skip-copy", false, "skip named volume data copy")
	skipStop := fs.Bool("skip-stop", false, "do not stop Docker containers before starting Apple containers")
	timeout := fs.String("timeout", "10", "docker stop timeout seconds")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := RequireExecutable(ctx, runner, "docker"); err != nil {
		return err
	}
	if err := RequireExecutable(ctx, runner, "container"); err != nil {
		return err
	}
	plan, err := BuildPlan(ctx, runner, AssessOptions{
		Containers: containers, ComposeProject: *composeProject, All: *all, TargetPrefix: *targetPrefix,
	})
	if err != nil {
		return err
	}
	if err := SavePlan(*planPath, plan); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote %s\n", *planPath)
	if !*skipImageLoad {
		if err := LoadImages(ctx, runner, plan); err != nil {
			return err
		}
	}
	if !*skipDryRun {
		if err := DryRunApple(ctx, runner, plan); err != nil {
			return err
		}
	}
	if !*skipCopy {
		if err := CopyVolumes(ctx, runner, plan, *copyImage); err != nil {
			return err
		}
	}
	if !*skipStop {
		if err := StopDocker(ctx, runner, plan, *timeout); err != nil {
			return err
		}
	}
	return StartApple(ctx, runner, plan)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `Xanship migrates running Docker Desktop containers to Apple Container.

Usage:
  xanship assess [--container NAME ... | --compose-project PROJECT | --all] [--plan xanship-plan.json]
  xanship commands [--plan xanship-plan.json] [--dry-run]
  xanship dry-run [--plan xanship-plan.json] [--apply]
  xanship load-images [--plan xanship-plan.json]
  xanship copy-volumes [--plan xanship-plan.json]
  xanship stop-docker [--plan xanship-plan.json]
  xanship start-apple [--plan xanship-plan.json]
  xanship migrate [--container NAME ... | --compose-project PROJECT | --all]
  xanship version

Migration phases:
  assess -> load-images -> dry-run --apply -> copy-volumes -> stop-docker -> start-apple`)
}
