package main

import (
	"fmt"
	"regexp"
)

// Accepting slugs on --project / --app / --env flags is a better UX than
// forcing callers to curl + jq their way to UUIDs. These helpers resolve a
// slug-or-UUID string to the canonical UUID the server wants.
//
// All three accept the empty string and return empty (optional flag shape).

var uuidRe = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// looksLikeUUID reports whether s matches the canonical UUID format.
func looksLikeUUID(s string) bool {
	return uuidRe.MatchString(s)
}

// resolveProjectInput returns the UUID and slug for a --project flag value.
// Accepts either a UUID or a slug. Requires an org to be set in the
// session config so slug lookups are unambiguous.
func resolveProjectInput(client *apiClient, input string) (id, slug string, err error) {
	if input == "" {
		return "", "", nil
	}
	org, orgErr := requireOrg()
	if orgErr != nil {
		return "", "", fmt.Errorf("cannot resolve project %q: %w", input, orgErr)
	}
	if looksLikeUUID(input) {
		// Look up the slug so subsequent --app resolution can use it.
		resp, gerr := client.get(fmt.Sprintf("/api/v1/orgs/%s/projects", org))
		if gerr != nil {
			return input, "", nil // fall back: UUID works even if slug lookup fails
		}
		projects, _ := resp["projects"].([]interface{})
		for _, p := range projects {
			m, ok := p.(map[string]interface{})
			if !ok {
				continue
			}
			if m["id"] == input {
				if s, ok := m["slug"].(string); ok {
					return input, s, nil
				}
			}
		}
		return input, "", nil
	}
	// Input is a slug — resolve to UUID.
	resp, gerr := client.get(fmt.Sprintf("/api/v1/orgs/%s/projects/%s", org, input))
	if gerr != nil {
		return "", "", fmt.Errorf("project %q not found in org %q: %w", input, org, gerr)
	}
	pid, _ := resp["id"].(string)
	if pid == "" {
		return "", "", fmt.Errorf("project %q not found in org %q", input, org)
	}
	return pid, input, nil
}

// resolveAppInput returns the UUID for a --app flag value. Accepts either
// a UUID or a slug; requires the project slug to disambiguate.
func resolveAppInput(client *apiClient, projectSlug, input string) (string, error) {
	if input == "" {
		return "", nil
	}
	if looksLikeUUID(input) {
		return input, nil
	}
	org, orgErr := requireOrg()
	if orgErr != nil {
		return "", fmt.Errorf("cannot resolve app %q: %w", input, orgErr)
	}
	if projectSlug == "" {
		return "", fmt.Errorf("resolving app slug %q requires --project as a slug (pass the project slug, not a UUID, or look up the project first)", input)
	}
	resp, gerr := client.get(fmt.Sprintf("/api/v1/orgs/%s/projects/%s/apps/%s", org, projectSlug, input))
	if gerr != nil {
		return "", fmt.Errorf("app %q not found in project %q: %w", input, projectSlug, gerr)
	}
	aid, _ := resp["id"].(string)
	if aid == "" {
		return "", fmt.Errorf("app %q not found in project %q", input, projectSlug)
	}
	return aid, nil
}

// resolveEnvInputs returns UUIDs for a repeated --env flag. Each value may
// be a UUID or a slug.
func resolveEnvInputs(client *apiClient, inputs []string) ([]string, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	// Fast path: everything is already a UUID.
	allUUID := true
	for _, v := range inputs {
		if !looksLikeUUID(v) {
			allUUID = false
			break
		}
	}
	if allUUID {
		return inputs, nil
	}

	org, orgErr := requireOrg()
	if orgErr != nil {
		return nil, fmt.Errorf("cannot resolve env slugs: %w", orgErr)
	}
	resp, gerr := client.get(fmt.Sprintf("/api/v1/orgs/%s/environments", org))
	if gerr != nil {
		return nil, fmt.Errorf("list environments: %w", gerr)
	}
	envs, _ := resp["environments"].([]interface{})
	slugToID := map[string]string{}
	for _, e := range envs {
		m, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		if slug, ok := m["slug"].(string); ok {
			if id, ok := m["id"].(string); ok {
				slugToID[slug] = id
			}
		}
	}

	out := make([]string, 0, len(inputs))
	for _, v := range inputs {
		if looksLikeUUID(v) {
			out = append(out, v)
			continue
		}
		id, ok := slugToID[v]
		if !ok {
			return nil, fmt.Errorf("environment %q not found in org %q", v, org)
		}
		out = append(out, id)
	}
	return out, nil
}
