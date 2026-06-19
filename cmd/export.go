package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const migrateNote = `# Migration Notes

Some Rancher v1.x organizational concepts have changed in Rancher v2.x:

| v1.6          | v2.x        |
|---------------|-------------|
| environment   | project     |
| stack         | namespace   |
| service       | workload    |

Recommended approach:
- Create one Rancher v2.x **project** per v1.6 environment.
- Create one **namespace** per v1.6 stack inside that project.
- Use the generated docker-compose.yml with kompose or Helm to deploy workloads.
`

func newExportCmd() *cobra.Command {
	var apiURL, accessKey, secretKey, exportDir string
	var includeAll, includeSystem, insecure bool

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export compose files for all cattle stacks from a Rancher v1.6 API",
		Example: `  migration-tools export \
    --url https://rancher.example/v2-beta \
    --access-key myAccessKey \
    --secret-key mySecretKey \
    --export-dir ./export`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if apiURL == "" || accessKey == "" || secretKey == "" {
				return fmt.Errorf("--url, --access-key and --secret-key are required")
			}
			if err := ensureEmpty(exportDir); err != nil {
				return err
			}
			c := NewRancherClient(apiURL, accessKey, secretKey, insecure)
			stacks, err := c.ListStacks(includeAll, includeSystem)
			if err != nil {
				return fmt.Errorf("listing stacks: %w", err)
			}
			projectNames := map[string]string{}
			var stackCount, projectCount int
			for _, s := range stacks {
				if _, ok := projectNames[s.AccountID]; !ok {
					p, err := c.GetProject(s.AccountID)
					if err != nil {
						return fmt.Errorf("fetching project %s: %w", s.AccountID, err)
					}
					if strings.EqualFold(p.Orchestration, "cattle") {
						projectNames[s.AccountID] = p.Name
						projectCount++
					} else {
						projectNames[s.AccountID] = ""
					}
				}
				if projectNames[s.AccountID] == "" {
					continue // skip non-cattle environments
				}
				cfg, err := c.ExportConfig(s.ID, s.ServiceIDs)
				if err != nil {
					return fmt.Errorf("exporting stack %s/%s: %w", projectNames[s.AccountID], s.Name, err)
				}
				dir := filepath.Join(exportDir, safeName(projectNames[s.AccountID]), safeName(s.Name))
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return err
				}
				for fname, content := range map[string]string{
					"docker-compose.yml":  cfg.DockerComposeConfig,
					"rancher-compose.yml": cfg.RancherComposeConfig,
					"README.md":           migrateNote,
				} {
					if err := os.WriteFile(filepath.Join(dir, fname), []byte(content), 0o644); err != nil {
						return err
					}
				}
				fmt.Printf("  exported: %s / %s\n", projectNames[s.AccountID], s.Name)
				stackCount++
			}
			fmt.Printf("\nexported %d stack(s) from %d project(s) → %s\n", stackCount, projectCount, exportDir)
			return nil
		},
	}

	cmd.Flags().StringVar(&apiURL, "url", "", "Rancher v1.6 API endpoint (e.g. https://rancher.example/v2-beta)")
	cmd.Flags().StringVar(&accessKey, "access-key", "", "Rancher API access key")
	cmd.Flags().StringVar(&secretKey, "secret-key", "", "Rancher API secret key")
	cmd.Flags().StringVar(&exportDir, "export-dir", "export", "Output directory for exported stacks (must be empty or new)")
	cmd.Flags().BoolVar(&includeAll, "all", false, "Include inactive, stopped and removing stacks")
	cmd.Flags().BoolVar(&includeSystem, "system", false, "Include system/infrastructure stacks")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "Skip TLS certificate verification")

	_ = cmd.MarkFlagRequired("url")
	_ = cmd.MarkFlagRequired("access-key")
	_ = cmd.MarkFlagRequired("secret-key")

	return cmd
}
