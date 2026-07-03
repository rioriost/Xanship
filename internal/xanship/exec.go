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
	stdout io.Writer
	stderr io.Writer
}

func (r commandRunner) output(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s %s failed: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

func (r commandRunner) run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
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
