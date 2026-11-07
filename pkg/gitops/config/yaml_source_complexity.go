package config

import "fmt"

// validateDesiredYAMLSourceComplexity conservatively counts YAML marker bytes
// before yaml.v3 builds a node tree. It deliberately has no YAML lexical state:
// marker-like bytes in scalar text, comments, and block scalars count too.
func validateDesiredYAMLSourceComplexity(data []byte, maxUnits int64) error {
	if maxUnits < 0 {
		return fmt.Errorf("YAML source complexity limit must not be negative")
	}

	units := int64(2) // The document node and its root content node.
	if units > maxUnits {
		return fmt.Errorf("desired-state YAML exceeds the supported source complexity limit of %s", desiredStateYAMLSourceUnitsDisplay(maxUnits))
	}
	for index := 0; index < len(data); index++ {
		weight := desiredYAMLSourceMarkerWeight(data[index])
		if weight == 0 {
			continue
		}
		if weight > maxUnits-units {
			return fmt.Errorf("desired-state YAML exceeds the supported source complexity limit of %s", desiredStateYAMLSourceUnitsDisplay(maxUnits))
		}
		units += weight
	}
	return nil
}

func desiredYAMLSourceMarkerWeight(character byte) int64 {
	switch character {
	case ':':
		return 3 // Mapping node plus a possible implicit key or value.
	case '-', '?':
		return 2 // Sequence entry or explicit mapping-key marker.
	case ',':
		return 2 // Flow-collection separator and a possible empty entry.
	case '[', ']', '{', '}', '*', '&', '!', '|', '>', '%':
		return 1
	default:
		return 0
	}
}

func desiredStateYAMLSourceUnitsDisplay(limit int64) string {
	if limit == maxDesiredStateYAMLSourceUnits {
		return desiredStateYAMLSourceUnitsDisplayValue
	}
	return fmt.Sprintf("%d YAML source complexity units", limit)
}
