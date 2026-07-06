package xanship

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type commandRunner struct {
	stdout        io.Writer
	stderr        io.Writer
	dockerArgs    []string
	containerArgs []string
}

func (r commandRunner) command(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, r.commandArgs(name, args...)...)
}

func (r commandRunner) commandArgs(name string, args ...string) []string {
	switch name {
	case "docker":
		return append(append([]string(nil), r.dockerArgs...), args...)
	case "container":
		return append(append([]string(nil), r.containerArgs...), args...)
	default:
		return args
	}
}

func (r commandRunner) output(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := r.command(ctx, name, args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s %s failed: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

func (r commandRunner) run(ctx context.Context, name string, args ...string) error {
	cmd := r.command(ctx, name, args...)
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func (r commandRunner) json(ctx context.Context, dest any, name string, args ...string) error {
	out, err := r.output(ctx, name, args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(out, dest); err != nil {
		return fmt.Errorf("decode %s %s JSON: %w", name, strings.Join(args, " "), err)
	}
	return nil
}
