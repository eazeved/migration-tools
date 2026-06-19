package cmd

import "github.com/spf13/cobra"

// Debug is the global debug flag shared by all commands.
var Debug bool

func NewRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "migration-tools",
		Short: "Rancher v1.6 → v2.x migration helper",
		Long: `migration-tools assists migration from Rancher v1.6 (cattle) to Rancher v2.x.

Commands:
  list    Show all environments and their stacks (use to discover IDs before exporting)
  export  Export compose files for cattle stacks
  parse   Analyse compose files for migration blockers

Some Rancher v1.x organizational concepts changed in v2.x:
  environments  → projects
  stacks        → namespaces
  services      → workloads`,
	}
	root.Version = version
	root.SetVersionTemplate("{{.Version}}\n")
	root.PersistentFlags().BoolVar(&Debug, "debug", false, "Enable debug output (HTTP requests, responses, internal decisions)")
	root.AddCommand(newListCmd())
	root.AddCommand(newExportCmd())
	root.AddCommand(newParseCmd())
	return root
}
