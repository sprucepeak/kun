package check

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRegister(t *testing.T) {
	root := &cobra.Command{Use: "testroot"}
	Register(root)

	foundCheck := false
	for _, c := range root.Commands() {
		if c.Name() == "check" {
			foundCheck = true
			foundInit := false
			for _, sub := range c.Commands() {
				if sub.Name() == "init" {
					foundInit = true
					break
				}
			}
			if !foundInit {
				t.Fatalf("expected 'init' subcommand under 'check'")
			}
			break
		}
	}
	if !foundCheck {
		t.Fatalf("expected 'check' command to be registered")
	}
}

func TestCheckFlags(t *testing.T) {
	flags := []string{"lint", "race", "cve", "nil", "deep", "no-install"}
	for _, f := range flags {
		if CmdCheck.Flags().Lookup(f) == nil {
			t.Errorf("expected flag --%s to exist", f)
		}
	}
	if CmdCheck.Flags().Lookup("vuln") != nil {
		t.Errorf("flag --vuln should have been removed")
	}
	if CmdCheck.Flags().Lookup("all") != nil {
		t.Errorf("flag --all should have been removed")
	}
}
