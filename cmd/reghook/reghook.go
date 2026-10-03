//go:build ignore

// Command reghook registers the hooks provided within the githooks folder of this repository.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	sourceDir := "githooks"
	targetDir := filepath.Join(".git", "hooks")

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		fmt.Printf("failed to create hooks directory: %v\n", err)
		os.Exit(1)
	}

	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		fmt.Printf("failed to read githooks directory: %v\n", err)
		os.Exit(1)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		srcPath := filepath.Join(sourceDir, entry.Name())
		dstPath := filepath.Join(targetDir, entry.Name())

		if err := copyFile(srcPath, dstPath); err != nil {
			fmt.Printf("failed to copy %s: %v\n", entry.Name(), err)
			continue
		}

		if err := os.Chmod(dstPath, 0755); err != nil {
			fmt.Printf("failed to set executable for %s: %v\n", entry.Name(), err)
		}

		fmt.Println("installed hook:", entry.Name())
	}

	fmt.Println("successfully installed Git pre-commit hooks")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Sync()
}
