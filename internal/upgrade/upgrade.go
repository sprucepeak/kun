package upgrade

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/sprucepeak/kun/config"
	"github.com/sprucepeak/kun/pkg/output"
)

var CmdUpgrade = &cobra.Command{
	Use:     "upgrade",
	Short:   "Upgrade the kun command.",
	Long:    "Upgrade the kun command.",
	Example: "kun upgrade",
	RunE: func(_ *cobra.Command, _ []string) error {
		output.Success("go install %s", config.KunUrl)
		cmd := exec.Command("go", "install", "-ldflags=-s -w", config.KunUrl)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("go install %s error: %w", config.KunUrl, err)
		}
		output.Success("kun upgrade successfully!")
		return nil
	},
}

// Register 将 upgrade 子命令挂载到 parent。
func Register(parent *cobra.Command) {
	parent.AddCommand(CmdUpgrade)
}
