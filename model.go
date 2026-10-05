package main

import (
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Release is one BM Git tag correlated with its DB version, EDS DB
// dependencies and the cs-eng-be release declared by this BM tag's pom.xml.
type Release struct {
	Project       string         `json:"project"`
	Tag           string         `json:"tag"`
	Release       string         `json:"release"`
	Module        string         `json:"module"`
	DBVersion     int            `json:"db_version"`
	EngBEVersion  string         `json:"engbe_version,omitempty"`
	InstallerPath string         `json:"installer_path"`
	Dependencies  []DBDependency `json:"dependencies,omitempty"`
}

// PlatformRelease is an available release tag of the platform backend project.
// No other platform repository is needed: BM pom.xml points directly to one of
// these cs-eng-be releases through the engbe.version property.
type PlatformRelease struct {
	Project string `json:"project"`
	Tag     string `json:"tag"`
	Release string `json:"release"`
}

type ScanError struct {
	Project string `json:"project"`
	Tag     string `json:"tag,omitempty"`
	Error   string `json:"error"`
}

type Catalog struct {
	GeneratedAt      time.Time         `json:"generated_at"`
	Releases         []Release         `json:"releases"`
	PlatformReleases []PlatformRelease `json:"platform_releases,omitempty"`
	Errors           []ScanError       `json:"scan_errors,omitempty"`
}

func NewCatalog(releases []Release, scanErrors []ScanError) Catalog {
	c := Catalog{
		GeneratedAt: time.Now().UTC(),
		Releases:    append([]Release(nil), releases...),
		Errors:      append([]ScanError(nil), scanErrors...),
	}
	c.sort()
	return c
}

func (c *Catalog) sort() {
	sort.Slice(c.Releases, func(i, j int) bool {
		if c.Releases[i].Module == c.Releases[j].Module {
			return versionLess(c.Releases[j].Release, c.Releases[i].Release)
		}
		return c.Releases[i].Module < c.Releases[j].Module
	})
	sort.Slice(c.PlatformReleases, func(i, j int) bool {
		return versionLess(c.PlatformReleases[j].Release, c.PlatformReleases[i].Release)
	})
}

func (c Catalog) modules() map[string]bool {
	out := make(map[string]bool)
	for _, r := range c.Releases {
		out[r.Module] = true
	}
	return out
}

func (c Catalog) releasesByModule(module string) []Release {
	module = normalizeModule(module)
	var out []Release
	for _, r := range c.Releases {
		if normalizeModule(r.Module) == module {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return versionLess(out[j].Release, out[i].Release) })
	return out
}

func (c Catalog) findRelease(module, release string) []Release {
	module = normalizeModule(module)
	release = normalizeVersion(release)
	var out []Release
	for _, r := range c.Releases {
		if normalizeModule(r.Module) == module && normalizeVersion(r.Release) == release {
			out = append(out, r)
		}
	}
	return out
}

func (c Catalog) compatibleReleases(module string, dbVersion int) []Release {
	module = normalizeModule(module)
	var out []Release
	for _, r := range c.Releases {
		if normalizeModule(r.Module) == module && r.DBVersion >= dbVersion {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return versionLess(out[j].Release, out[i].Release) })
	return out
}

func (c Catalog) hasPlatformRelease(release string) bool {
	release = normalizeVersion(release)
	for _, r := range c.PlatformReleases {
		if normalizeVersion(r.Release) == release {
			return true
		}
	}
	return false
}

func normalizeModule(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func normalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

func releaseID(r Release) string {
	return r.Project + "@" + r.Tag
}

func shortProject(project string) string {
	return path.Base(project)
}

func isInternalDBDependency(module, dep string) bool {
	module = normalizeModule(module)
	dep = normalizeModule(dep)
	return dep == module || strings.HasPrefix(dep, module+".")
}

func versionLess(a, b string) bool {
	aa := strings.Split(normalizeVersion(a), ".")
	bb := strings.Split(normalizeVersion(b), ".")
	n := len(aa)
	if len(bb) > n {
		n = len(bb)
	}
	for i := 0; i < n; i++ {
		var x, y string
		if i < len(aa) {
			x = aa[i]
		}
		if i < len(bb) {
			y = bb[i]
		}
		if x == y {
			continue
		}
		xi, xerr := strconv.Atoi(x)
		yi, yerr := strconv.Atoi(y)
		if xerr == nil && yerr == nil {
			return xi < yi
		}
		return x < y
	}
	return a < b
}
