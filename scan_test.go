package main

import (
	"context"
	"fmt"
	"testing"
)

type fakeFiles map[string]string

func (f fakeFiles) RawFile(_ context.Context, project, ref, filePath string) ([]byte, error) {
	key := project + "@" + ref + ":" + filePath
	v, ok := f[key]
	if !ok {
		return nil, fmt.Errorf("missing %s", key)
	}
	return []byte(v), nil
}

func TestAnalyzeReleaseUsesTagAndEDSNotPOM(t *testing.T) {
	files := fakeFiles{
		"bm/cs-dpd@2.3.21:dataModel/install.eds": `
run /version.3600/install.eds
run /version.3700/install.eds
`,
		"bm/cs-dpd@2.3.21:dataModel/version.3700/install.eds": `
run '3709/install.eds'
run '3710/install.eds'
`,
		"bm/cs-dpd@2.3.21:dataModel/version.3700/3710/install.eds": `
assert-dependency should-be 4800 name "glo"
assert-dependency should-be 3709 name "dpd"
assert-module-version should-be 3710 module-name "dpd"
`,
	}

	r, err := analyzeRelease(context.Background(), files, "dataModel", "bm/cs-dpd", "2.3.21")
	if err != nil {
		t.Fatal(err)
	}
	if r.Release != "2.3.21" || r.Module != "dpd" || r.DBVersion != 3710 {
		t.Fatalf("bad release: %+v", r)
	}
	if len(r.Dependencies) != 2 || r.Dependencies[0].Module != "glo" || r.Dependencies[0].Version != 4800 {
		t.Fatalf("bad dependencies: %+v", r.Dependencies)
	}
}

func TestAnalyzeReleaseAddsEngBEFromBMPOM(t *testing.T) {
	files := fakeFiles{
		"bm/cs-dpd@2.3.21:dataModel/install.eds":              `run /version.3700/install.eds`,
		"bm/cs-dpd@2.3.21:dataModel/version.3700/install.eds": `assert-module-version should-be 3710 module-name "dpd"`,
		"bm/cs-dpd@2.3.21:pom.xml":                            `<project><properties><engbe.version>2.38.1</engbe.version></properties></project>`,
	}
	r, err := analyzeRelease(context.Background(), files, "dataModel", "bm/cs-dpd", "2.3.21")
	if err != nil {
		t.Fatal(err)
	}
	if r.EngBEVersion != "2.38.1" {
		t.Fatalf("engbe=%q release=%+v", r.EngBEVersion, r)
	}
}

func TestAnalyzeReleaseFollowsVersionedNestedInstaller(t *testing.T) {
	files := fakeFiles{
		"cs-bm/cs-ultimatek@v0.1.0:dataModel/install.eds": `
run /version.100/install.eds
run 'GLOBAL_SCRIPTS/install.eds'
`,
		"cs-bm/cs-ultimatek@v0.1.0:dataModel/version.100/install.eds": `
warn 'Установка ultimatek 100'
run '/version.100/install.eds'
`,
		"cs-bm/cs-ultimatek@v0.1.0:dataModel/version.100/version.100/install.eds": `
assert-dependency should-be 4712 name "glo"
assert-dependency should-be 2107 name "dms"
assert-module-version should-be 100 module-name "ultc"
`,
	}

	r, err := analyzeRelease(context.Background(), files, "dataModel", "cs-bm/cs-ultimatek", "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if r.Module != "ultc" || r.DBVersion != 100 {
		t.Fatalf("bad release: %+v", r)
	}
	if r.InstallerPath != "dataModel/version.100/version.100/install.eds" {
		t.Fatalf("installer=%q", r.InstallerPath)
	}
}
