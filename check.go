package main

import (
	"fmt"
	"sort"
	"strings"
)

const (
	CheckOK             = "ok"
	CheckWarning        = "warning"
	CheckMismatch       = "mismatch"
	CheckMissingModule  = "missing-module"
	CheckMissingRelease = "missing-release"
	CheckUnknownModule  = "unknown-module"
	CheckAmbiguous      = "ambiguous-release"
)

type SelectedModule struct {
	Module     string   `json:"module"`
	ArtifactID string   `json:"artifact_id"`
	Release    string   `json:"release"`
	DBVersion  int      `json:"db_version,omitempty"`
	Status     string   `json:"status"`
	Project    string   `json:"project,omitempty"`
	Tag        string   `json:"tag,omitempty"`
	Candidates []string `json:"candidates,omitempty"`
}

type DependencyCheck struct {
	SourceModule      string   `json:"source_module"`
	SourceRelease     string   `json:"source_release"`
	SourceDBVersion   int      `json:"source_db_version"`
	TargetModule      string   `json:"target_module"`
	RequiredDBVersion int      `json:"required_db_version"`
	SelectedRelease   string   `json:"selected_release,omitempty"`
	ActualDBVersion   int      `json:"actual_db_version,omitempty"`
	Status            string   `json:"status"`
	Compatible        []string `json:"compatible_releases,omitempty"`
	Detail            string   `json:"detail,omitempty"`
}

type RequirementConflict struct {
	TargetModule string                `json:"target_module"`
	Requirements []ConflictRequirement `json:"requirements"`
}

type ConflictRequirement struct {
	SourceModule  string `json:"source_module"`
	SourceRelease string `json:"source_release"`
	DBVersion     int    `json:"db_version"`
}

type StandReport struct {
	StandArtifactID string                `json:"stand_artifact_id"`
	StandVersion    string                `json:"stand_version"`
	Compatible      bool                  `json:"compatible"`
	Selected        []SelectedModule      `json:"selected_modules"`
	Checks          []DependencyCheck     `json:"dependency_checks"`
	Conflicts       []RequirementConflict `json:"requirement_conflicts,omitempty"`
	Warnings        []string              `json:"warnings,omitempty"`
}

func CheckStand(c Catalog, pom PomInfo, artifactPrefix string) StandReport {
	if artifactPrefix == "" {
		artifactPrefix = "cs-"
	}

	report := StandReport{
		StandArtifactID: pom.ArtifactID,
		StandVersion:    pom.Version,
		Compatible:      true,
	}

	modules := c.modules()
	aliases := buildModuleAliases(c, artifactPrefix)
	selectedByModule := map[string]SelectedModule{}
	selectedReleaseByModule := map[string]Release{}

	for _, dep := range pom.Dependencies {
		module, known := aliases[strings.ToLower(dep.ArtifactID)]
		if !known {
			// The stand POM supplied for this project uses cs-* for BM artifacts.
			// Call out a cs-* dependency missing from PROJECTS instead of silently
			// pretending it was verified.
			if strings.HasPrefix(strings.ToLower(dep.ArtifactID), strings.ToLower(artifactPrefix)) {
				module = normalizeModule(strings.TrimPrefix(dep.ArtifactID, artifactPrefix))
				sm := SelectedModule{
					Module: module, ArtifactID: dep.ArtifactID, Release: dep.Version,
					Status: CheckUnknownModule,
				}
				report.Selected = append(report.Selected, sm)
				selectedByModule[module] = sm
				report.Compatible = false
			}
			continue
		}

		candidates := c.findRelease(module, dep.Version)
		sm := SelectedModule{Module: module, ArtifactID: dep.ArtifactID, Release: dep.Version}
		switch len(candidates) {
		case 0:
			sm.Status = CheckMissingRelease
			report.Compatible = false
		case 1:
			r := candidates[0]
			sm.Status = CheckOK
			sm.DBVersion = r.DBVersion
			sm.Project = r.Project
			sm.Tag = r.Tag
			selectedReleaseByModule[module] = r
		default:
			sm.Status = CheckAmbiguous
			for _, r := range candidates {
				sm.Candidates = append(sm.Candidates, r.Project+"@"+r.Tag)
			}
			report.Compatible = false
		}
		report.Selected = append(report.Selected, sm)
		selectedByModule[module] = sm
	}

	// Build dependency checks only from EDS. pom.xml of the BM projects is not
	// involved: the stand POM merely selects releases.
	for sourceModule, sourceRelease := range selectedReleaseByModule {
		for _, dep := range sourceRelease.Dependencies {
			targetModule := normalizeModule(dep.Module)
			if isInternalDBDependency(sourceModule, targetModule) {
				continue
			}

			check := DependencyCheck{
				SourceModule: sourceModule, SourceRelease: sourceRelease.Release,
				SourceDBVersion: sourceRelease.DBVersion,
				TargetModule:    targetModule, RequiredDBVersion: dep.Version,
			}

			if !modules[targetModule] {
				check.Status = CheckUnknownModule
				check.Detail = "EDS dependency points to a module that is not present in the scanned catalog"
				report.Checks = append(report.Checks, check)
				report.Compatible = false
				continue
			}

			targetSelected, exists := selectedByModule[targetModule]
			if !exists {
				check.Status = CheckMissingModule
				check.Compatible = releaseNames(c.compatibleReleases(targetModule, dep.Version))
				check.Detail = "required BM is not selected in the stand pom.xml"
				if len(check.Compatible) == 0 {
					check.Detail += fmt.Sprintf("; no scanned %s release provides DB >= %d", targetModule, dep.Version)
				}
				report.Checks = append(report.Checks, check)
				report.Compatible = false
				continue
			}

			check.SelectedRelease = targetSelected.Release
			if targetSelected.Status != CheckOK {
				check.Status = targetSelected.Status
				check.Compatible = releaseNames(c.compatibleReleases(targetModule, dep.Version))
				check.Detail = "selected stand release could not be resolved in the catalog"
				if len(check.Compatible) == 0 {
					check.Detail += fmt.Sprintf("; no scanned %s release provides DB >= %d", targetModule, dep.Version)
				}
				report.Checks = append(report.Checks, check)
				report.Compatible = false
				continue
			}

			check.ActualDBVersion = targetSelected.DBVersion
			switch {
			case targetSelected.DBVersion == dep.Version:
				check.Status = CheckOK
			case targetSelected.DBVersion > dep.Version:
				// assert-dependency is treated as a minimum DB requirement.
				// A newer DB is allowed, but is shown as a warning.
				check.Status = CheckWarning
				check.Detail = fmt.Sprintf("selected release provides newer DB %d; minimum required DB is %d", targetSelected.DBVersion, dep.Version)
				report.Warnings = append(report.Warnings, fmt.Sprintf(
					"%s %s requires %s DB >= %d; stand has DB %d",
					sourceModule, sourceRelease.Release, targetModule, dep.Version, targetSelected.DBVersion,
				))
			default:
				check.Status = CheckMismatch
				check.Compatible = releaseNames(c.compatibleReleases(targetModule, dep.Version))
				check.Detail = fmt.Sprintf("selected release provides DB %d, but minimum required DB is %d", targetSelected.DBVersion, dep.Version)
				if len(check.Compatible) == 0 {
					check.Detail += fmt.Sprintf("; no scanned %s release provides DB >= %d", targetModule, dep.Version)
				}
				report.Compatible = false
			}
			report.Checks = append(report.Checks, check)
		}
	}

	// EDS DB requirements are lower bounds. If two modules require GLO 4700
	// and GLO 4800, GLO 4800 satisfies both, so this is not a conflict.
	report.Conflicts = nil

	sort.Slice(report.Selected, func(i, j int) bool { return report.Selected[i].Module < report.Selected[j].Module })
	sort.Slice(report.Checks, func(i, j int) bool {
		if report.Checks[i].SourceModule == report.Checks[j].SourceModule {
			return report.Checks[i].TargetModule < report.Checks[j].TargetModule
		}
		return report.Checks[i].SourceModule < report.Checks[j].SourceModule
	})
	return report
}

func buildModuleAliases(c Catalog, artifactPrefix string) map[string]string {
	out := map[string]string{}
	for _, r := range c.Releases {
		module := normalizeModule(r.Module)
		out[module] = module
		out[strings.ToLower(artifactPrefix)+module] = module
		out[strings.ToLower(shortProject(r.Project))] = module
	}
	return out
}

func releaseNames(releases []Release) []string {
	out := make([]string, 0, len(releases))
	seen := map[string]bool{}
	for _, r := range releases {
		if !seen[r.Release] {
			out = append(out, r.Release)
			seen[r.Release] = true
		}
	}
	return out
}

func findRequirementConflicts(checks []DependencyCheck) []RequirementConflict {
	// Kept for compatibility with the report model. With minimum-version
	// semantics, multiple requirements for one module collapse to the highest
	// required DB version instead of conflicting.
	return nil
}
