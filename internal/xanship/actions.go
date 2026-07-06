package xanship

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func PrintAppleCommands(w io.Writer, plan *Plan, dryRun bool) {
	for _, n := range plan.Networks {
		fmt.Fprintln(w, commandLine("container", appleNetworkCreateArgs(n)))
	}
	for _, v := range plan.Volumes {
		fmt.Fprintln(w, commandLine("container", appleVolumeCreateArgs(v)))
	}
	for _, c := range plan.Containers {
		action := "run"
		name := ""
		if dryRun {
			action = "create"
			name = c.TargetName + "-dryrun"
		}
		fmt.Fprintln(w, commandLine("container", appleContainerArgs(action, c, name)))
		if dryRun {
			fmt.Fprintln(w, commandLine("container", []string{"delete", name}))
		}
	}
}

type RunOptions struct {
	ExistingPolicy string
}

func DryRunApple(ctx context.Context, runner commandRunner, plan *Plan) error {
	return DryRunAppleWithOptions(ctx, runner, plan, RunOptions{ExistingPolicy: planExistingPolicy(plan)})
}

func DryRunAppleWithOptions(ctx context.Context, runner commandRunner, plan *Plan, opts RunOptions) error {
	policy, err := normalizeExistingPolicy(opts.ExistingPolicy)
	if err != nil {
		return err
	}
	for _, n := range plan.Networks {
		if err := ensureAppleNetwork(ctx, runner, n, policy); err != nil {
			return err
		}
	}
	for _, v := range plan.Volumes {
		if err := ensureAppleVolume(ctx, runner, v, policy); err != nil {
			return err
		}
	}
	for _, c := range plan.Containers {
		name := c.TargetName + "-dryrun"
		args := appleContainerArgs("create", c, name)
		if err := runner.run(ctx, "container", args...); err != nil {
			return err
		}
		_ = runner.run(ctx, "container", "delete", name)
	}
	return nil
}

func StartApple(ctx context.Context, runner commandRunner, plan *Plan) error {
	return StartAppleWithOptions(ctx, runner, plan, RunOptions{ExistingPolicy: planExistingPolicy(plan)})
}

func StartAppleWithOptions(ctx context.Context, runner commandRunner, plan *Plan, opts RunOptions) error {
	policy, err := normalizeExistingPolicy(opts.ExistingPolicy)
	if err != nil {
		return err
	}
	for _, n := range plan.Networks {
		if err := ensureAppleNetwork(ctx, runner, n, policy); err != nil {
			return err
		}
	}
	for _, v := range plan.Volumes {
		if err := ensureAppleVolume(ctx, runner, v, policy); err != nil {
			return err
		}
	}
	for _, c := range plan.Containers {
		if exists(ctx, runner, "container", "inspect", c.TargetName) {
			switch policy {
			case ExistingPolicyReuse:
				continue
			case ExistingPolicyReplace:
				if err := runner.run(ctx, "container", "delete", "--force", c.TargetName); err != nil {
					return err
				}
			default:
				return fmt.Errorf("Apple container %s already exists; use --existing reuse or --existing replace", c.TargetName)
			}
		}
		if err := runner.run(ctx, "container", appleContainerArgs("run", c, "")...); err != nil {
			return err
		}
	}
	return nil
}

func planExistingPolicy(plan *Plan) string {
	if plan != nil && plan.Options.ExistingPolicy != "" {
		return plan.Options.ExistingPolicy
	}
	return ExistingPolicyReuse
}

func StopDocker(ctx context.Context, runner commandRunner, plan *Plan, timeout string) error {
	for _, c := range plan.Containers {
		args := []string{"stop"}
		if timeout != "" {
			args = append(args, "--timeout", timeout)
		}
		args = append(args, c.SourceID)
		if err := runner.run(ctx, "docker", args...); err != nil {
			return err
		}
	}
	return nil
}

func LoadImages(ctx context.Context, runner commandRunner, plan *Plan) error {
	for _, image := range plan.Images {
		decision := DecideImageTransfer(image)
		if decision.Mode == ImageTransferPull {
			if err := runner.run(ctx, "container", "image", "pull", "--platform", "linux/arm64", image); err == nil {
				continue
			}
		}
		tmp, err := os.CreateTemp("", "xanship-image-*.tar")
		if err != nil {
			return err
		}
		tmpPath := tmp.Name()
		if err := tmp.Close(); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
		defer os.Remove(tmpPath)
		if err := runner.run(ctx, "docker", "save", "--output", tmpPath, image); err != nil {
			return err
		}
		if err := runner.run(ctx, "container", "image", "load", "--input", filepath.Clean(tmpPath)); err != nil {
			return err
		}
		_ = os.Remove(tmpPath)
	}
	return nil
}

func CopyVolumes(ctx context.Context, runner commandRunner, plan *Plan, image string) error {
	for _, v := range plan.Volumes {
		if err := ensureAppleVolume(ctx, runner, v, planExistingPolicy(plan)); err != nil {
			return err
		}
		if err := copyOneVolume(ctx, runner, v, image); err != nil {
			return err
		}
	}
	return nil
}

func VerifyVolumes(ctx context.Context, runner commandRunner, plan *Plan, image string) error {
	for _, v := range plan.Volumes {
		sourceEntries, err := volumeEntries(ctx, runner, "docker", sourceMountSpec(v), image)
		if err != nil {
			return fmt.Errorf("source volume %s manifest: %w", v.SourceName, err)
		}
		targetEntries, err := volumeEntries(ctx, runner, "container", "type=volume,source="+v.TargetName+",target=/from,readonly", image)
		if err != nil {
			return fmt.Errorf("target volume %s manifest: %w", v.TargetName, err)
		}
		result := CompareVolumeEntries(sourceEntries, targetEntries)
		if !result.OK() {
			return fmt.Errorf("volume %s verification failed: %#v", v.TargetName, result)
		}
	}
	return nil
}

func volumeEntries(ctx context.Context, runner commandRunner, runtime, mount, image string) ([]VolumeEntry, error) {
	args := []string{
		"run", "--rm",
		"--mount", mount,
		"--entrypoint", "sh",
		image, "-c", "cd /from && find . -type f -exec wc -c {} \\;",
	}
	out, err := runner.output(ctx, runtime, args...)
	if err != nil {
		return nil, err
	}
	return parseVolumeEntries(string(out)), nil
}

func copyOneVolume(ctx context.Context, runner commandRunner, volume VolumePlan, image string) error {
	dockerArgs := []string{
		"run", "--rm", "-i",
		"--mount", sourceMountSpec(volume),
		"--entrypoint", "tar",
		image, "-C", "/from", "-cpf", "-", ".",
	}
	appleArgs := []string{
		"run", "--rm", "-i",
		"--mount", "type=volume,source=" + volume.TargetName + ",target=/to",
		"--entrypoint", "tar",
		image, "-C", "/to", "-xpf", "-",
	}
	dockerCmd := runner.command(ctx, "docker", dockerArgs...)
	appleCmd := runner.command(ctx, "container", appleArgs...)
	pipe, err := dockerCmd.StdoutPipe()
	if err != nil {
		return err
	}
	appleCmd.Stdin = pipe
	dockerCmd.Stderr = runner.stderr
	appleCmd.Stdout = runner.stdout
	appleCmd.Stderr = runner.stderr
	if err := dockerCmd.Start(); err != nil {
		return fmt.Errorf("docker volume export %s: %w", volume.SourceName, err)
	}
	if err := appleCmd.Start(); err != nil {
		_ = dockerCmd.Process.Kill()
		return fmt.Errorf("Apple volume import %s: %w", volume.TargetName, err)
	}
	appleErr := appleCmd.Wait()
	dockerErr := dockerCmd.Wait()
	if dockerErr != nil {
		return fmt.Errorf("docker volume export %s failed: %w", volume.SourceName, dockerErr)
	}
	if appleErr != nil {
		return fmt.Errorf("Apple volume import %s failed: %w", volume.TargetName, appleErr)
	}
	return nil
}

func sourceMountSpec(v VolumePlan) string {
	if v.SourceKind == "bind" {
		return "type=bind,source=" + v.SourcePath + ",target=/from,readonly"
	}
	return "type=volume,source=" + v.SourceName + ",target=/from,readonly"
}

func ensureAppleVolume(ctx context.Context, runner commandRunner, v VolumePlan, policy string) error {
	if exists(ctx, runner, "container", "volume", "inspect", v.TargetName) {
		switch policy {
		case ExistingPolicyReuse:
			return nil
		case ExistingPolicyReplace:
			if err := runner.run(ctx, "container", "volume", "delete", v.TargetName); err != nil {
				return err
			}
		default:
			return fmt.Errorf("Apple volume %s already exists; use --existing reuse or --existing replace", v.TargetName)
		}
	}
	return runner.run(ctx, "container", appleVolumeCreateArgs(v)...)
}

func ensureAppleNetwork(ctx context.Context, runner commandRunner, n NetworkPlan, policy string) error {
	if exists(ctx, runner, "container", "network", "inspect", n.TargetName) {
		switch policy {
		case ExistingPolicyReuse:
			return nil
		case ExistingPolicyReplace:
			if err := runner.run(ctx, "container", "network", "delete", n.TargetName); err != nil {
				return err
			}
		default:
			return fmt.Errorf("Apple network %s already exists; use --existing reuse or --existing replace", n.TargetName)
		}
	}
	return runner.run(ctx, "container", appleNetworkCreateArgs(n)...)
}

func Rollback(ctx context.Context, runner commandRunner, plan *Plan) error {
	for i := len(plan.Containers) - 1; i >= 0; i-- {
		_ = runner.run(ctx, "container", "delete", "--force", plan.Containers[i].TargetName)
	}
	for i := len(plan.Networks) - 1; i >= 0; i-- {
		_ = runner.run(ctx, "container", "network", "delete", plan.Networks[i].TargetName)
	}
	for i := len(plan.Containers) - 1; i >= 0; i-- {
		if err := runner.run(ctx, "docker", "start", plan.Containers[i].SourceID); err != nil {
			return err
		}
	}
	return nil
}

func exists(ctx context.Context, runner commandRunner, name string, args ...string) bool {
	_, err := runner.output(ctx, name, args...)
	return err == nil
}

func RequireExecutable(ctx context.Context, runner commandRunner, name string) error {
	if _, err := runner.output(ctx, name, "--help"); err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return fmt.Errorf("%s CLI is not installed or not on PATH", name)
		}
		return err
	}
	return nil
}
