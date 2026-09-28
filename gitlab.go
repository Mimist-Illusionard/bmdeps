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
	baseURL string
	token   string
	client  *http.Client
}

type GitLabTag struct {
	Name string `json:"name"`
}

func NewGitLabClient(baseURL, token string, timeout time.Duration) *GitLabClient {
	return &GitLabClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client:  &http.Client{Timeout: timeout},
	}
}

func (c *GitLabClient) ListTags(ctx context.Context, project string) ([]GitLabTag, error) {
	projectID := url.PathEscape(project)
	var all []GitLabTag

	for page := 1; ; page++ {
		endpoint := fmt.Sprintf("%s/api/v4/projects/%s/repository/tags?per_page=100&page=%d", c.baseURL, projectID, page)
		resp, err := c.do(ctx, http.MethodGet, endpoint)
		if err != nil {
			return nil, err
		}

		var tags []GitLabTag
		decodeErr := json.NewDecoder(resp.Body).Decode(&tags)
		nextPage := resp.Header.Get("X-Next-Page")
		resp.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode tags for %s: %w", project, decodeErr)
		}
		all = append(all, tags...)

		if nextPage == "" {
			break
		}
		if _, err := strconv.Atoi(nextPage); err != nil {
			break
		}
	}
	return all, nil
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
	req.Header.Set("PRIVATE-TOKEN", c.token)
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
