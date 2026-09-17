package check

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/sprucepeak/kun/config"
	"github.com/sprucepeak/kun/pkg/helper"
	"github.com/sprucepeak/kun/pkg/output"
)

var (
	flagLint      bool
	flagRace      bool
	flagCVE       bool
	flagNil       bool
	flagDeep      bool
	flagNoInstall bool
)

func init() {
	CmdCheck.Flags().BoolVar(&flagLint, "lint", false, "run 'golangci-lint' static analysis (includes vet, errcheck, gosec, nilerr)")
	CmdCheck.Flags().BoolVar(&flagRace, "race", false, "run 'go test -race' concurrency race check")
	CmdCheck.Flags().BoolVar(&flagCVE, "cve", false, "run 'govulncheck' security CVE vulnerability audit")
	CmdCheck.Flags().BoolVar(&flagNil, "nil", false, "run 'nilaway' deep SSA nil-panic dataflow analysis")
	CmdCheck.Flags().BoolVar(&flagDeep, "deep", false, "run full deep check suite (lint + race + cve + nil)")
	CmdCheck.Flags().BoolVar(&flagNoInstall, "no-install", false, "do not auto-install missing external tools")
}

var CmdCheck = &cobra.Command{
	Use:   "check [path]",
	Short: "Run code quality, security and race checks",
	Long: `kun check provides a tiered engineering quality gate:
  • Default Fast Mode: runs 'golangci-lint' (covering vet, errcheck, gosec, nilerr) + 'race test' (< 3s)
  • Deep / Security Audit Mode (--deep / --cve / --nil): adds 'govulncheck' and Uber 'nilaway' SSA analysis.

Missing external tools are automatically installed on first run.`,
	Example: `  kun check                       # Default: runs golangci-lint + race test
  kun check ./internal/...        # Check specified path
  kun check --deep                # Full deep suite (lint + race + cve + nil)
  kun check --cve                 # Run CVE vulnerability audit
  kun check --nil                 # Run nilaway SSA analysis
  kun check --no-install          # Offline mode`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmdArgs, _ := helper.SplitArgs(cmd, args)
		target := "./..."
		if len(cmdArgs) > 0 && cmdArgs[0] != "" {
			target = cmdArgs[0]
		}

		// 判断运行模式:
		// 1. 如果指定了 --deep: 执行全部 4 项
		// 2. 如果未指定任何子项开关: 默认走日常极速模式 (lint + race)
		// 3. 如果指定了具体开关: 仅执行选中的项
		hasSpecificFlag := flagLint || flagRace || flagCVE || flagNil
		runDefault := !hasSpecificFlag && !flagDeep

		type runner struct {
			name     string
			desc     string
			selected bool
			status   string
			err      error
			action   func(target string) error
		}

		runners := []*runner{
			{
				name:     "golangci-lint",
				desc:     "多维静态分析 (vet/errcheck/gosec/nilerr)",
				selected: flagDeep || flagLint || runDefault,
				action: func(t string) error {
					bin, err := ensureTool("golangci-lint", config.GolangciLintUrl, flagNoInstall)
					if err != nil {
						return err
					}
					// golangci-lint run [path]
					return runCommand(bin, "run", t)
				},
			},
			{
				name:     "race test",
				desc:     "并发竞态动态检测 (go test -race)",
				selected: flagDeep || flagRace || runDefault,
				action: func(t string) error {
					return runCommand("go", "test", "-race", t)
				},
			},
			{
				name:     "govulncheck",
				desc:     "第三方依赖公开 CVE 漏洞审计",
				selected: flagDeep || flagCVE,
				action: func(t string) error {
					bin, err := ensureTool("govulncheck", config.GovulnUrl, flagNoInstall)
					if err != nil {
						return err
					}
					return runCommand(bin, t)
				},
			},
			{
				name:     "nilaway",
				desc:     "跨函数 SSA 深度空指针追踪",
				selected: flagDeep || flagNil,
				action: func(t string) error {
					bin, err := ensureTool("nilaway", config.NilawayUrl, flagNoInstall)
					if err != nil {
						return err
					}
					return runCommand(bin, t)
				},
			},
		}

		output.Green("开始执行代码质量检查，目标: %s", target)

		for _, r := range runners {
			if !r.selected {
				continue
			}

			fmt.Println()
			output.Green("▶ 正在执行 [%s] (%s)...", r.name, r.desc)

			if err := r.action(target); err != nil {
				var toolErr *toolInstallError
				if errors.As(err, &toolErr) {
					r.status = "SKIPPED"
					r.err = toolErr.Err
					output.Warn("%s 已跳过: %v", r.name, toolErr.Err)
				} else {
					r.status = "FAILED"
					r.err = err
					output.Error("%s 检查未通过: %v", r.name, err)
				}
			} else {
				r.status = "PASSED"
				output.Success("%s 检查通过", r.name)
			}
		}

		// 汇总看板展示
		fmt.Println()
		output.Green("============================================================")
		output.Green("               kun check 质量检查结果汇总                    ")
		output.Green("============================================================")
		hasFailure := false
		for _, r := range runners {
			if !r.selected {
				continue
			}
			switch r.status {
			case "PASSED":
				output.Success("%-15s - %-30s PASSED", r.name, r.desc)
			case "SKIPPED":
				output.Warn("%-15s - %-30s SKIPPED", r.name, r.desc)
			case "FAILED":
				hasFailure = true
				output.Error("%-15s - %-30s FAILED", r.name, r.desc)
			}
		}
		output.Green("============================================================")

		if hasFailure {
			return errors.New("存在未通过的质量检查项，请根据上方日志进行排查修复")
		}

		output.Success("所有已执行的质量检查均已顺利通过！")
		return nil
	},
}

// CmdCheckInit 一键安装所有外部依赖分析工具
var CmdCheckInit = &cobra.Command{
	Use:   "init",
	Short: "Install or update all external check tools (golangci-lint, govulncheck, nilaway)",
	RunE: func(_ *cobra.Command, _ []string) error {
		tools := []struct {
			name string
			url  string
		}{
			{"golangci-lint", config.GolangciLintUrl},
			{"govulncheck", config.GovulnUrl},
			{"nilaway", config.NilawayUrl},
		}

		for _, t := range tools {
			output.Green("正在安装/更新 %s (%s)...", t.name, t.url)
			cmd := exec.Command("go", "install", t.url)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("install %s error: %w", t.name, err)
			}
			output.Success("%s 安装成功！", t.name)
		}
		output.Success("全部代码检查工具链初始化完成！")
		return nil
	},
}

// Register 将 check 及其子命令挂载到 parent。
func Register(parent *cobra.Command) {
	parent.AddCommand(CmdCheck)
	CmdCheck.AddCommand(CmdCheckInit)
}

type toolInstallError struct {
	Tool string
	Err  error
}

func (e *toolInstallError) Error() string {
	return fmt.Sprintf("tool %s install error: %v", e.Tool, e.Err)
}

// ensureTool 查找工具路径，若未找到且允许安装则执行自动安装
func ensureTool(name, installUrl string, noInstall bool) (string, error) {
	if bin, err := findTool(name); err == nil {
		return bin, nil
	}

	if noInstall {
		return "", &toolInstallError{Tool: name, Err: fmt.Errorf("工具未安装且已指定 --no-install")}
	}

	output.Warn("检测到系统未安装 %s，首次使用正在自动安装中，请稍候...", name)
	output.Tip("执行命令: go install %s", installUrl)

	cmd := exec.Command("go", "install", installUrl)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", &toolInstallError{
			Tool: name,
			Err:  fmt.Errorf("自动安装失败: %w（若遇网络超时请检查 GOPROXY，或手动执行: go install %s）", err, installUrl),
		}
	}

	output.Success("%s 安装成功！", name)

	bin, err := findTool(name)
	if err != nil {
		return "", &toolInstallError{
			Tool: name,
			Err:  fmt.Errorf("安装成功但未能在 PATH 或 GOPATH/bin 中定位到可执行文件: %w", err),
		}
	}
	return bin, nil
}

// findTool 在 PATH 以及 GOBIN/GOPATH 目录下查找可执行文件
func findTool(toolName string) (string, error) {
	if p, err := exec.LookPath(toolName); err == nil {
		return p, nil
	}

	gobin := getGoBin()
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

func getGoPath() string {
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

func getGoBin() string {
	gobin := os.Getenv("GOBIN")
	if gobin != "" {
		return gobin
	}
	gopath := getGoPath()
	if gopath != "" {
		return filepath.Join(gopath, "bin")
	}
	return ""
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
