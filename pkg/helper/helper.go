package helper

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var reMainFunc = regexp.MustCompile(`func\s+main\s*\(`)

// GetProjectName 从 dir 目录的 go.mod 读取 module 名称。
// 返回 error 而非静默返回空串,调用方必须检查,避免空包名流入代码生成。
func GetProjectName(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("go.mod does not exist: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read go.mod error: %w", err)
	}
	return "", fmt.Errorf("module declaration not found in go.mod")
}

func SplitArgs(cmd *cobra.Command, args []string) (cmdArgs, programArgs []string) {
	dashAt := cmd.ArgsLenAtDash()
	if dashAt >= 0 {
		return args[:dashAt], args[dashAt:]
	}
	return args, []string{}
}

func FindMain(base, excludeDir string) (map[string]string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	// 统一为正斜杠,避免 Windows 下 / 与 \ 混合导致 TrimPrefix 失效
	wd = filepath.ToSlash(wd)
	if !strings.HasSuffix(wd, "/") {
		wd += "/"
	}
	excludeDirArr := strings.Split(excludeDir, ",")
	for i := range excludeDirArr {
		excludeDirArr[i] = strings.TrimSpace(excludeDirArr[i])
	}
	cmdPath := make(map[string]string)
	err = filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && IsExcluded(path, excludeDirArr) {
			return filepath.SkipDir
		}
		if !info.IsDir() && filepath.Ext(path) == ".go" {
			// 只检查 cmd/ 目录下的文件，避免读取无关文件
			// 用路径分段判断,避免 Contains 误匹配 mycmd/、acmd.go 等
			segs := strings.Split(filepath.ToSlash(path), "/")
			hasCmdSeg := false
			for _, s := range segs {
				if s == "cmd" {
					hasCmdSeg = true
					break
				}
			}
			if !hasCmdSeg {
				return nil
			}
			// M4: 超过 1MB 的文件肯定不是普通 Go 源文件，跳过以防读二进制文件
			const maxGoFileSize = 1 << 20 // 1 MB
			if info.Size() > maxGoFileSize {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !strings.Contains(string(content), "package main") {
				return nil
			}
			if reMainFunc.Match(content) {
				absPath, absErr := filepath.Abs(path)
				if absErr != nil {
					return absErr
				}
				d, _ := filepath.Split(absPath)
				cmdPath[strings.TrimPrefix(filepath.ToSlash(absPath), wd)] = d
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return cmdPath, nil
}

// IsExcluded 判断 path 是否位于任一排除目录下（或本身就是排除目录）。
// 统一用正斜杠比较,跨平台一致。
func IsExcluded(path string, excludeDirs []string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	for _, dir := range excludeDirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		if clean == dir || strings.HasPrefix(clean, dir+"/") {
			return true
		}
	}
	return false
}

// FindTool 在 PATH 以及 GOBIN/GOPATH 目录下查找可执行文件。
func FindTool(toolName string) (string, error) {
	if p, err := exec.LookPath(toolName); err == nil {
		return p, nil
	}

	gobin := GetGoBin()
	if gobin != "" {
		candidates := []string{
			filepath.Join(gobin, toolName),
			filepath.Join(gobin, toolName+".exe"),
		}
		for _, c := range candidates {
			if info, err := os.Stat(c); err == nil && !info.IsDir() {
				return c, nil
			}
		}
	}
	return "", fmt.Errorf("tool %s not found in PATH or GOPATH/bin", toolName)
}

// GetGoPath 获取系统的 GOPATH 路径。
func GetGoPath() string {
	gopath := os.Getenv("GOPATH")
	if gopath != "" {
		return gopath
	}
	out, err := exec.Command("go", "env", "GOPATH").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

// GetGoBin 获取 GOBIN 路径（若未设置则回退至 GOPATH/bin）。
func GetGoBin() string {
	gobin := os.Getenv("GOBIN")
	if gobin != "" {
		return gobin
	}
	gopath := GetGoPath()
	if gopath != "" {
		return filepath.Join(gopath, "bin")
	}
	return ""
}
