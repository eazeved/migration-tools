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
	var envFilter, stackFilter string
	var includeAll, includeSystem, insecure bool

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export compose files for cattle stacks from a Rancher v1.6 API",
		Example: `  # Export everything
  migration-tools export --url http://rancher.example:8080 --access-key K --secret-key S

  # Export only one environment
  migration-tools export --url http://rancher.example:8080 --access-key K --secret-key S \
    --env "Default"

  # Export one specific stack
  migration-tools export --url http://rancher.example:8080 --access-key K --secret-key S \
    --env "Default" --stack "my-app"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureEmpty(exportDir); err != nil {
				return err
			}
			c := NewRancherClient(apiURL, accessKey, secretKey, insecure, Debug)

			// 1. Resolve environments.
			projects, err := c.ListProjects()
			if err != nil {
				return fmt.Errorf("listing environments: %w", err)
			}
			if Debug {
				fmt.Fprintf(os.Stderr, "[debug] found %d environment(s) total\n", len(projects))
				for _, p := range projects {
					fmt.Fprintf(os.Stderr, "[debug]   env id=%s name=%q orchestration=%s\n", p.ID, p.Name, p.Orchestration)
				}
			}

			// 2. Filter environments.
			var selected []Project
			for _, p := range projects {
				if envFilter != "" && !matchNameOrID(p.Name, p.ID, envFilter) {
					if Debug {
						fmt.Fprintf(os.Stderr, "[debug] skipping env %q (doesn't match --env %q)\n", p.Name, envFilter)
					}
					continue
				}
				if !strings.EqualFold(p.Orchestration, "cattle") && !strings.EqualFold(p.Orchestration, "") {
					if Debug {
						fmt.Fprintf(os.Stderr, "[debug] skipping env %q orchestration=%q (not cattle)\n", p.Name, p.Orchestration)
					}
					continue
				}
				selected = append(selected, p)
			}

			if len(selected) == 0 {
				if envFilter != "" {
					return fmt.Errorf("no cattle environment matched %q — run 'list' to see available environments", envFilter)
				}
				return fmt.Errorf("no cattle environments found — run 'list' to inspect what is available")
			}

			var stackCount, projectCount int
			for _, p := range selected {
				stacks, err := c.ListStacks(p.ID, includeAll, includeSystem)
				if err != nil {
					return fmt.Errorf("listing stacks for env %q: %w", p.Name, err)
				}
				if Debug {
					fmt.Fprintf(os.Stderr, "[debug] env %q has %d stack(s)\n", p.Name, len(stacks))
					for _, s := range stacks {
						fmt.Fprintf(os.Stderr, "[debug]   stack id=%s name=%q state=%s system=%v\n", s.ID, s.Name, s.State, s.System)
					}
				}

				var envExported int
				for _, s := range stacks {
					if stackFilter != "" && !matchNameOrID(s.Name, s.ID, stackFilter) {
						continue
					}
					cfg, err := c.ExportConfig(p.ID, s.ID, s.ServiceIDs)
					if err != nil {
						fmt.Printf("  [warn] export failed for %s/%s: %v\n", p.Name, s.Name, err)
						continue
					}
					dir := filepath.Join(exportDir, safeName(p.Name), safeName(s.Name))
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
					fmt.Printf("  ✓ exported: %s / %s → %s\n", p.Name, s.Name, dir)
					envExported++
					stackCount++
				}
				if envExported > 0 {
					projectCount++
				}
			}

			if stackCount == 0 {
				if stackFilter != "" {
					return fmt.Errorf("no stacks matched --stack %q — run 'list' to see available stacks", stackFilter)
				}
				fmt.Println("no stacks were exported — all environments may be empty or all stacks inactive")
				fmt.Println("tip: use --all to include inactive stacks, --system to include system stacks")
				fmt.Println("tip: run 'list' to inspect what is visible via this API key")
				return nil
			}

			fmt.Printf("\nexported %d stack(s) from %d environment(s) → %s\n", stackCount, projectCount, exportDir)
			return nil
		},
	}

	cmd.Flags().StringVar(&apiURL, "url", "", "Rancher v1.6 API endpoint")
	cmd.Flags().StringVar(&accessKey, "access-key", "", "Rancher API access key")
	cmd.Flags().StringVar(&secretKey, "secret-key", "", "Rancher API secret key")
	cmd.Flags().StringVar(&exportDir, "export-dir", "export", "Output directory (must be empty or new)")
	cmd.Flags().StringVar(&envFilter, "env", "", "Export only this environment (name or ID)")
	cmd.Flags().StringVar(&stackFilter, "stack", "", "Export only this stack (name or ID, requires --env)")
	cmd.Flags().BoolVar(&includeAll, "all", false, "Include inactive, stopped and removing stacks")
	cmd.Flags().BoolVar(&includeSystem, "system", false, "Include system/infrastructure stacks")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "Skip TLS certificate verification")

	_ = cmd.MarkFlagRequired("url")
	_ = cmd.MarkFlagRequired("access-key")
	_ = cmd.MarkFlagRequired("secret-key")

	return cmd
}
