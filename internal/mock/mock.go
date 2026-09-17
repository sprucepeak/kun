package mock

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/sprucepeak/kun/config"
	"github.com/sprucepeak/kun/pkg/helper"
	"github.com/sprucepeak/kun/pkg/output"
)

var (
	verbose  bool
	printCmd bool
	runRegex string
)

func init() {
	CmdMock.Flags().BoolVarP(&verbose, "verbose", "v", false, "print the names of packages and files as they are evaluated")
	CmdMock.Flags().BoolVarP(&printCmd, "print", "x", false, "print commands as they are executed")
	CmdMock.Flags().StringVarP(&runRegex, "run", "r", "", "regular expression to select generator commands (e.g. -r mockgen)")
}

var CmdMock = &cobra.Command{
	Use:     "mock [path...]",
	Short:   "Generate mock code using mockgen and go:generate",
	Long:    "kun mock executes //go:generate directives to generate mock code.",
	Example: "  kun mock\n  kun mock ./internal/service/svc/...\n  kun mock ./internal/service/svc/demo.go\n  kun mock -r mockgen",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("mockgen"); err != nil {
			return fmt.Errorf("mockgen is not installed or not in PATH, please install it: go install %s", config.MockgenUrl)
		}

		cmdArgs, programArgs := helper.SplitArgs(cmd, args)

		genArgs := []string{"generate"}
		if runRegex != "" {
			genArgs = append(genArgs, "-run="+runRegex)
		}
		if verbose {
			genArgs = append(genArgs, "-v")
		}
		if printCmd {
			genArgs = append(genArgs, "-x")
		}

		if len(cmdArgs) > 0 {
			genArgs = append(genArgs, cmdArgs...)
		} else {
			genArgs = append(genArgs, "./...")
		}

		if len(programArgs) > 0 {
			genArgs = append(genArgs, programArgs...)
		}

		output.Success("running: go %s", strings.Join(genArgs, " "))

		execCmd := exec.Command("go", genArgs...)
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
		if err := execCmd.Run(); err != nil {
			return fmt.Errorf("generate mock error: %w", err)
		}

		output.Success("mock generated successfully!")
		return nil
	},
}

// Register 将 mock 子命令挂载到 parent。
func Register(parent *cobra.Command) {
	parent.AddCommand(CmdMock)
}
