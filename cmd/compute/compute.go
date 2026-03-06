package compute

import "github.com/spf13/cobra"

var Cmd = &cobra.Command{
	Use:     "compute",
	Aliases: []string{"c"},
	Short:   "Manage cloud compute resources",
	Args:    cobra.NoArgs,
}

func init() {
	Cmd.AddCommand(serverCmd)
	Cmd.AddCommand(volumeCmd)
	Cmd.AddCommand(firewallCmd)
	Cmd.AddCommand(imageCmd)
	Cmd.AddCommand(ipv4Cmd)
	Cmd.AddCommand(ipv6Cmd)
	Cmd.AddCommand(networkCmd)
	Cmd.AddCommand(packageCmd)
}
