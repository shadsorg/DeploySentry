package main

import (
	"errors"
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

// resolveProjectID returns the project's UUID by GETting
// /api/v1/orgs/:org/projects/:project. Errors out with a useful
// message if the project doesn't exist or the API is unreachable.
func resolveProjectID(client *apiClient, org, projectSlug string) (string, error) {
	path := fmt.Sprintf("/api/v1/orgs/%s/projects/%s", org, projectSlug)
	resp, err := client.get(path)
	if err != nil {
		return "", fmt.Errorf("resolve project %q: %w", projectSlug, err)
	}
	id, ok := resp["id"].(string)
	if !ok || id == "" {
		return "", fmt.Errorf("resolve project %q: response missing id", projectSlug)
	}
	return id, nil
}

// resolveEnvID returns the environment's UUID for the given org by listing
// org-level environments and matching on slug.
func resolveEnvID(client *apiClient, org, envSlug string) (string, error) {
	path := fmt.Sprintf("/api/v1/orgs/%s/environments", org)
	resp, err := client.get(path)
	if err != nil {
		return "", fmt.Errorf("resolve environment %q: %w", envSlug, err)
	}
	envs, _ := resp["environments"].([]any)
	for _, e := range envs {
		obj, ok := e.(map[string]any)
		if !ok {
			continue
		}
		slug, _ := obj["slug"].(string)
		if slug == envSlug {
			id, _ := obj["id"].(string)
			if id != "" {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("environment %q not found in org %q", envSlug, org)
}

// ErrFlagNotFound is returned when resolveFlagID can't find a flag with the
// given key in the given project.
var ErrFlagNotFound = errors.New("flag not found")

// resolveFlagID returns a flag's UUID by listing flags in the given project
// and matching on key.
func resolveFlagID(client *apiClient, projectID, flagKey string) (string, error) {
	path := fmt.Sprintf("/api/v1/flags?project_id=%s", projectID)
	resp, err := client.get(path)
	if err != nil {
		return "", fmt.Errorf("resolve flag %q: %w", flagKey, err)
	}
	flags, _ := resp["flags"].([]any)
	for _, f := range flags {
		obj, ok := f.(map[string]any)
		if !ok {
			continue
		}
		key, _ := obj["key"].(string)
		if key == flagKey {
			id, _ := obj["id"].(string)
			if id != "" {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("%w: %q in project %s", ErrFlagNotFound, flagKey, projectID)
}
