package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var apiURL, accessKey, secretKey string
	var includeSystem, insecure bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all environments and their stacks from a Rancher v1.6 API",
		Example: `  migration-tools list \
    --url http://rancher.example:8080 \
    --access-key myKey \
    --secret-key mySecret`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := NewRancherClient(apiURL, accessKey, secretKey, insecure, Debug)

			projects, err := c.ListProjects()
			if err != nil {
				return fmt.Errorf("listing environments: %w", err)
			}

			if len(projects) == 0 {
				fmt.Println("no environments found")
				return nil
			}

			fmt.Printf("Found %d environment(s):\n\n", len(projects))

			for _, p := range projects {
				orchestration := p.Orchestration
				if orchestration == "" {
					orchestration = "unknown"
				}
				fmt.Printf("  Environment: %-30s  id: %s  orchestration: %s\n",
					p.Name, p.ID, orchestration)

				stacks, err := c.ListStacks(p.ID, true, includeSystem)
				if err != nil {
					fmt.Printf("    [warn] could not list stacks: %v\n", err)
					continue
				}

				if len(stacks) == 0 {
					fmt.Printf("    (no stacks)\n")
					continue
				}

				for _, s := range stacks {
					system := ""
					if s.System {
						system = "  [system]"
					}
					fmt.Printf("    stack: %-30s  id: %s  state: %s%s\n",
						s.Name, s.ID, s.State, system)
				}
				fmt.Println()
			}

			fmt.Println(strings.Repeat("-", 60))
			fmt.Println("Use --env <name-or-id> in the export command to target a specific environment.")
			fmt.Println("Use --stack <name-or-id> to target a specific stack.")
			return nil
		},
	}

	cmd.Flags().StringVar(&apiURL, "url", "", "Rancher v1.6 API endpoint")
	cmd.Flags().StringVar(&accessKey, "access-key", "", "Rancher API access key")
	cmd.Flags().StringVar(&secretKey, "secret-key", "", "Rancher API secret key")
	cmd.Flags().BoolVar(&includeSystem, "system", false, "Include system/infrastructure stacks")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "Skip TLS certificate verification")

	_ = cmd.MarkFlagRequired("url")
	_ = cmd.MarkFlagRequired("access-key")
	_ = cmd.MarkFlagRequired("secret-key")

	return cmd
}
