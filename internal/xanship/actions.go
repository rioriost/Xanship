package xanship

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
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

func DryRunApple(ctx context.Context, runner commandRunner, plan *Plan) error {
	for _, n := range plan.Networks {
		if err := ensureAppleNetwork(ctx, runner, n); err != nil {
			return err
		}
	}
	for _, v := range plan.Volumes {
		if err := ensureAppleVolume(ctx, runner, v); err != nil {
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
	for _, n := range plan.Networks {
		if err := ensureAppleNetwork(ctx, runner, n); err != nil {
			return err
		}
	}
	for _, v := range plan.Volumes {
		if err := ensureAppleVolume(ctx, runner, v); err != nil {
			return err
		}
	}
	for _, c := range plan.Containers {
		if err := runner.run(ctx, "container", appleContainerArgs("run", c, "")...); err != nil {
			return err
		}
	}
	return nil
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
		if err := runner.run(ctx, "container", "image", "pull", "--platform", "linux/arm64", image); err == nil {
			continue
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
		if err := ensureAppleVolume(ctx, runner, v); err != nil {
			return err
		}
		if err := copyOneVolume(ctx, runner, v.SourceName, v.TargetName, image); err != nil {
			return err
		}
	}
	return nil
}

func copyOneVolume(ctx context.Context, runner commandRunner, dockerVolume, appleVolume, image string) error {
	dockerArgs := []string{
		"run", "--rm", "-i",
		"--mount", "type=volume,source=" + dockerVolume + ",target=/from,readonly",
		"--entrypoint", "tar",
		image, "-C", "/from", "-cpf", "-", ".",
	}
	appleArgs := []string{
		"run", "--rm", "-i",
		"--mount", "type=volume,source=" + appleVolume + ",target=/to",
		"--entrypoint", "tar",
		image, "-C", "/to", "-xpf", "-",
	}
	dockerCmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	appleCmd := exec.CommandContext(ctx, "container", appleArgs...)
	pipe, err := dockerCmd.StdoutPipe()
	if err != nil {
		return err
	}
	appleCmd.Stdin = pipe
	dockerCmd.Stderr = runner.stderr
	appleCmd.Stdout = runner.stdout
	appleCmd.Stderr = runner.stderr
	if err := dockerCmd.Start(); err != nil {
		return fmt.Errorf("docker volume export %s: %w", dockerVolume, err)
	}
	if err := appleCmd.Start(); err != nil {
		_ = dockerCmd.Process.Kill()
		return fmt.Errorf("Apple volume import %s: %w", appleVolume, err)
	}
	appleErr := appleCmd.Wait()
	dockerErr := dockerCmd.Wait()
	if dockerErr != nil {
		return fmt.Errorf("docker volume export %s failed: %w", dockerVolume, dockerErr)
	}
	if appleErr != nil {
		return fmt.Errorf("Apple volume import %s failed: %w", appleVolume, appleErr)
	}
	return nil
}

func ensureAppleVolume(ctx context.Context, runner commandRunner, v VolumePlan) error {
	if exists(ctx, runner, "container", "volume", "inspect", v.TargetName) {
		return nil
	}
	return runner.run(ctx, "container", appleVolumeCreateArgs(v)...)
}

func ensureAppleNetwork(ctx context.Context, runner commandRunner, n NetworkPlan) error {
	if exists(ctx, runner, "container", "network", "inspect", n.TargetName) {
		return nil
	}
	return runner.run(ctx, "container", appleNetworkCreateArgs(n)...)
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
