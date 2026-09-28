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

func TestCheckStandMissingModule(t *testing.T) {
	catalog := NewCatalog([]Release{
		{Project: "bm/cs-glo", Tag: "2.8.5", Release: "2.8.5", Module: "glo", DBVersion: 4800},
		{Project: "bm/cs-dpd", Tag: "2.3.0", Release: "2.3.0", Module: "dpd", DBVersion: 3710,
			Dependencies: []DBDependency{{Module: "glo", Version: 4712}}},
	}, nil)
	pom := PomInfo{Dependencies: []PomDependency{{ArtifactID: "cs-dpd", Version: "2.3.0"}}}

	report := CheckStand(catalog, pom, "cs-")
	if report.Compatible || len(report.Checks) != 1 || report.Checks[0].Status != CheckMissingModule {
		t.Fatalf("bad report: %+v", report)
	}
	if len(report.Checks[0].Compatible) != 1 || report.Checks[0].Compatible[0] != "2.8.5" {
		t.Fatalf("bad suggestions: %+v", report.Checks[0].Compatible)
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
