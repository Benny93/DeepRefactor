package processor

import (
	"bytes"
	"context"
	"deeprefactor/internal/types"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func FindGoFiles(dir string) ([]*types.FileProcess, error) {
	var files []*types.FileProcess
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			files = append(files, &types.FileProcess{
				Path:   path,
				Status: "Pending",
			})
		}
		return nil
	})
	return files, err
}

func RunLintCommand(ctx context.Context, cmd string) (string, error) {
	parts := strings.Split(cmd, " ")
	var stdout, stderr bytes.Buffer
	c := exec.CommandContext(ctx, parts[0], parts[1:]...)
	c.Stdout = &stdout
	c.Stderr = &stderr

	err := c.Run()
	output := strings.TrimSpace(stderr.String() + stdout.String())

	if err != nil {
		return output, fmt.Errorf("lint failed: %w", err)
	}
	return output, nil
}

func ShortPath(path string) string {
	if len(path) > 50 {
		return "..." + path[len(path)-47:]
	}
	return path
}
