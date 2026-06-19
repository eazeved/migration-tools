package cmd

// Stack represents a Rancher v1.6 stack object from the API.
type Stack struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	AccountID  string   `json:"accountId"`
	System     bool     `json:"system"`
	State      string   `json:"state"`
	ServiceIDs []string `json:"serviceIds"`
}

// Project represents a Rancher v1.6 environment/project.
type Project struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Orchestration string `json:"orchestration"`
}

// ComposeConfig holds the exported docker-compose and rancher-compose YAML strings.
type ComposeConfig struct {
	DockerComposeConfig  string `json:"dockerComposeConfig"`
	RancherComposeConfig string `json:"rancherComposeConfig"`
}

// ComposeInput is the request body for the Rancher exportconfig action.
type ComposeInput struct {
	ServiceIDs []string `json:"serviceIds"`
}

// ListResponse is a generic Rancher API paginated list response.
type ListResponse[T any] struct {
	Data       []T         `json:"data"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// Pagination describes the next-page link from a Rancher list response.
type Pagination struct {
	Next    string `json:"next,omitempty"`
	Partial bool   `json:"partial,omitempty"`
}

// ComposeFile is a minimal representation of a docker-compose.yml for analysis.
type ComposeFile struct {
	Version  any                       `yaml:"version,omitempty"`
	Services map[string]map[string]any `yaml:"services"`
	Volumes  map[string]map[string]any `yaml:"volumes,omitempty"`
	Networks map[string]map[string]any `yaml:"networks,omitempty"`
}

// Finding is a single migration issue found during compose analysis.
type Finding struct {
	Service string
	Level   string // INFO | WARN | ERROR
	Issue   string
	Advice  string
}
