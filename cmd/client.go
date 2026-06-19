package cmd

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// RancherClient is a minimal HTTP client for the Rancher v1.6 API.
type RancherClient struct {
	baseURL    string
	accessKey  string
	secretKey  string
	httpClient *http.Client
	debug      bool
}

// NewRancherClient builds an authenticated Rancher API client.
// rawURL is normalised: any trailing /schemas, /v1, /v2-beta path is stripped
// so that both "http://host:8080" and "http://host:8080/v2-beta/schemas" work.
func NewRancherClient(rawURL, accessKey, secretKey string, insecure, debug bool) *RancherClient {
	base := normaliseURL(rawURL)
	if debug {
		fmt.Fprintf(os.Stderr, "[debug] base URL resolved to: %s\n", base)
	}
	tr := &http.Transport{}
	if insecure {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	return &RancherClient{
		baseURL:   base,
		accessKey: accessKey,
		secretKey: secretKey,
		debug:     debug,
		httpClient: &http.Client{
			Timeout:   60 * time.Second,
			Transport: tr,
		},
	}
}

// normaliseURL strips well-known suffixes so users can pass the full API URL
// copied from a browser (e.g. http://host:8080/v2-beta/schemas) or just the
// host (e.g. http://host:8080) and both will resolve to the right base.
func normaliseURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return strings.TrimRight(raw, "/")
	}
	p := u.Path
	for _, suffix := range []string{"/schemas", "/v2-beta", "/v1"} {
		if strings.HasSuffix(p, suffix) {
			p = p[:len(p)-len(suffix)]
		}
	}
	// Always append /v2-beta so all relative paths are consistent.
	u.Path = strings.TrimRight(p, "/") + "/v2-beta"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// ListProjects returns all Rancher v1.6 environments/projects.
func (c *RancherClient) ListProjects() ([]Project, error) {
	return paginate[Project](c, "/projects?limit=200")
}

// ListStacks returns stacks. When projectID is non-empty only stacks for that
// project are fetched using the project-scoped endpoint.
func (c *RancherClient) ListStacks(projectID string, includeAll, includeSystem bool) ([]Stack, error) {
	v := url.Values{}
	v.Set("limit", "200")
	if !includeSystem {
		v.Set("system", "false")
	}
	if !includeAll {
		v.Add("state", "active")
	}
	var path string
	if projectID != "" {
		path = "/projects/" + projectID + "/stacks?" + v.Encode()
	} else {
		path = "/stacks?" + v.Encode()
	}
	return paginate[Stack](c, path)
}

// GetProject fetches a single project by ID.
func (c *RancherClient) GetProject(id string) (*Project, error) {
	resp, err := c.get("/projects/" + id)
	if err != nil {
		return nil, err
	}
	var p Project
	if err := json.Unmarshal(resp, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// ExportConfig calls the Rancher exportconfig action for a stack.
func (c *RancherClient) ExportConfig(projectID, stackID string, serviceIDs []string) (*ComposeConfig, error) {
	payload, _ := json.Marshal(&ComposeInput{ServiceIDs: serviceIDs})
	// Try project-scoped endpoint first, fall back to global.
	path := "/projects/" + projectID + "/stacks/" + stackID + "?action=exportconfig"
	resp, err := c.post(path, payload)
	if err != nil {
		c.debugf("project-scoped export failed (%v), falling back to global endpoint", err)
		resp, err = c.post("/stacks/"+stackID+"?action=exportconfig", payload)
		if err != nil {
			return nil, err
		}
	}
	var cfg ComposeConfig
	if err := json.Unmarshal(resp, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// paginate[T] fetches all pages of a Rancher list endpoint.
func paginate[T any](c *RancherClient, path string) ([]T, error) {
	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	var out ListResponse[T]
	if err := json.Unmarshal(resp, &out); err != nil {
		return nil, fmt.Errorf("decoding response: %w\nbody: %s", err, string(resp))
	}
	all := out.Data
	for out.Pagination != nil && out.Pagination.Next != "" {
		resp, err = c.getAbs(out.Pagination.Next)
		if err != nil {
			return nil, err
		}
		out = ListResponse[T]{}
		if err := json.Unmarshal(resp, &out); err != nil {
			return nil, err
		}
		all = append(all, out.Data...)
		if !out.Pagination.Partial {
			break
		}
	}
	return all, nil
}

func (c *RancherClient) get(path string) ([]byte, error) {
	return c.do(http.MethodGet, c.baseURL+path, nil)
}
func (c *RancherClient) getAbs(rawURL string) ([]byte, error) {
	return c.do(http.MethodGet, rawURL, nil)
}
func (c *RancherClient) post(path string, body []byte) ([]byte, error) {
	return c.do(http.MethodPost, c.baseURL+path, body)
}

func (c *RancherClient) do(method, target string, body []byte) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
		c.debugf("--> %s %s  body: %s", method, target, string(body))
	} else {
		c.debugf("--> %s %s", method, target)
	}
	req, err := http.NewRequest(method, target, reader)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.accessKey, c.secretKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.debugf("<-- ERROR: %v", err)
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	c.debugf("<-- %s  body: %s", resp.Status, truncate(string(data), 512))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s → %s: %s", method, target, resp.Status, string(data))
	}
	return data, nil
}

func (c *RancherClient) debugf(format string, args ...any) {
	if c.debug {
		fmt.Fprintf(os.Stderr, "[debug] "+format+"\n", args...)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + fmt.Sprintf(" …[%d bytes total]", len(s))
}
