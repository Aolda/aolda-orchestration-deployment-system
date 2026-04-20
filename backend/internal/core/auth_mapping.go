package core

import (
	"fmt"
	"net/http"
	"strings"
)

type AuthorityMappingUserProvider struct {
	Base     UserProvider
	Mappings map[string][]string
}

func (p AuthorityMappingUserProvider) CurrentUser(r *http.Request) (User, error) {
	user, err := p.Base.CurrentUser(r)
	if err != nil {
		return User{}, err
	}
	user.Groups = expandAuthorities(user.Groups, p.Mappings)
	return user, nil
}

func expandAuthorities(groups []string, mappings map[string][]string) []string {
	if len(groups) == 0 || len(mappings) == 0 {
		return groups
	}

	seen := make(map[string]struct{}, len(groups))
	items := make([]string, 0, len(groups))
	for _, group := range groups {
		trimmed := strings.TrimSpace(group)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; !ok {
			seen[trimmed] = struct{}{}
			items = append(items, trimmed)
		}
		for _, alias := range mappings[trimmed] {
			normalizedAlias := strings.TrimSpace(alias)
			if normalizedAlias == "" {
				continue
			}
			if _, ok := seen[normalizedAlias]; ok {
				continue
			}
			seen[normalizedAlias] = struct{}{}
			items = append(items, normalizedAlias)
		}
	}

	return items
}

func parseAuthorityMappings(raw string) (map[string][]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	pairs := strings.Split(raw, ",")
	mappings := make(map[string][]string)
	for _, pair := range pairs {
		trimmedPair := strings.TrimSpace(pair)
		if trimmedPair == "" {
			continue
		}

		source, targetsRaw, ok := strings.Cut(trimmedPair, "=")
		if !ok {
			return nil, fmt.Errorf("invalid authority mapping %q: expected source=target", trimmedPair)
		}

		normalizedSource := strings.TrimSpace(source)
		if normalizedSource == "" {
			return nil, fmt.Errorf("invalid authority mapping %q: source is empty", trimmedPair)
		}

		targets := dedupeAuthorityStrings(strings.FieldsFunc(targetsRaw, func(r rune) bool {
			return r == '|' || r == ';'
		}))
		if len(targets) == 0 {
			return nil, fmt.Errorf("invalid authority mapping %q: target is empty", trimmedPair)
		}

		mappings[normalizedSource] = dedupeAuthorityStrings(append(mappings[normalizedSource], targets...))
	}

	return mappings, nil
}

func dedupeAuthorityStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	items := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}
	return items
}
