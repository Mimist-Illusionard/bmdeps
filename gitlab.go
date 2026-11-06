package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type GitLabClient struct {
	baseURL    string
	token      string
	authHeader string
	client     *http.Client
}

type GitLabTag struct {
	Name string `json:"name"`
}

type GitLabProject struct {
	Name          string `json:"name"`
	NameNamespace string `json:"name_with_namespace"`
	PathNamespace string `json:"path_with_namespace"`
}

func NewGitLabClient(baseURL, token string, timeout time.Duration) *GitLabClient {
	return &GitLabClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		authHeader: "PRIVATE-TOKEN",
		client:     &http.Client{Timeout: timeout},
	}
}

// SetAuthHeader selects the GitLab authentication header. It defaults to
// PRIVATE-TOKEN; use JOB-TOKEN to authenticate with CI_JOB_TOKEN.
func (c *GitLabClient) SetAuthHeader(header string) {
	if header != "" {
		c.authHeader = header
	}
}

func (c *GitLabClient) ListTags(ctx context.Context, project string) ([]GitLabTag, error) {
	projectID := url.PathEscape(project)
	var all []GitLabTag

	for page := 1; page > 0; {
		endpoint := fmt.Sprintf("%s/api/v4/projects/%s/repository/tags?per_page=100&page=%d", c.baseURL, projectID, page)
		resp, err := c.do(ctx, http.MethodGet, endpoint)
		if err != nil {
			return nil, err
		}

		var tags []GitLabTag
		decodeErr := json.NewDecoder(resp.Body).Decode(&tags)
		next := nextPageNumber(resp)
		resp.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode tags for %s: %w", project, decodeErr)
		}
		all = append(all, tags...)
		page = next
	}
	return all, nil
}

// ListProjects returns every project of a group, including projects of
// subgroups, following GitLab pagination.
func (c *GitLabClient) ListProjects(ctx context.Context, groupID string) ([]GitLabProject, error) {
	var all []GitLabProject

	for page := 1; page > 0; {
		endpoint := fmt.Sprintf(
			"%s/api/v4/groups/%s/projects?per_page=100&page=%d&include_subgroups=true",
			c.baseURL, url.PathEscape(groupID), page,
		)
		resp, err := c.do(ctx, http.MethodGet, endpoint)
		if err != nil {
			return nil, err
		}

		var projects []GitLabProject
		decodeErr := json.NewDecoder(resp.Body).Decode(&projects)
		next := nextPageNumber(resp)
		resp.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode projects for %s: %w", groupID, decodeErr)
		}
		all = append(all, projects...)
		page = next
	}
	return all, nil
}

func nextPageNumber(resp *http.Response) int {
	raw := resp.Header.Get("X-Next-Page")
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0
	}
	return n
}

func (c *GitLabClient) RawFile(ctx context.Context, project, ref, filePath string) ([]byte, error) {
	projectID := url.PathEscape(project)
	escapedPath := url.PathEscape(strings.TrimLeft(filePath, "/"))
	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/repository/files/%s/raw?ref=%s", c.baseURL, projectID, escapedPath, url.QueryEscape(ref))

	resp, err := c.do(ctx, http.MethodGet, endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s@%s:%s: %w", project, ref, filePath, err)
	}
	return data, nil
}

func (c *GitLabClient) do(ctx context.Context, method, endpoint string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(c.authHeader, c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}

	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	return nil, fmt.Errorf("gitlab HTTP %d for %s: %s", resp.StatusCode, endpoint, strings.TrimSpace(string(body)))
}
