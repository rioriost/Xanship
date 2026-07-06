package xanship

import (
	"context"
	"encoding/json"
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

type sourceFlags struct {
	dockerHost    *string
	dockerContext *string
}

func addSourceFlags(fs *flag.FlagSet) sourceFlags {
	return sourceFlags{
		dockerHost:    fs.String("docker-host", "", "Docker source host, e.g. ssh://user@linux-host"),
		dockerContext: fs.String("docker-context", "", "Docker source context name"),
	}
}

func (s sourceFlags) dockerArgs() []string {
	var args []string
	if s.dockerContext != nil && *s.dockerContext != "" {
		args = append(args, "--context", *s.dockerContext)
	}
	if s.dockerHost != nil && *s.dockerHost != "" {
		args = append(args, "--host", *s.dockerHost)
	}
	return args
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
	case "preflight":
		return runPreflight(args[1:], stdout)
	case "dry-run":
		return runDryRun(ctx, runner, args[1:], stdout)
	case "load-images":
		return runLoadImages(ctx, runner, args[1:])
	case "copy-volumes":
		return runCopyVolumes(ctx, runner, args[1:])
	case "verify-volumes":
		return runVerifyVolumes(ctx, runner, args[1:])
	case "stop-docker":
		return runStopDocker(ctx, runner, args[1:])
	case "start-apple":
		return runStartApple(ctx, runner, args[1:])
	case "rollback":
		return runRollback(ctx, runner, args[1:])
	case "plan":
		return runPlanCommand(args[1:], stdout)
	case "report":
		return runReport(args[1:], stdout)
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
	source := addSourceFlags(fs)
	var containers stringList
	fs.Var(&containers, "container", "Docker container name or ID to assess; repeatable")
	composeProject := fs.String("compose-project", "", "Docker Compose project label to assess")
	var services stringList
	var excludeServices stringList
	fs.Var(&services, "service", "Compose service to include; repeatable")
	fs.Var(&excludeServices, "exclude-service", "Compose service to exclude; repeatable")
	all := fs.Bool("all", false, "assess all running Docker containers")
	planPath := fs.String("plan", "xanship-plan.json", "path to write the migration plan")
	targetPrefix := fs.String("target-prefix", "xanship-", "prefix for Apple Container names, volumes, and networks")
	bindPolicy := fs.String("bind-policy", BindPolicyKeep, "bind mount policy: keep, warn, fail, copy-to-volume")
	existingPolicy := fs.String("existing", ExistingPolicyReuse, "existing Apple resource policy: fail, reuse, replace")
	if err := fs.Parse(args); err != nil {
		return err
	}
	runner.dockerArgs = source.dockerArgs()
	if err := RequireExecutable(ctx, runner, "docker"); err != nil {
		return err
	}
	plan, err := BuildPlan(ctx, runner, AssessOptions{
		Containers: containers, ComposeProject: *composeProject, ComposeServices: services, ExcludeServices: excludeServices, All: *all, TargetPrefix: *targetPrefix, BindPolicy: *bindPolicy, ExistingPolicy: *existingPolicy,
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

func runPreflight(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("preflight", flag.ContinueOnError)
	fs.SetOutput(stdout)
	planPath := fs.String("plan", "xanship-plan.json", "migration plan path")
	format := fs.String("format", "text", "output format: text, json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	plan, err := LoadPlan(*planPath)
	if err != nil {
		return err
	}
	report := AnalyzePlan(plan, nil)
	if *format == "json" {
		return writeJSON(stdout, report)
	}
	for _, issue := range report.Issues {
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", issue.Severity, issue.Component, issue.Code, issue.Message)
	}
	if report.HasErrors() {
		return fmt.Errorf("preflight found compatibility errors")
	}
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
	existingPolicy := fs.String("existing", "", "existing Apple resource policy: fail, reuse, replace")
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
	opts := RunOptions{ExistingPolicy: *existingPolicy}
	if opts.ExistingPolicy == "" {
		opts.ExistingPolicy = planExistingPolicy(plan)
	}
	return DryRunAppleWithOptions(ctx, runner, plan, opts)
}

func runLoadImages(ctx context.Context, runner commandRunner, args []string) error {
	fs := flag.NewFlagSet("load-images", flag.ContinueOnError)
	source := addSourceFlags(fs)
	planPath := fs.String("plan", "xanship-plan.json", "migration plan path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	runner.dockerArgs = source.dockerArgs()
	plan, err := LoadPlan(*planPath)
	if err != nil {
		return err
	}
	return LoadImages(ctx, runner, plan)
}

func runCopyVolumes(ctx context.Context, runner commandRunner, args []string) error {
	fs := flag.NewFlagSet("copy-volumes", flag.ContinueOnError)
	source := addSourceFlags(fs)
	planPath := fs.String("plan", "xanship-plan.json", "migration plan path")
	image := fs.String("copy-image", "docker.io/library/busybox:latest", "image used to stream volume tar data")
	verify := fs.Bool("verify", false, "print a verification reminder after volume copy")
	if err := fs.Parse(args); err != nil {
		return err
	}
	runner.dockerArgs = source.dockerArgs()
	plan, err := LoadPlan(*planPath)
	if err != nil {
		return err
	}
	if err := CopyVolumes(ctx, runner, plan, *image); err != nil {
		return err
	}
	if *verify {
		return VerifyVolumes(ctx, runner, plan, *image)
	}
	return nil
}

func runVerifyVolumes(ctx context.Context, runner commandRunner, args []string) error {
	fs := flag.NewFlagSet("verify-volumes", flag.ContinueOnError)
	source := addSourceFlags(fs)
	planPath := fs.String("plan", "xanship-plan.json", "migration plan path")
	image := fs.String("copy-image", "docker.io/library/busybox:latest", "image used to inspect volume data")
	if err := fs.Parse(args); err != nil {
		return err
	}
	runner.dockerArgs = source.dockerArgs()
	plan, err := LoadPlan(*planPath)
	if err != nil {
		return err
	}
	return VerifyVolumes(ctx, runner, plan, *image)
}

func runStopDocker(ctx context.Context, runner commandRunner, args []string) error {
	fs := flag.NewFlagSet("stop-docker", flag.ContinueOnError)
	source := addSourceFlags(fs)
	planPath := fs.String("plan", "xanship-plan.json", "migration plan path")
	timeout := fs.String("timeout", "10", "docker stop timeout seconds")
	if err := fs.Parse(args); err != nil {
		return err
	}
	runner.dockerArgs = source.dockerArgs()
	plan, err := LoadPlan(*planPath)
	if err != nil {
		return err
	}
	return StopDocker(ctx, runner, plan, *timeout)
}

func runStartApple(ctx context.Context, runner commandRunner, args []string) error {
	fs := flag.NewFlagSet("start-apple", flag.ContinueOnError)
	planPath := fs.String("plan", "xanship-plan.json", "migration plan path")
	existingPolicy := fs.String("existing", "", "existing Apple resource policy: fail, reuse, replace")
	if err := fs.Parse(args); err != nil {
		return err
	}
	plan, err := LoadPlan(*planPath)
	if err != nil {
		return err
	}
	opts := RunOptions{ExistingPolicy: *existingPolicy}
	if opts.ExistingPolicy == "" {
		opts.ExistingPolicy = planExistingPolicy(plan)
	}
	return StartAppleWithOptions(ctx, runner, plan, opts)
}

func runRollback(ctx context.Context, runner commandRunner, args []string) error {
	fs := flag.NewFlagSet("rollback", flag.ContinueOnError)
	source := addSourceFlags(fs)
	planPath := fs.String("plan", "xanship-plan.json", "migration plan path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	runner.dockerArgs = source.dockerArgs()
	plan, err := LoadPlan(*planPath)
	if err != nil {
		return err
	}
	return Rollback(ctx, runner, plan)
}

func runPlanCommand(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("missing plan subcommand")
	}
	switch args[0] {
	case "summary":
		fs := flag.NewFlagSet("plan summary", flag.ContinueOnError)
		planPath := fs.String("plan", "xanship-plan.json", "migration plan path")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		plan, err := LoadPlan(*planPath)
		if err != nil {
			return err
		}
		PrintPlanSummary(stdout, plan)
		return nil
	case "validate":
		fs := flag.NewFlagSet("plan validate", flag.ContinueOnError)
		planPath := fs.String("plan", "xanship-plan.json", "migration plan path")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		plan, err := LoadPlan(*planPath)
		if err != nil {
			return err
		}
		return ValidatePlan(plan)
	case "set":
		fs := flag.NewFlagSet("plan set", flag.ContinueOnError)
		planPath := fs.String("plan", "xanship-plan.json", "migration plan path")
		container := fs.String("container", "", "source or target container name")
		field := fs.String("field", "", "field to set: target-name, image")
		value := fs.String("value", "", "new value")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		plan, err := LoadPlan(*planPath)
		if err != nil {
			return err
		}
		if err := SetPlanValue(plan, *container, *field, *value); err != nil {
			return err
		}
		return SavePlan(*planPath, plan)
	default:
		return fmt.Errorf("unknown plan subcommand %q", args[0])
	}
}

func runReport(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	planPath := fs.String("plan", "xanship-plan.json", "migration plan path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	plan, err := LoadPlan(*planPath)
	if err != nil {
		return err
	}
	WriteMarkdownReport(stdout, plan, AnalyzePlan(plan, nil))
	return nil
}

func runMigrate(ctx context.Context, runner commandRunner, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(stdout)
	source := addSourceFlags(fs)
	var containers stringList
	fs.Var(&containers, "container", "Docker container name or ID to migrate; repeatable")
	composeProject := fs.String("compose-project", "", "Docker Compose project label to migrate")
	var services stringList
	var excludeServices stringList
	fs.Var(&services, "service", "Compose service to include; repeatable")
	fs.Var(&excludeServices, "exclude-service", "Compose service to exclude; repeatable")
	all := fs.Bool("all", false, "migrate all running Docker containers")
	planPath := fs.String("plan", "xanship-plan.json", "migration plan path")
	targetPrefix := fs.String("target-prefix", "xanship-", "prefix for Apple Container names, volumes, and networks")
	copyImage := fs.String("copy-image", "docker.io/library/busybox:latest", "image used to stream volume tar data")
	bindPolicy := fs.String("bind-policy", BindPolicyKeep, "bind mount policy: keep, warn, fail, copy-to-volume")
	existingPolicy := fs.String("existing", ExistingPolicyReuse, "existing Apple resource policy: fail, reuse, replace")
	rollbackOnFailure := fs.Bool("rollback-on-failure", true, "restart Docker containers if Apple startup fails")
	skipImageLoad := fs.Bool("skip-image-load", false, "do not docker save | container image load images")
	skipDryRun := fs.Bool("skip-dry-run", false, "skip Apple Container create/delete validation")
	skipCopy := fs.Bool("skip-copy", false, "skip named volume data copy")
	skipStop := fs.Bool("skip-stop", false, "do not stop Docker containers before starting Apple containers")
	timeout := fs.String("timeout", "10", "docker stop timeout seconds")
	if err := fs.Parse(args); err != nil {
		return err
	}
	runner.dockerArgs = source.dockerArgs()
	if err := RequireExecutable(ctx, runner, "docker"); err != nil {
		return err
	}
	if err := RequireExecutable(ctx, runner, "container"); err != nil {
		return err
	}
	plan, err := BuildPlan(ctx, runner, AssessOptions{
		Containers: containers, ComposeProject: *composeProject, ComposeServices: services, ExcludeServices: excludeServices, All: *all, TargetPrefix: *targetPrefix, BindPolicy: *bindPolicy, ExistingPolicy: *existingPolicy,
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
	if err := StartAppleWithOptions(ctx, runner, plan, RunOptions{ExistingPolicy: *existingPolicy}); err != nil {
		if *rollbackOnFailure && !*skipStop {
			_ = Rollback(ctx, runner, plan)
		}
		return err
	}
	return nil
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `Xanship migrates running Docker Desktop containers to Apple Container.

Usage:
  xanship assess [--docker-host ssh://USER@HOST] [--container NAME ... | --compose-project PROJECT | --all]
  xanship commands [--plan xanship-plan.json] [--dry-run]
  xanship preflight [--plan xanship-plan.json] [--format text|json]
  xanship dry-run [--plan xanship-plan.json] [--apply]
  xanship load-images [--plan xanship-plan.json]
  xanship copy-volumes [--plan xanship-plan.json]
  xanship verify-volumes [--plan xanship-plan.json]
  xanship stop-docker [--plan xanship-plan.json]
  xanship start-apple [--plan xanship-plan.json]
  xanship rollback [--plan xanship-plan.json]
  xanship plan summary|validate|set [--plan xanship-plan.json]
  xanship report [--plan xanship-plan.json]
  xanship migrate [--docker-host ssh://USER@HOST] [--container NAME ... | --compose-project PROJECT | --all]
  xanship version

Migration phases:
  assess -> load-images -> dry-run --apply -> copy-volumes -> stop-docker -> start-apple`)
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
