package main

import (
	"fmt"
	"sort"
)

type discoveredSkill struct {
	Name   string
	Path   string
	Origin string
	Root   string
}

func mergeDiscoveredSkills(dst map[string]discoveredSkill, incoming []discoveredSkill) error {
	sort.Slice(incoming, func(i, j int) bool {
		if incoming[i].Name != incoming[j].Name {
			return incoming[i].Name < incoming[j].Name
		}
		if incoming[i].Origin != incoming[j].Origin {
			return incoming[i].Origin < incoming[j].Origin
		}
		if incoming[i].Root != incoming[j].Root {
			return incoming[i].Root < incoming[j].Root
		}
		return incoming[i].Path < incoming[j].Path
	})
	for _, skill := range incoming {
		if existing, ok := dst[skill.Name]; ok {
			return fmt.Errorf("skill %q from %s (root %s) collides with %s (root %s)", skill.Name, skill.Origin, skill.Root, existing.Origin, existing.Root)
		}
		dst[skill.Name] = skill
	}
	return nil
}

func discoveredSkillValues(skills map[string]discoveredSkill) []discoveredSkill {
	values := make([]discoveredSkill, 0, len(skills))
	for _, skill := range skills {
		values = append(values, skill)
	}
	return values
}

func discoveredSkillPaths(skills map[string]discoveredSkill) map[string]string {
	paths := make(map[string]string, len(skills))
	for name, skill := range skills {
		paths[name] = skill.Path
	}
	return paths
}
