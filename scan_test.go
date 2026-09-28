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
