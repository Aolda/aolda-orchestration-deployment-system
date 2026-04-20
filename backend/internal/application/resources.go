package application

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	defaultResourceRequestCPU    = "250m"
	defaultResourceRequestMemory = "256Mi"
	defaultResourceLimitCPU      = "500m"
	defaultResourceLimitMemory   = "512Mi"
)

func DefaultResourceRequirements() ResourceRequirements {
	return ResourceRequirements{
		Requests: ResourceQuantity{
			CPU:    defaultResourceRequestCPU,
			Memory: defaultResourceRequestMemory,
		},
		Limits: ResourceQuantity{
			CPU:    defaultResourceLimitCPU,
			Memory: defaultResourceLimitMemory,
		},
	}
}

func (r ResourceRequirements) IsZero() bool {
	return strings.TrimSpace(r.Requests.CPU) == "" &&
		strings.TrimSpace(r.Requests.Memory) == "" &&
		strings.TrimSpace(r.Limits.CPU) == "" &&
		strings.TrimSpace(r.Limits.Memory) == ""
}

func normalizeResourceRequirements(input ResourceRequirements, applyDefaults bool) (ResourceRequirements, error) {
	normalized := ResourceRequirements{
		Requests: ResourceQuantity{
			CPU:    strings.TrimSpace(input.Requests.CPU),
			Memory: strings.TrimSpace(input.Requests.Memory),
		},
		Limits: ResourceQuantity{
			CPU:    strings.TrimSpace(input.Limits.CPU),
			Memory: strings.TrimSpace(input.Limits.Memory),
		},
	}

	if applyDefaults {
		defaults := DefaultResourceRequirements()
		if normalized.Requests.CPU == "" {
			normalized.Requests.CPU = defaults.Requests.CPU
		}
		if normalized.Requests.Memory == "" {
			normalized.Requests.Memory = defaults.Requests.Memory
		}
		if normalized.Limits.CPU == "" {
			normalized.Limits.CPU = defaults.Limits.CPU
		}
		if normalized.Limits.Memory == "" {
			normalized.Limits.Memory = defaults.Limits.Memory
		}
	}

	if err := validateResourceRequirements(normalized); err != nil {
		return ResourceRequirements{}, err
	}
	return normalized, nil
}

func validateResourceRequirements(input ResourceRequirements) error {
	checks := []struct {
		label string
		value string
		parse func(string) (float64, error)
	}{
		{label: "resources.requests.cpu", value: input.Requests.CPU, parse: parseCPUQuantityToCoresForValidation},
		{label: "resources.requests.memory", value: input.Requests.Memory, parse: parseMemoryQuantityToMiBForValidation},
		{label: "resources.limits.cpu", value: input.Limits.CPU, parse: parseCPUQuantityToCoresForValidation},
		{label: "resources.limits.memory", value: input.Limits.Memory, parse: parseMemoryQuantityToMiBForValidation},
	}
	for _, check := range checks {
		if strings.TrimSpace(check.value) == "" {
			return ValidationError{
				Message: fmt.Sprintf("%s is required", check.label),
				Details: map[string]any{"field": check.label},
			}
		}
		parsed, err := check.parse(check.value)
		if err != nil || parsed <= 0 {
			return ValidationError{
				Message: fmt.Sprintf("%s is invalid", check.label),
				Details: map[string]any{"field": check.label, "value": check.value},
			}
		}
	}

	requestCPU, _ := parseCPUQuantityToCoresForValidation(input.Requests.CPU)
	limitCPU, _ := parseCPUQuantityToCoresForValidation(input.Limits.CPU)
	requestMemory, _ := parseMemoryQuantityToMiBForValidation(input.Requests.Memory)
	limitMemory, _ := parseMemoryQuantityToMiBForValidation(input.Limits.Memory)
	if limitCPU < requestCPU {
		return ValidationError{
			Message: "resources.limits.cpu must be greater than or equal to resources.requests.cpu",
			Details: map[string]any{"field": "resources.limits.cpu"},
		}
	}
	if limitMemory < requestMemory {
		return ValidationError{
			Message: "resources.limits.memory must be greater than or equal to resources.requests.memory",
			Details: map[string]any{"field": "resources.limits.memory"},
		}
	}

	return nil
}

func parseCPUQuantityToCoresForValidation(raw string) (float64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, nil
	}

	for _, unit := range []struct {
		Suffix     string
		Multiplier float64
	}{
		{Suffix: "n", Multiplier: 1e-9},
		{Suffix: "u", Multiplier: 1e-6},
		{Suffix: "m", Multiplier: 1e-3},
	} {
		if strings.HasSuffix(value, unit.Suffix) {
			number, err := strconv.ParseFloat(strings.TrimSuffix(value, unit.Suffix), 64)
			if err != nil {
				return 0, err
			}
			return number * unit.Multiplier, nil
		}
	}

	return strconv.ParseFloat(value, 64)
}

func parseMemoryQuantityToMiBForValidation(raw string) (float64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, nil
	}

	for _, unit := range []struct {
		Suffix     string
		Multiplier float64
	}{
		{Suffix: "Ki", Multiplier: 1 / 1024.0},
		{Suffix: "Mi", Multiplier: 1},
		{Suffix: "Gi", Multiplier: 1024},
		{Suffix: "Ti", Multiplier: 1024 * 1024},
		{Suffix: "Pi", Multiplier: 1024 * 1024 * 1024},
		{Suffix: "Ei", Multiplier: 1024 * 1024 * 1024 * 1024},
		{Suffix: "K", Multiplier: 1000 / (1024.0 * 1024.0)},
		{Suffix: "M", Multiplier: 1000 * 1000 / (1024.0 * 1024.0)},
		{Suffix: "G", Multiplier: 1000 * 1000 * 1000 / (1024.0 * 1024.0)},
		{Suffix: "T", Multiplier: 1000 * 1000 * 1000 * 1000 / (1024.0 * 1024.0)},
		{Suffix: "P", Multiplier: 1000 * 1000 * 1000 * 1000 * 1000 / (1024.0 * 1024.0)},
		{Suffix: "E", Multiplier: 1000 * 1000 * 1000 * 1000 * 1000 * 1000 / (1024.0 * 1024.0)},
	} {
		if strings.HasSuffix(value, unit.Suffix) {
			number, err := strconv.ParseFloat(strings.TrimSuffix(value, unit.Suffix), 64)
			if err != nil {
				return 0, err
			}
			return number * unit.Multiplier, nil
		}
	}

	return strconv.ParseFloat(value, 64)
}
