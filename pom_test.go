package main

import "testing"

func TestParseStandPOMResolvesModuleProperties(t *testing.T) {
	src := []byte(`<?xml version="1.0"?>
<project>
  <artifactId>staxel_start</artifactId>
  <version>0.1.16</version>
  <properties>
    <engdb.glo.version>2.6.3</engdb.glo.version>
    <engdb.dpd.version>2.3.21</engdb.dpd.version>
  </properties>
  <dependencies>
    <dependency><groupId>ru.cs</groupId><artifactId>cs-glo</artifactId><version>${engdb.glo.version}</version><scope>provided</scope></dependency>
    <dependency><groupId>ru.cs</groupId><artifactId>cs-dpd</artifactId><version>${engdb.dpd.version}</version><scope>provided</scope></dependency>
  </dependencies>
</project>`)
	got, err := ParsePOM(src)
	if err != nil {
		t.Fatal(err)
	}
	if got.ArtifactID != "staxel_start" || got.Version != "0.1.16" {
		t.Fatalf("bad pom: %+v", got)
	}
	if len(got.Dependencies) != 2 || got.Dependencies[0].Version != "2.6.3" || got.Dependencies[1].Version != "2.3.21" {
		t.Fatalf("bad deps: %+v", got.Dependencies)
	}
}

func TestParseEngBEVersion(t *testing.T) {
	src := []byte(`<?xml version="1.0"?>
<project>
  <properties>
    <engbe.version>2.38.1</engbe.version>
  </properties>
</project>`)
	got, err := ParseEngBEVersion(src)
	if err != nil {
		t.Fatal(err)
	}
	if got != "2.38.1" {
		t.Fatalf("got %q", got)
	}
}
