package main

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
)

type PomInfo struct {
	ArtifactID   string
	Version      string
	Dependencies []PomDependency
}

type PomDependency struct {
	GroupID    string `json:"group_id"`
	ArtifactID string `json:"artifact_id"`
	Version    string `json:"version"`
	Scope      string `json:"scope,omitempty"`
}

type pomXML struct {
	ArtifactID   string        `xml:"artifactId"`
	Version      string        `xml:"version"`
	Properties   pomProperties `xml:"properties"`
	Dependencies []pomDepXML   `xml:"dependencies>dependency"`
}

type pomDepXML struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
	Scope      string `xml:"scope"`
}

type pomProperties map[string]string

func (p *pomProperties) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	*p = make(map[string]string)
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			var value string
			if err := d.DecodeElement(&value, &t); err != nil {
				return err
			}
			(*p)[t.Name.Local] = strings.TrimSpace(value)
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func ParsePOM(data []byte) (PomInfo, error) {
	var p pomXML
	if err := xml.Unmarshal(data, &p); err != nil {
		return PomInfo{}, fmt.Errorf("parse pom.xml: %w", err)
	}

	info := PomInfo{
		ArtifactID: strings.TrimSpace(p.ArtifactID),
		Version:    resolveProperties(strings.TrimSpace(p.Version), p.Properties),
	}
	for _, dep := range p.Dependencies {
		info.Dependencies = append(info.Dependencies, PomDependency{
			GroupID:    strings.TrimSpace(dep.GroupID),
			ArtifactID: strings.TrimSpace(dep.ArtifactID),
			Version:    resolveProperties(strings.TrimSpace(dep.Version), p.Properties),
			Scope:      strings.TrimSpace(dep.Scope),
		})
	}
	return info, nil
}

// ParseEngBEVersion reads only the platform dependency marker from a BM pom.xml.
// BM Maven dependencies are deliberately not used for BM-to-BM compatibility;
// only <engbe.version> is taken from the BM pom.
func ParseEngBEVersion(data []byte) (string, error) {
	var p pomXML
	if err := xml.Unmarshal(data, &p); err != nil {
		return "", fmt.Errorf("parse BM pom.xml: %w", err)
	}
	v := resolveProperties(strings.TrimSpace(p.Properties["engbe.version"]), p.Properties)
	if strings.Contains(v, "${") {
		return "", fmt.Errorf("engbe.version could not be resolved: %s", v)
	}
	return normalizeVersion(v), nil
}

var propertyRefRE = regexp.MustCompile(`\$\{([^}]+)\}`)

func resolveProperties(value string, props map[string]string) string {
	// A few passes also handle properties referencing other properties without
	// pulling Maven itself into this deliberately small CLI.
	for i := 0; i < 10; i++ {
		changed := false
		value = propertyRefRE.ReplaceAllStringFunc(value, func(ref string) string {
			key := strings.TrimSuffix(strings.TrimPrefix(ref, "${"), "}")
			v, ok := props[key]
			if !ok {
				return ref
			}
			changed = true
			return v
		})
		if !changed {
			break
		}
	}
	return value
}
