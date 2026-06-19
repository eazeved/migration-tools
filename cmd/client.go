package cmd

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RancherClient is a minimal HTTP client for the Rancher v1.6 API.
type RancherClient struct {
	baseURL    string
	accessKey  string
	secretKey  string
	httpClient *http.Client
}

// NewRancherClient builds an authenticated Rancher API client.
func NewRancherClient(rawURL, accessKey, secretKey string, insecure bool) *RancherClient {
	tr := &http.Transport{}
	if insecure {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	return &RancherClient{
		baseURL:   strings.TrimRight(rawURL, "/"),
		accessKey: accessKey,
		secretKey: secretKey,
		httpClient: &http.Client{
			Timeout:   60 * time.Second,
			Transport: tr,
		},
	}
}

// ListStacks returns all cattle stacks visible to this API token.
func (c *RancherClient) ListStacks(includeAll, includeSystem bool) ([]Stack, error) {
	v := url.Values{}
	v.Set("limit", "-1")
	v.Set("system", boolStr(includeSystem))
	if includeAll {
		v.Set("all", "true")
	} else {
		v.Set("removed_null", "1")
		v.Add("state_ne", "inactive")
		v.Add("state_ne", "stopped")
		v.Add("state_ne", "removing")
	}
	resp, err := c.get("/stacks?" + v.Encode())
	if err != nil {
		return nil, err
	}
	var out ListResponse[Stack]
	if err := json.Unmarshal(resp, &out); err != nil {
		return nil, err
	}
	all := out.Data
	for out.Pagination != nil && out.Pagination.Next != "" {
		resp, err = c.getAbs(out.Pagination.Next)
		if err != nil {
			return nil, err
		}
		out = ListResponse[Stack]{}
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

// GetProject fetches a single Rancher v1.6 environment/project by ID.
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
func (c *RancherClient) ExportConfig(stackID string, serviceIDs []string) (*ComposeConfig, error) {
	payload, _ := json.Marshal(&ComposeInput{ServiceIDs: serviceIDs})
	resp, err := c.post("/stacks/"+stackID+"?action=exportconfig", payload)
	if err != nil {
		return nil, err
	}
	var cfg ComposeConfig
	if err := json.Unmarshal(resp, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
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
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s → %s: %s", method, target, resp.Status, string(data))
	}
	return data, nil
}
