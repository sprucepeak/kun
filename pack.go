//go:build ignore

package main

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 模板打包脚本：用于将 tpl/basic 和 tpl/advanced 目录打包生成对应的 zip 文件。
// 仅用于开发维护，执行命令：go run pack.go

var ignorePatterns = []string{
	".git",
	".idea",
	".vscode",
	".DS_Store",
	"bin",
}

func shouldIgnore(relPath string) bool {
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	for _, part := range parts {
		for _, ignore := range ignorePatterns {
			if part == ignore {
				return true
			}
		}
		if strings.HasSuffix(part, ".zip") || strings.HasSuffix(part, ".exe") {
			return true
		}
	}
	return false
}

func packTemplate(srcDir, zipFilePath string) (int, int64, error) {
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return 0, 0, fmt.Errorf("source directory %s does not exist", srcDir)
	}

	zipFile, err := os.Create(zipFilePath)
	if err != nil {
		return 0, 0, fmt.Errorf("create zip file failed: %w", err)
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	fileCount := 0

	err = filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// 计算相对于 srcDir 的路径
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		if shouldIgnore(relPath) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// zip 内路径统一使用 '/' 分隔符
		zipPath := filepath.ToSlash(relPath)

		info, err := d.Info()
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		header.Name = zipPath
		header.Modified = time.Now()

		if d.IsDir() {
			header.Name += "/"
			header.Method = zip.Store
			if _, err := archive.CreateHeader(header); err != nil {
				return err
			}
			return nil
		}

		header.Method = zip.Deflate // 开启 Deflate 压缩

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		if _, err := io.Copy(writer, file); err != nil {
			return err
		}

		fileCount++
		return nil
	})

	if err != nil {
		return 0, 0, err
	}

	if err := archive.Close(); err != nil {
		return 0, 0, err
	}

	fi, err := zipFile.Stat()
	if err != nil {
		return 0, 0, err
	}

	return fileCount, fi.Size(), nil
}

func main() {
	templates := []struct {
		name    string
		srcDir  string
		zipFile string
	}{
		{name: "basic", srcDir: filepath.Join("tpl", "basic"), zipFile: filepath.Join("tpl", "basic.zip")},
		{name: "advanced", srcDir: filepath.Join("tpl", "advanced"), zipFile: filepath.Join("tpl", "advanced.zip")},
	}

	fmt.Println("==> Packing templates to zip archives...")
	hasError := false

	for _, t := range templates {
		if _, err := os.Stat(t.srcDir); os.IsNotExist(err) {
			fmt.Printf("[SKIP] %-10s does not exist, keeping existing %s\n", t.srcDir, t.zipFile)
			continue
		}

		count, size, err := packTemplate(t.srcDir, t.zipFile)
		if err != nil {
			fmt.Printf("[ERROR] Failed to pack %s: %v\n", t.name, err)
			hasError = true
			continue
		}
		fmt.Printf("[OK]   %-10s -> %-18s (%d files, %.2f KB)\n",
			t.srcDir, t.zipFile, count, float64(size)/1024)
	}

	if hasError {
		os.Exit(1)
	}
	fmt.Println("==> Packing completed successfully!")
}
