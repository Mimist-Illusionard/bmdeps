package main

import (
	"context"
	"fmt"
	"path"
	"sort"
	"sync"
)

type rawFileReader interface {
	RawFile(ctx context.Context, project, ref, filePath string) ([]byte, error)
}

type scanJob struct {
	Project string
	Tag     string
}

type scanResult struct {
	Release Release
	Err     error
	Job     scanJob
}

func ScanGitLab(ctx context.Context, cfg Config, limit int) (Catalog, error) {
	client := NewGitLabClient(cfg.GitLabURL, cfg.GitLabToken, cfg.HTTPTimeout)

	var platformReleases []PlatformRelease
	var listErrors []ScanError
	if cfg.EngBEProject != "" {
		tags, err := client.ListTags(ctx, cfg.EngBEProject)
		if err != nil {
			listErrors = append(listErrors, ScanError{Project: cfg.EngBEProject, Error: "scan platform tags: " + err.Error()})
		} else {
			for _, tag := range tags {
				if cfg.TagRegex.MatchString(tag.Name) {
					platformReleases = append(platformReleases, PlatformRelease{
						Project: cfg.EngBEProject,
						Tag:     tag.Name,
						Release: normalizeVersion(tag.Name),
					})
				}
			}
		}
	}

	var jobsList []scanJob
	for _, project := range cfg.Projects {
		tags, err := client.ListTags(ctx, project)
		if err != nil {
			listErrors = append(listErrors, ScanError{Project: project, Error: err.Error()})
			continue
		}

		var matching []string
		for _, tag := range tags {
			if cfg.TagRegex.MatchString(tag.Name) {
				matching = append(matching, tag.Name)
			}
		}
		// GitLab does not have to return semantic versions in semantic order.
		// Sort locally so --limit means newest BM release versions.
		sort.Slice(matching, func(i, j int) bool {
			return versionLess(matching[j], matching[i])
		})
		if limit > 0 && len(matching) > limit {
			matching = matching[:limit]
		}
		for _, tag := range matching {
			jobsList = append(jobsList, scanJob{Project: project, Tag: tag})
		}
	}

	if len(jobsList) == 0 {
		c := NewCatalog(nil, listErrors)
		c.PlatformReleases = platformReleases
		c.sort()
		return c, fmt.Errorf("no matching BM release tags found")
	}

	jobs := make(chan scanJob)
	results := make(chan scanResult)
	var wg sync.WaitGroup

	workers := cfg.Concurrency
	if workers > len(jobsList) {
		workers = len(jobsList)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				r, err := analyzeRelease(ctx, client, cfg.DataModelPath, job.Project, job.Tag)
				results <- scanResult{Release: r, Err: err, Job: job}
			}
		}()
	}

	go func() {
		for _, job := range jobsList {
			jobs <- job
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	var releases []Release
	errors := append([]ScanError(nil), listErrors...)
	for result := range results {
		if result.Err != nil {
			errors = append(errors, ScanError{Project: result.Job.Project, Tag: result.Job.Tag, Error: result.Err.Error()})
			continue
		}
		releases = append(releases, result.Release)
	}

	c := NewCatalog(releases, errors)
	c.PlatformReleases = platformReleases
	c.sort()
	return c, nil
}

func analyzeRelease(ctx context.Context, reader rawFileReader, dataModelPath, project, tag string) (Release, error) {
	rootInstallPath := path.Join(dataModelPath, "install.eds")
	rootInstall, err := reader.RawFile(ctx, project, tag, rootInstallPath)
	if err != nil {
		return Release{}, fmt.Errorf("%s: %w", rootInstallPath, err)
	}

	groupInstallPath, err := LatestVersionInstaller(rootInstall, dataModelPath)
	if err != nil {
		return Release{}, err
	}
	groupInstall, err := reader.RawFile(ctx, project, tag, groupInstallPath)
	if err != nil {
		return Release{}, fmt.Errorf("%s: %w", groupInstallPath, err)
	}

	info := ParseInstaller(groupInstall)
	selectedInstaller := groupInstallPath
	if info.ModuleVersion == 0 {
		nestedPath, ok := LatestNestedInstaller(groupInstall, groupInstallPath)
		if !ok {
			return Release{}, fmt.Errorf("%s has no assert-module-version and no active numeric nested installer", groupInstallPath)
		}
		nestedInstall, fetchErr := reader.RawFile(ctx, project, tag, nestedPath)
		if fetchErr != nil {
			return Release{}, fmt.Errorf("%s: %w", nestedPath, fetchErr)
		}
		info = ParseInstaller(nestedInstall)
		selectedInstaller = nestedPath
	}

	if info.ModuleVersion == 0 || info.Module == "" {
		return Release{}, fmt.Errorf("%s: assert-module-version not found", selectedInstaller)
	}

	// BM-to-BM compatibility still comes only from EDS. The one Maven value we
	// intentionally read from a BM tag is <engbe.version>, because it describes
	// the platform backend release this exact BM release was built against.
	var engBEVersion string
	if pomData, pomErr := reader.RawFile(ctx, project, tag, "pom.xml"); pomErr == nil {
		if parsed, parseErr := ParseEngBEVersion(pomData); parseErr == nil {
			engBEVersion = parsed
		}
	}

	return Release{
		Project:       project,
		Tag:           tag,
		Release:       normalizeVersion(tag),
		Module:        normalizeModule(info.Module),
		DBVersion:     info.ModuleVersion,
		EngBEVersion:  engBEVersion,
		InstallerPath: selectedInstaller,
		Dependencies:  info.Dependencies,
	}, nil
}
