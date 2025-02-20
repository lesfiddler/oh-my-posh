package cli

import (
	"fmt"

	"github.com/jandedobbeleer/oh-my-posh/src/cache"
	"github.com/jandedobbeleer/oh-my-posh/src/config"
	"github.com/jandedobbeleer/oh-my-posh/src/runtime"
	"github.com/jandedobbeleer/oh-my-posh/src/upgrade"

	"github.com/spf13/cobra"
)

var (
	toggleUse  = "%s [notice]"
	toggleLong = `%s a feature

This command is used to %s one of the following features:

- notice`
	toggleArgs = []string{
		"notice",
		"autoupgrade",
	}
)

// getCmd represents the get command
var enableCmd = &cobra.Command{
	Use:       fmt.Sprintf(toggleUse, "enable"),
	Short:     "Enable a feature",
	Long:      fmt.Sprintf(toggleLong, "Enable", "enable"),
	ValidArgs: toggleArgs,
	Args:      NoArgsOrOneValidArg,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			_ = cmd.Help()
			return
		}
		toggleFeature(cmd, args[0], true)
	},
}

func init() {
	RootCmd.AddCommand(enableCmd)
}

func toggleFeature(cmd *cobra.Command, feature string, enable bool) {
	env := &runtime.Terminal{
		CmdFlags: &runtime.Flags{
			Shell:     shellName,
			SaveCache: true,
		},
	}

	env.Init()
	defer env.Close()

	switch feature {
	case "notice":
		if enable {
			env.Cache().Delete(upgrade.CACHEKEY)
			return
		}

		env.Cache().Set(upgrade.CACHEKEY, "disabled", cache.INFINITE)
	case "autoupgrade":
		if enable {
			env.Cache().Set(config.AUTOUPGRADE, "true", cache.INFINITE)
			return
		}

		env.Cache().Delete(config.AUTOUPGRADE)
	default:
		_ = cmd.Help()
	}
}
