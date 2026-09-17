package swag

import (
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
	generalInfo     string
	outputDir       string
	parseDependency bool
	noInstall       bool
)

func init() {
	CmdSwag.Flags().StringVarP(&generalInfo, "generalInfo", "g", "", "Go file path containing general API info (default auto-detected, e.g. cmd/server/main.go)")
	CmdSwag.Flags().StringVarP(&outputDir, "output", "o", "swagger", "Output directory for swagger files")
	CmdSwag.Flags().BoolVarP(&parseDependency, "parseDependency", "p", true, "Parse outside dependency packages")
	CmdSwag.Flags().BoolVar(&noInstall, "no-install", false, "Do not auto-install swag if missing")
}

var CmdSwag = &cobra.Command{
	Use:     "swag",
	Short:   "Generate Swagger API documentation using swag",
	Long:    "kun swag parses Go source comments and generates Swagger API documentation into the swagger directory.",
	Example: "  kun swag\n  kun swag -g cmd/server/main.go -o swagger\n  kun swag --no-install",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. 检查 swagger 目标文件夹是否存在
		out := outputDir
		if out == "" {
			out = "swagger"
		}

		info, err := os.Stat(out)
		if err != nil || !info.IsDir() {
			output.Error("未找到 %s 文件夹！", out)
			output.Tip("在 kun 架构中，Swagger 文档产物默认统一存放在 %s/ 目录下供路由层直接导入编译。", out)
			output.Tip("请先在项目根目录下执行创建命令: mkdir %s", out)
			return fmt.Errorf("%s 文件夹不存在，请先创建", out)
		}

		// 2. 检查并确保 swag 工具已就绪
		bin, err := ensureSwag(noInstall)
		if err != nil {
			return err
		}

		// 3. 自动探测 generalInfo (入口 main.go 路径)
		mainPath := generalInfo
		if mainPath == "" {
			if _, e := os.Stat(filepath.Join("cmd", "server", "main.go")); e == nil {
				mainPath = filepath.Join("cmd", "server", "main.go")
			} else if _, e := os.Stat("main.go"); e == nil {
				mainPath = "main.go"
			} else {
				// 尝试通过 helper.FindMain 搜索
				if mains, findErr := helper.FindMain(".", config.RunExcludeDir); findErr == nil && len(mains) > 0 {
					for k := range mains {
						mainPath = k
						break
					}
				}
				if mainPath == "" {
					mainPath = filepath.Join("cmd", "server", "main.go")
				}
			}
		}

		// 4. 构建 swag 命令参数
		swagArgs := []string{"init", "-g", filepath.ToSlash(mainPath), "-o", filepath.ToSlash(out)}
		if parseDependency {
			swagArgs = append(swagArgs, "--parseDependency")
		}

		_, extraArgs := helper.SplitArgs(cmd, args)
		if len(extraArgs) > 0 {
			swagArgs = append(swagArgs, extraArgs...)
		}

		output.Success("running: swag %s", strings.Join(swagArgs, " "))

		execCmd := exec.Command(bin, swagArgs...)
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
		if err := execCmd.Run(); err != nil {
			return fmt.Errorf("swag generate error: %w", err)
		}

		output.Success("Swagger API 文档已成功生成至 %s/ 目录！", out)
		return nil
	},
}

// Register 将 swag 子命令挂载到 parent。
func Register(parent *cobra.Command) {
	parent.AddCommand(CmdSwag)
}

func ensureSwag(noInstall bool) (string, error) {
	if bin, err := findTool("swag"); err == nil {
		return bin, nil
	}

	if noInstall {
		return "", fmt.Errorf("swag is not installed and --no-install was specified, please install it: go install %s", config.SwagUrl)
	}

	output.Warn("检测到系统未安装 swag，首次使用正在自动安装中，请稍候...")
	output.Tip("执行命令: go install %s", config.SwagUrl)

	cmd := exec.Command("go", "install", config.SwagUrl)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("auto-install swag failed: %w (请检查 GOPROXY 或手动执行: go install %s)", err, config.SwagUrl)
	}

	output.Success("swag 安装成功！")

	bin, err := findTool("swag")
	if err != nil {
		return "", fmt.Errorf("swag installed but cannot be located in PATH or GOPATH/bin: %w", err)
	}
	return bin, nil
}

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
