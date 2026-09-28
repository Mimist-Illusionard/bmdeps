package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

var errIncompatible = errors.New("stand is incompatible")

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "scan":
		err = runScan(os.Args[2:])
	case "list":
		err = runList(os.Args[2:])
	case "check":
		err = runCheck(os.Args[2:])
	case "help", "-h", "--help":
		usage()
		return
	default:
		usage()
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}

	if err != nil {
		if errors.Is(err, errIncompatible) {
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runScan(args []string) error {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	envPath := fs.String("env", "./bmdeps.env", "path to env file")
	outDir := fs.String("out", "./out", "output directory")
	limit := fs.Int("limit", 0, "max newest matching tags per project; 0 = all")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := LoadConfig(*envPath)
	if err != nil {
		return err
	}

	catalog, scanErr := ScanGitLab(context.Background(), cfg, *limit)
	if err := WriteCatalogFiles(catalog, *outDir); err != nil {
		return err
	}
	PrintScanSummary(catalog)
	fmt.Printf("Wrote %s and %s\n", filepath.Join(*outDir, "catalog.json"), filepath.Join(*outDir, "graph.dot"))
	return scanErr
}

func runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	catalogPath := fs.String("catalog", "./out/catalog.json", "path to catalog.json")
	module := fs.String("module", "", "optional EDS module filter, for example glo")
	if err := fs.Parse(args); err != nil {
		return err
	}
	catalog, err := LoadCatalog(*catalogPath)
	if err != nil {
		return err
	}
	PrintCatalog(catalog, *module)
	return nil
}

func runCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	catalogPath := fs.String("catalog", "./out/catalog.json", "path to catalog.json")
	pomPath := fs.String("pom", "", "path to stand/builder pom.xml")
	artifactPrefix := fs.String("prefix", "cs-", "BM Maven artifact prefix")
	reportPath := fs.String("report", "", "optional path to write JSON report")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *pomPath == "" {
		return fmt.Errorf("check requires --pom /path/to/stand/pom.xml")
	}

	catalog, err := LoadCatalog(*catalogPath)
	if err != nil {
		return err
	}
	pomData, err := os.ReadFile(*pomPath)
	if err != nil {
		return fmt.Errorf("read stand pom: %w", err)
	}
	pom, err := ParsePOM(pomData)
	if err != nil {
		return err
	}

	report := CheckStand(catalog, pom, *artifactPrefix)
	PrintStandReport(report)
	if *reportPath != "" {
		if err := WriteStandReport(report, *reportPath); err != nil {
			return err
		}
		fmt.Printf("JSON report: %s\n", *reportPath)
	}
	if !report.Compatible {
		return errIncompatible
	}
	return nil
}

func usage() {
	fmt.Print(`bmdeps - correlate BM Git tags with EDS DB versions and validate a stand POM

Usage:
  bmdeps scan  --env ./bmdeps.env --out ./out [--limit 20]
  bmdeps list  --catalog ./out/catalog.json [--module glo]
  bmdeps check --catalog ./out/catalog.json --pom ./stand_pom.xml [--report ./report.json]

Commands:
  scan   For every configured BM project/tag, read dataModel install.eds files.
         The Git tag is the BM release. EDS gives the DB version and DB dependencies.
         BM pom.xml files are intentionally NOT used for compatibility.

  list   Show release -> DB mappings discovered from Git tags + EDS.

  check  Read the builder/stand pom.xml, resolve selected cs-* release versions,
         translate them through catalog.json to DB versions, then compare them with
         assert-dependency requirements from EDS. Exit code 2 means incompatible.
`)
}
