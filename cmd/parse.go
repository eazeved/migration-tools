package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newParseCmd() *cobra.Command {
	var dockerFile, rancherFile, outputFile, komposeBin string

	cmd := &cobra.Command{
		Use:   "parse",
		Short: "Analyse compose files for migration blockers and generate Kubernetes manifests",
		Example: `  migration-tools parse \
    --docker-file docker-compose.yml \
    --rancher-file rancher-compose.yml \
    --output-file output.txt`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dc, err := loadCompose(dockerFile)
			if err != nil {
				return err
			}
			rc, _ := loadCompose(rancherFile)

			findings := analyse(dc, rc)
			if err := writeReport(outputFile, findings); err != nil {
				return err
			}
			fmt.Printf("migration analysis written → %s\n", outputFile)

			manifest, err := runKompose(komposeBin, dockerFile)
			if err != nil {
				fmt.Printf("note: %v\n", err)
			} else {
				fmt.Printf("Kubernetes manifest written → %s\n", manifest)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dockerFile, "docker-file", "docker-compose.yml", "Path to docker-compose.yml")
	cmd.Flags().StringVar(&rancherFile, "rancher-file", "rancher-compose.yml", "Path to rancher-compose.yml (optional)")
	cmd.Flags().StringVar(&outputFile, "output-file", "output.txt", "Path for the analysis report")
	cmd.Flags().StringVar(&komposeBin, "kompose-bin", "kompose", "Path to kompose binary (must be in PATH or explicit)")

	return cmd
}

func loadCompose(path string) (*ComposeFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var cf ComposeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if cf.Services == nil {
		cf.Services = map[string]map[string]any{}
	}
	return &cf, nil
}

func analyse(dc, rc *ComposeFile) []Finding {
	var findings []Finding

	names := make([]string, 0, len(dc.Services))
	for n := range dc.Services {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		svc := dc.Services[name]
		warn := func(key, advice string) {
			if _, ok := svc[key]; ok {
				findings = append(findings, Finding{
					Service: name, Level: "WARN",
					Issue:  key + " requires manual translation",
					Advice: advice,
				})
			}
		}
		warn("links", "Replace with Kubernetes Service DNS discovery.")
		warn("depends_on", "Kubernetes has no strict startup ordering; use init containers or readiness probes.")
		warn("network_mode", "host/container network modes require manual pod spec translation.")
		warn("pid", "PID namespace sharing requires explicit pod security review.")
		warn("privileged", "Avoid privileged where possible; use securityContext capabilities instead.")
		warn("devices", "Device mounts require privileged pods or device plugins.")
		warn("volumes_from", "Convert to explicit named volume mounts.")
		warn("external_links", "Use Kubernetes Services or ExternalName Services instead.")
		warn("restart", "Kubernetes controllers own restart; remove this field.")
		warn("build", "Build directives are not deployable; push images to a registry first.")
		warn("cap_add", "Translate to securityContext.capabilities.add.")
		warn("cap_drop", "Translate to securityContext.capabilities.drop.")

		if _, ok := svc["ports"]; !ok {
			findings = append(findings, Finding{
				Service: name, Level: "INFO",
				Issue:  "no ports declared",
				Advice: "Verify whether a Kubernetes Service (ClusterIP/NodePort/LoadBalancer) is needed.",
			})
		}
	}

	if rc != nil {
		for name, svc := range rc.Services {
			if _, ok := svc["health_check"]; ok {
				findings = append(findings, Finding{
					Service: name, Level: "WARN",
					Issue:  "rancher health_check must be translated",
					Advice: "Convert to readinessProbe and/or livenessProbe in your Deployment spec.",
				})
			}
			if _, ok := svc["scale"]; ok {
				findings = append(findings, Finding{
					Service: name, Level: "INFO",
					Issue:  "rancher scale found",
					Advice: "Set Deployment spec.replicas to this value.",
				})
			}
			if _, ok := svc["metadata"]; ok {
				findings = append(findings, Finding{
					Service: name, Level: "INFO",
					Issue:  "rancher metadata labels found",
					Advice: "Translate to Kubernetes labels/annotations on the Pod template.",
				})
			}
		}
	}

	if len(findings) == 0 {
		findings = append(findings, Finding{
			Service: "*", Level: "INFO",
			Issue:  "no obvious migration blockers found",
			Advice: "Run kompose output through manual Kubernetes review before production use.",
		})
	}
	return findings
}

func writeReport(path string, findings []Finding) error {
	var b strings.Builder
	b.WriteString("# Rancher Migration Analysis\n\n")
	var warns, infos []Finding
	for _, f := range findings {
		if f.Level == "WARN" || f.Level == "ERROR" {
			warns = append(warns, f)
		} else {
			infos = append(infos, f)
		}
	}
	if len(warns) > 0 {
		b.WriteString("## Action Required\n\n")
		for _, f := range warns {
			b.WriteString(fmt.Sprintf("**[%s] %s** — %s\n\n> %s\n\n", f.Level, f.Service, f.Issue, f.Advice))
		}
	}
	if len(infos) > 0 {
		b.WriteString("## Informational\n\n")
		for _, f := range infos {
			b.WriteString(fmt.Sprintf("**[%s] %s** — %s\n\n> %s\n\n", f.Level, f.Service, f.Issue, f.Advice))
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func runKompose(bin, dockerFile string) (string, error) {
	path, err := exec.LookPath(bin)
	if err != nil {
		return "", errors.New("kompose not found in PATH — install it or pass --kompose-bin to skip")
	}
	outFile := "k8s-manifests.yaml"
	out, err := exec.Command(path, "convert", "-f", dockerFile, "--stdout").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("kompose: %s", strings.TrimSpace(string(out)))
	}
	if err := os.WriteFile(outFile, out, 0o644); err != nil {
		return "", err
	}
	return outFile, nil
}
