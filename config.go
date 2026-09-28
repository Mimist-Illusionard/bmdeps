package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	GitLabURL     string
	GitLabToken   string
	Projects      []string
	TagRegex      *regexp.Regexp
	DataModelPath string
	Concurrency   int
	HTTPTimeout   time.Duration
}

func LoadConfig(path string) (Config, error) {
	values, err := readEnvFile(path)
	if err != nil {
		return Config{}, err
	}

	// Real environment variables override values from the file.
	get := func(key, def string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		if v := values[key]; v != "" {
			return v
		}
		return def
	}

	cfg := Config{
		GitLabURL:     strings.TrimRight(get("GITLAB_URL", ""), "/"),
		GitLabToken:   get("GITLAB_TOKEN", ""),
		Projects:      splitCSV(get("PROJECTS", "")),
		DataModelPath: strings.Trim(get("DATA_MODEL_PATH", "dataModel"), "/"),
		Concurrency:   6,
		HTTPTimeout:   20 * time.Second,
	}

	if cfg.GitLabURL == "" {
		return Config{}, fmt.Errorf("GITLAB_URL is required")
	}
	if cfg.GitLabToken == "" {
		return Config{}, fmt.Errorf("GITLAB_TOKEN is required")
	}
	if len(cfg.Projects) == 0 {
		return Config{}, fmt.Errorf("PROJECTS is required")
	}

	tagPattern := get("TAG_REGEX", `^v?[0-9]+\.[0-9]+\.[0-9]+$`)
	cfg.TagRegex, err = regexp.Compile(tagPattern)
	if err != nil {
		return Config{}, fmt.Errorf("invalid TAG_REGEX: %w", err)
	}

	if raw := get("CONCURRENCY", ""); raw != "" {
		n, convErr := strconv.Atoi(raw)
		if convErr != nil || n < 1 {
			return Config{}, fmt.Errorf("CONCURRENCY must be a positive integer")
		}
		cfg.Concurrency = n
	}

	if raw := get("HTTP_TIMEOUT", ""); raw != "" {
		d, parseErr := time.ParseDuration(raw)
		if parseErr != nil {
			return Config{}, fmt.Errorf("invalid HTTP_TIMEOUT: %w", parseErr)
		}
		cfg.HTTPTimeout = d
	}

	return cfg, nil
}

func readEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open env file %q: %w", path, err)
	}
	defer f.Close()

	out := make(map[string]string)
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected KEY=VALUE", path, lineNo)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			return nil, fmt.Errorf("%s:%d: empty key", path, lineNo)
		}
		out[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
