package cmd

import "github.com/spf13/cobra"

func NewRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "migration-tools",
		Short: "Rancher v1.6 → v2.x migration helper",
		Long: `migration-tools assists migration from Rancher v1.6 (cattle) to Rancher v2.x.

It can export compose files directly from a running Rancher v1.6 API and parse
those files to surface migration blockers, then optionally generate Kubernetes
manifests via kompose.

Some Rancher v1.x organizational concepts have changed in Rancher v2.x:
  environments  → projects
  stacks        → namespaces
  services      → workloads`,
	}
	root.Version = version
	root.SetVersionTemplate("{{.Version}}\n")
	root.AddCommand(newExportCmd())
	root.AddCommand(newParseCmd())
	return root
}
