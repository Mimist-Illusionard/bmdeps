package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func WriteCatalogFiles(c Catalog, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	jsonData, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "catalog.json"), append(jsonData, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "graph.dot"), []byte(CatalogDOT(c)), 0o644); err != nil {
		return err
	}
	return nil
}

func LoadCatalog(filePath string) (Catalog, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return Catalog{}, err
	}
	var c Catalog
	if err := json.Unmarshal(data, &c); err != nil {
		return Catalog{}, err
	}
	return c, nil
}

func WriteStandReport(report StandReport, filePath string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, append(data, '\n'), 0o644)
}

func CatalogDOT(c Catalog) string {
	var b strings.Builder
	b.WriteString("digraph bmdeps {\n")
	b.WriteString("  rankdir=LR;\n")
	b.WriteString("  node [shape=box];\n")

	releases := append([]Release(nil), c.Releases...)
	sort.Slice(releases, func(i, j int) bool { return releaseID(releases[i]) < releaseID(releases[j]) })
	for _, r := range releases {
		label := fmt.Sprintf("%s\\nrelease %s\\nDB %d", dotEscape(r.Module), dotEscape(r.Release), r.DBVersion)
		b.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\"];\n", dotEscape(releaseID(r)), label))
	}

	constraintNodes := map[string]bool{}
	for _, src := range releases {
		for _, dep := range src.Dependencies {
			if isInternalDBDependency(src.Module, dep.Module) {
				continue
			}
			constraint := fmt.Sprintf("db:%s:%d", normalizeModule(dep.Module), dep.Version)
			if !constraintNodes[constraint] {
				constraintNodes[constraint] = true
				label := fmt.Sprintf("%s DB >= %d", normalizeModule(dep.Module), dep.Version)
				b.WriteString(fmt.Sprintf("  \"%s\" [shape=ellipse,label=\"%s\"];\n", dotEscape(constraint), dotEscape(label)))
			}
			b.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [label=\"requires\"];\n", dotEscape(releaseID(src)), dotEscape(constraint)))

			matches := c.compatibleReleases(dep.Module, dep.Version)
			for _, target := range matches {
				b.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [style=dashed,label=\"provided by\"];\n", dotEscape(constraint), dotEscape(releaseID(target))))
			}
		}
	}
	b.WriteString("}\n")
	return b.String()
}

func dotEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	return strings.ReplaceAll(s, "\"", "\\\"")
}

func PrintScanSummary(c Catalog) {
	projects := map[string]bool{}
	modules := map[string]bool{}
	for _, r := range c.Releases {
		projects[r.Project] = true
		modules[r.Module] = true
	}
	fmt.Printf("Scanned releases: %d across %d projects / %d EDS modules\n", len(c.Releases), len(projects), len(modules))
	if len(c.Errors) > 0 {
		fmt.Printf("Scan warnings/errors: %d (see catalog.json)\n", len(c.Errors))
	}
}

func PrintCatalog(c Catalog, moduleFilter string) {
	moduleFilter = normalizeModule(moduleFilter)
	for _, r := range c.Releases {
		if moduleFilter != "" && normalizeModule(r.Module) != moduleFilter {
			continue
		}
		fmt.Printf("%-12s %-14s DB %-8d tag=%-14s project=%s\n", r.Module, r.Release, r.DBVersion, r.Tag, r.Project)
	}
}

func PrintStandReport(report StandReport) {
	name := report.StandArtifactID
	if name == "" {
		name = "stand"
	}
	fmt.Printf("Checking %s %s\n\n", name, report.StandVersion)
	fmt.Println("Selected BM releases")
	fmt.Printf("%-12s %-14s %-10s %-18s\n", "MODULE", "RELEASE", "DB", "STATUS")
	for _, s := range report.Selected {
		db := "-"
		if s.DBVersion != 0 {
			db = fmt.Sprint(s.DBVersion)
		}
		fmt.Printf("%-12s %-14s %-10s %-18s\n", s.Module, s.Release, db, strings.ToUpper(s.Status))
		if s.Status == CheckUnknownModule {
			fmt.Printf("  ! artifact %s is not represented by a project in the catalog\n", s.ArtifactID)
		}
	}

	if len(report.Checks) > 0 {
		fmt.Println("\nEDS dependency checks")
		for _, c := range report.Checks {
			mark := "✓"
			switch c.Status {
			case CheckWarning:
				mark = "!"
			case CheckOK:
				mark = "✓"
			default:
				mark = "✗"
			}
			fmt.Printf("%s %s %s [DB %d] -> %s DB >= %d", mark, c.SourceModule, c.SourceRelease, c.SourceDBVersion, c.TargetModule, c.RequiredDBVersion)
			if c.SelectedRelease != "" {
				fmt.Printf("; stand has %s %s", c.TargetModule, c.SelectedRelease)
				if c.ActualDBVersion != 0 {
					fmt.Printf(" [DB %d]", c.ActualDBVersion)
				}
			}
			fmt.Printf("  [%s]\n", strings.ToUpper(c.Status))
			if c.Detail != "" {
				fmt.Printf("  %s\n", c.Detail)
			}
			if len(c.Compatible) > 0 && c.Status != CheckOK && c.Status != CheckWarning {
				fmt.Printf("  compatible %s releases for DB >= %d: %s\n", c.TargetModule, c.RequiredDBVersion, limitedJoin(c.Compatible, 10))
			}
		}
	}

	if len(report.Conflicts) > 0 {
		fmt.Println("\nRequirement conflicts")
		for _, conflict := range report.Conflicts {
			fmt.Printf("✗ %s is required at more than one DB version:\n", conflict.TargetModule)
			for _, req := range conflict.Requirements {
				fmt.Printf("  %s %s -> DB %d\n", req.SourceModule, req.SourceRelease, req.DBVersion)
			}
		}
	}

	fmt.Println()
	if report.Compatible {
		if len(report.Warnings) > 0 {
			fmt.Println("RESULT: COMPATIBLE (WITH WARNINGS)")
		} else {
			fmt.Println("RESULT: COMPATIBLE")
		}
	} else {
		fmt.Println("RESULT: INCOMPATIBLE")
	}
}

func limitedJoin(values []string, max int) string {
	if len(values) <= max {
		return strings.Join(values, ", ")
	}
	return strings.Join(values[:max], ", ") + fmt.Sprintf(" ... (+%d more)", len(values)-max)
}
