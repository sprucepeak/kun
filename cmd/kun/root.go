package kun

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/sprucepeak/kun/config"
	"github.com/sprucepeak/kun/internal/check"
	"github.com/sprucepeak/kun/internal/gen"
	"github.com/sprucepeak/kun/internal/mock"
	"github.com/sprucepeak/kun/internal/new"
	"github.com/sprucepeak/kun/internal/run"
	"github.com/sprucepeak/kun/internal/swag"
	"github.com/sprucepeak/kun/internal/upgrade"
	"github.com/sprucepeak/kun/internal/wire"
)

var CmdRoot = &cobra.Command{
	Use:               "kun",
	Example:           "kun new demo",
	Short:             config.Short,
	Version:           config.Version,
	SilenceErrors:     true, // 子命令失败由其自行 output.Error 提示,避免重复打印 "Error: ..."
	SilenceUsage:      true, // 运行期错误不打印 usage,仅参数校验错误由 cobra 打印
	CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
}

func init() {
	// E6: 各子命令包通过 Register 自行维护命令树，root 只负责顶层注册，
	// 新增子命令只需在对应包中修改，不会遗漏。
	new.Register(CmdRoot)
	run.Register(CmdRoot)
	upgrade.Register(CmdRoot)
	gen.Register(CmdRoot)
	wire.Register(CmdRoot)
	mock.Register(CmdRoot)
	check.Register(CmdRoot)
	swag.Register(CmdRoot)
}

// CommandError 包装命令执行过程中发生的错误，附带目标命令的路径。
type CommandError struct {
	Err     error
	CmdPath string
}

func (e *CommandError) Error() string {
	return e.Err.Error()
}

func (e *CommandError) Unwrap() error {
	return e.Err
}

// UsageError 与 ArgsError 是 CommandError 的别名，保证向后兼容。
type UsageError = CommandError
type ArgsError = CommandError

// IsUsageError 判断错误是否为参数、子命令或 flag 用法相关错误。
func IsUsageError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "unknown command ") ||
		strings.Contains(msg, "arg(s), received ") ||
		strings.Contains(msg, "arg, received ") ||
		(strings.Contains(msg, "accepts between ") && strings.Contains(msg, "arg")) ||
		(strings.Contains(msg, "requires at least ") && strings.Contains(msg, "arg")) ||
		(strings.Contains(msg, "accepts at most ") && strings.Contains(msg, "arg")) ||
		(strings.Contains(msg, "accepts ") && strings.Contains(msg, "arg(s)")) ||
		strings.Contains(msg, "requires a subcommand") ||
		strings.Contains(msg, "unknown flag: ") ||
		strings.Contains(msg, "unknown shorthand flag: ")
}

// IsArgsError 是 IsUsageError 的别名，保持向后兼容。
func IsArgsError(err error) bool {
	return IsUsageError(err)
}

// Execute executes the root command.
func Execute() error {
	cmd, err := CmdRoot.ExecuteC()
	if err != nil {
		cmdPath := CmdRoot.CommandPath()
		if cmd != nil {
			cmdPath = cmd.CommandPath()
		}
		return &CommandError{
			Err:     err,
			CmdPath: cmdPath,
		}
	}
	return nil
}
