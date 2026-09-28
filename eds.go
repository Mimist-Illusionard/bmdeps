package main

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
)

var (
	runRE           = regexp.MustCompile(`(?i)^\s*run\s+["']?([^"'\s]+)["']?`)
	dependencyRE    = regexp.MustCompile(`(?i)assert-dependency\s+should-be\s+([0-9]+)\s+name\s+["']([^"']+)["']`)
	moduleVersionRE = regexp.MustCompile(`(?i)assert-module-version\s+should-be\s+([0-9]+)\s+module-name\s+["']([^"']+)["']`)
	versionRunRE    = regexp.MustCompile(`(?i)^/?version\.([0-9]+)/install\.eds$`)
	numericRunRE    = regexp.MustCompile(`(?i)^/?([0-9]+)/install\.eds$`)
)

type DBDependency struct {
	Module  string `json:"module"`
	Version int    `json:"version"`
}

type InstallerInfo struct {
	Module        string
	ModuleVersion int
	Dependencies  []DBDependency
}

func LatestVersionInstaller(rootInstall []byte, dataModelPath string) (string, error) {
	var last string
	for _, target := range activeRuns(rootInstall) {
		if versionRunRE.MatchString(target) {
			last = target
		}
	}
	if last == "" {
		return "", fmt.Errorf("no active run /version.<n>/install.eds in %s/install.eds", dataModelPath)
	}
	return path.Join(dataModelPath, strings.TrimPrefix(last, "/")), nil
}

func ParseInstaller(data []byte) InstallerInfo {
	var info InstallerInfo
	for _, line := range activeLines(data) {
		if m := dependencyRE.FindStringSubmatch(line); m != nil {
			v, _ := strconv.Atoi(m[1])
			info.Dependencies = append(info.Dependencies, DBDependency{Module: m[2], Version: v})
		}
		if m := moduleVersionRE.FindStringSubmatch(line); m != nil {
			info.ModuleVersion, _ = strconv.Atoi(m[1])
			info.Module = m[2]
		}
	}
	return info
}

func LatestNestedInstaller(groupInstall []byte, groupInstallPath string) (string, bool) {
	var last string
	for _, target := range activeRuns(groupInstall) {
		if numericRunRE.MatchString(target) {
			last = target
		}
	}
	if last == "" {
		return "", false
	}
	return path.Join(path.Dir(groupInstallPath), strings.TrimPrefix(last, "/")), true
}

func activeRuns(data []byte) []string {
	var out []string
	for _, line := range activeLines(data) {
		m := runRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		out = append(out, strings.TrimSpace(m[1]))
	}
	return out
}

func activeLines(data []byte) []string {
	raw := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}
