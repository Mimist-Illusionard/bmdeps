package main

import "testing"

func TestLatestVersionInstallerIgnoresCommentedRuns(t *testing.T) {
	root := []byte(`
run /version.400/install.eds
run /version.500/install.eds
//run /version.501/install.eds
run /version.20000/install.eds
`)
	got, err := LatestVersionInstaller(root, "dataModel")
	if err != nil {
		t.Fatal(err)
	}
	if got != "dataModel/version.20000/install.eds" {
		t.Fatalf("got %q", got)
	}
}

func TestDPDStyleNestedInstaller(t *testing.T) {
	group := []byte(`
warn 'Установка dpd 3700'
run '3708/install.eds'
run '3709/install.eds'
run '3710/install.eds'
`)
	got, ok := LatestNestedInstaller(group, "dataModel/version.3700/install.eds")
	if !ok {
		t.Fatal("not found")
	}
	if got != "dataModel/version.3700/3710/install.eds" {
		t.Fatalf("got %q", got)
	}
}

func TestParseInstaller(t *testing.T) {
	src := []byte(`
start-trans
assert-dependency should-be 4712 name "glo"
assert-dependency should-be 2107 name "dms"
assert-dependency should-be 3709 name "dpd"
assert-module-version should-be 3710 module-name "dpd"
commit-trans
`)
	got := ParseInstaller(src)
	if got.Module != "dpd" || got.ModuleVersion != 3710 {
		t.Fatalf("bad module: %+v", got)
	}
	if len(got.Dependencies) != 3 {
		t.Fatalf("bad deps: %+v", got.Dependencies)
	}
	if got.Dependencies[0].Module != "glo" || got.Dependencies[0].Version != 4712 {
		t.Fatalf("bad first dep: %+v", got.Dependencies[0])
	}
}
