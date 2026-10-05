package main

import "testing"

func TestCheckStandRejectsLowerDBAndSuggestsCompatibleRelease(t *testing.T) {
	catalog := NewCatalog([]Release{
		{Project: "bm/cs-glo", Tag: "2.7.9", Release: "2.7.9", Module: "glo", DBVersion: 4700},
		{Project: "bm/cs-glo", Tag: "2.8.5", Release: "2.8.5", Module: "glo", DBVersion: 4800},
		{Project: "bm/cs-glo", Tag: "2.9.0", Release: "2.9.0", Module: "glo", DBVersion: 4900},
		{Project: "bm/cs-dpd", Tag: "2.3.0", Release: "2.3.0", Module: "dpd", DBVersion: 3710,
			Dependencies: []DBDependency{{Module: "glo", Version: 4800}, {Module: "dpd", Version: 3709}}},
	}, nil)

	pom := PomInfo{ArtifactID: "stand", Version: "1.0.0", Dependencies: []PomDependency{
		{GroupID: "ru.cs", ArtifactID: "cs-dpd", Version: "2.3.0"},
		{GroupID: "ru.cs", ArtifactID: "cs-glo", Version: "2.7.9"},
	}}

	report := CheckStand(catalog, pom, "cs-")
	if report.Compatible {
		t.Fatal("expected incompatible stand")
	}
	if len(report.Checks) != 1 {
		t.Fatalf("checks=%+v", report.Checks)
	}
	check := report.Checks[0]
	if check.Status != CheckMismatch || check.RequiredDBVersion != 4800 || check.ActualDBVersion != 4700 {
		t.Fatalf("bad check: %+v", check)
	}
	if len(check.Compatible) != 2 || check.Compatible[0] != "2.9.0" || check.Compatible[1] != "2.8.5" {
		t.Fatalf("bad suggestions: %+v", check.Compatible)
	}
}

func TestCheckStandAllowsHigherDBWithWarning(t *testing.T) {
	catalog := NewCatalog([]Release{
		{Project: "bm/cs-glo", Tag: "2.6.3", Release: "2.6.3", Module: "glo", DBVersion: 4800},
		{Project: "bm/cs-dpd", Tag: "2.3.21", Release: "2.3.21", Module: "dpd", DBVersion: 3710,
			Dependencies: []DBDependency{{Module: "glo", Version: 4712}}},
	}, nil)
	pom := PomInfo{Dependencies: []PomDependency{
		{ArtifactID: "cs-dpd", Version: "2.3.21"},
		{ArtifactID: "cs-glo", Version: "2.6.3"},
	}}

	report := CheckStand(catalog, pom, "cs-")
	if !report.Compatible {
		t.Fatalf("higher DB must be compatible: %+v", report)
	}
	if len(report.Checks) != 1 || report.Checks[0].Status != CheckWarning {
		t.Fatalf("expected warning: %+v", report.Checks)
	}
	if report.Checks[0].RequiredDBVersion != 4712 || report.Checks[0].ActualDBVersion != 4800 {
		t.Fatalf("bad versions: %+v", report.Checks[0])
	}
	if len(report.Warnings) != 1 {
		t.Fatalf("expected one warning: %+v", report.Warnings)
	}
}

func TestCheckStandExactDBIsOK(t *testing.T) {
	catalog := NewCatalog([]Release{
		{Project: "bm/cs-glo", Tag: "2.8.5", Release: "2.8.5", Module: "glo", DBVersion: 4800},
		{Project: "bm/cs-dpd", Tag: "2.3.0", Release: "2.3.0", Module: "dpd", DBVersion: 3710,
			Dependencies: []DBDependency{{Module: "glo", Version: 4800}}},
	}, nil)
	pom := PomInfo{Dependencies: []PomDependency{
		{ArtifactID: "cs-dpd", Version: "2.3.0"},
		{ArtifactID: "cs-glo", Version: "2.8.5"},
	}}

	report := CheckStand(catalog, pom, "cs-")
	if !report.Compatible || len(report.Checks) != 1 || report.Checks[0].Status != CheckOK {
		t.Fatalf("expected exact match: %+v", report)
	}
}

func TestCheckStandIgnoresEDSDependencyAbsentFromStandPOM(t *testing.T) {
	catalog := NewCatalog([]Release{
		{Project: "bm/cs-glo", Tag: "2.8.5", Release: "2.8.5", Module: "glo", DBVersion: 4800},
		{Project: "bm/cs-dpd", Tag: "2.3.0", Release: "2.3.0", Module: "dpd", DBVersion: 3710,
			Dependencies: []DBDependency{{Module: "glo", Version: 4712}}},
	}, nil)
	pom := PomInfo{Dependencies: []PomDependency{{ArtifactID: "cs-dpd", Version: "2.3.0"}}}

	report := CheckStand(catalog, pom, "cs-")
	if !report.Compatible {
		t.Fatalf("dependency on module absent from stand pom must be ignored: %+v", report)
	}
	if len(report.Checks) != 0 {
		t.Fatalf("expected no dependency checks for modules absent from stand pom: %+v", report.Checks)
	}
}

func TestDifferentMinimumRequirementsDoNotConflict(t *testing.T) {
	catalog := NewCatalog([]Release{
		{Project: "bm/cs-glo", Tag: "2.8.5", Release: "2.8.5", Module: "glo", DBVersion: 4800},
		{Project: "bm/cs-dpd", Tag: "2.3.0", Release: "2.3.0", Module: "dpd", DBVersion: 3710,
			Dependencies: []DBDependency{{Module: "glo", Version: 4800}}},
		{Project: "bm/cs-ped", Tag: "2.1.0", Release: "2.1.0", Module: "ped", DBVersion: 2100,
			Dependencies: []DBDependency{{Module: "glo", Version: 4700}}},
	}, nil)
	pom := PomInfo{Dependencies: []PomDependency{
		{ArtifactID: "cs-dpd", Version: "2.3.0"},
		{ArtifactID: "cs-ped", Version: "2.1.0"},
		{ArtifactID: "cs-glo", Version: "2.8.5"},
	}}

	report := CheckStand(catalog, pom, "cs-")
	if !report.Compatible {
		t.Fatalf("4800 must satisfy both 4800 and 4700 minimums: %+v", report)
	}
	if len(report.Conflicts) != 0 {
		t.Fatalf("minimum requirements must not conflict: %+v", report.Conflicts)
	}
	var warningFound bool
	for _, c := range report.Checks {
		if c.SourceModule == "ped" && c.TargetModule == "glo" && c.Status == CheckWarning {
			warningFound = true
		}
	}
	if !warningFound {
		t.Fatalf("expected warning for PED requiring 4700 while stand has 4800: %+v", report.Checks)
	}
}

func TestCheckStandValidatesEngBERelease(t *testing.T) {
	catalog := NewCatalog([]Release{
		{Project: "bm/cs-dpd", Tag: "2.3.21", Release: "2.3.21", Module: "dpd", DBVersion: 3710, EngBEVersion: "2.38.1"},
	}, nil)
	catalog.PlatformReleases = []PlatformRelease{{Project: "lithtechteam/cs-eng-be", Tag: "2.38.1", Release: "2.38.1"}}
	pom := PomInfo{Dependencies: []PomDependency{{ArtifactID: "cs-dpd", Version: "2.3.21"}}}

	report := CheckStand(catalog, pom, "cs-")
	if !report.Compatible {
		t.Fatalf("expected compatible: %+v", report)
	}
	if len(report.PlatformChecks) != 1 || report.PlatformChecks[0].Status != CheckOK || report.PlatformChecks[0].RequiredEngBE != "2.38.1" {
		t.Fatalf("bad platform check: %+v", report.PlatformChecks)
	}
	if len(report.Selected) != 1 || report.Selected[0].EngBEVersion != "2.38.1" {
		t.Fatalf("bad selected module: %+v", report.Selected)
	}
}

func TestCheckStandRejectsMissingEngBETag(t *testing.T) {
	catalog := NewCatalog([]Release{
		{Project: "bm/cs-dpd", Tag: "2.3.21", Release: "2.3.21", Module: "dpd", DBVersion: 3710, EngBEVersion: "2.38.1"},
	}, nil)
	catalog.PlatformReleases = []PlatformRelease{{Project: "lithtechteam/cs-eng-be", Tag: "2.38.0", Release: "2.38.0"}}
	pom := PomInfo{Dependencies: []PomDependency{{ArtifactID: "cs-dpd", Version: "2.3.21"}}}

	report := CheckStand(catalog, pom, "cs-")
	if report.Compatible {
		t.Fatalf("missing required engbe tag must be incompatible: %+v", report)
	}
	if len(report.PlatformChecks) != 1 || report.PlatformChecks[0].Status != CheckMissingRelease {
		t.Fatalf("bad platform check: %+v", report.PlatformChecks)
	}
}

func TestCheckStandChecksOnlyTargetsPresentInPOM(t *testing.T) {
	catalog := NewCatalog([]Release{
		{Project: "bm/cs-glo", Tag: "2.6.3", Release: "2.6.3", Module: "glo", DBVersion: 4800},
		{Project: "bm/cs-dms", Tag: "2.0.14", Release: "2.0.14", Module: "dms", DBVersion: 2000},
		{Project: "bm/cs-dpd", Tag: "2.3.21", Release: "2.3.21", Module: "dpd", DBVersion: 3710,
			Dependencies: []DBDependency{
				{Module: "glo", Version: 4712},
				{Module: "dms", Version: 2107},
			}},
	}, nil)
	pom := PomInfo{Dependencies: []PomDependency{
		{ArtifactID: "cs-dpd", Version: "2.3.21"},
		{ArtifactID: "cs-glo", Version: "2.6.3"},
	}}

	report := CheckStand(catalog, pom, "cs-")
	if !report.Compatible {
		t.Fatalf("DMS is absent from stand POM and must not make the stand incompatible: %+v", report)
	}
	if len(report.Checks) != 1 {
		t.Fatalf("expected exactly one in-POM dependency check, got %+v", report.Checks)
	}
	check := report.Checks[0]
	if check.SourceModule != "dpd" || check.TargetModule != "glo" || check.Status != CheckWarning {
		t.Fatalf("unexpected check: %+v", check)
	}
}
