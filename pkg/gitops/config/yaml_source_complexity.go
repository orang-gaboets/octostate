package config

import "fmt"

type desiredYAMLSourceScanner struct {
	data                    []byte
	maxUnits                int64
	units                   int64
	quote                   byte
	plainTokenActive        bool
	blockScalarParentIndent int
}

// validateDesiredYAMLSourceComplexity estimates parser work from YAML syntax
// before yaml.v3 builds a node tree. Colons are weighted at three units,
// sequence and explicit-key markers and flow separators at two, and
// collection/alias/tag markers at one to account for implicit nodes. The
// exact node and alias-expanded budget is checked afterward.
func validateDesiredYAMLSourceComplexity(data []byte, maxUnits int64) error {
	if maxUnits < 0 {
		return fmt.Errorf("YAML source complexity limit must not be negative")
	}
	scanner := desiredYAMLSourceScanner{
		data:                    data,
		maxUnits:                maxUnits,
		units:                   2, // The document node and its root content node.
		blockScalarParentIndent: -1,
	}
	if scanner.units > scanner.maxUnits {
		return scanner.limitError()
	}
	return scanner.scan()
}

func (s *desiredYAMLSourceScanner) scan() error {
	for index := 0; index < len(s.data); index++ {
		if next, skipped := s.skipBlockScalarLine(index); skipped {
			index = next
			continue
		}
		if s.consumeQuotedByte(&index) || s.skipComment(&index) {
			continue
		}

		character := s.data[index]
		if isYAMLSourceWhitespace(character) {
			continue
		}
		if s.startQuotedScalar(character) {
			continue
		}
		if handled, err := s.startBlockScalar(index); handled {
			if err != nil {
				return err
			}
			index = s.lineEnd(index)
			continue
		}

		weight := desiredYAMLSourceMarkerWeight(character)
		if weight > 0 {
			if err := s.addUnits(weight); err != nil {
				return err
			}
			s.plainTokenActive = false
		} else {
			s.plainTokenActive = true
		}
	}
	return nil
}

func (s *desiredYAMLSourceScanner) skipBlockScalarLine(index int) (int, bool) {
	if s.blockScalarParentIndent < 0 || (index > 0 && s.data[index-1] != '\n') {
		return index, false
	}

	lineEnd := s.lineEnd(index)
	indent := 0
	for index+indent < lineEnd && s.data[index+indent] == ' ' {
		indent++
	}
	if index+indent == lineEnd || indent > s.blockScalarParentIndent {
		return lineEnd, true
	}

	s.blockScalarParentIndent = -1
	return index, false
}

func (s *desiredYAMLSourceScanner) consumeQuotedByte(index *int) bool {
	if s.quote == 0 {
		return false
	}

	character := s.data[*index]
	if s.quote == '"' && character == '\\' {
		*index = *index + 1
		return true
	}
	if s.quote == '\'' && character == '\'' && *index+1 < len(s.data) && s.data[*index+1] == '\'' {
		*index = *index + 1
		return true
	}
	if character == s.quote {
		s.quote = 0
		s.plainTokenActive = true
	}
	return true
}

func (s *desiredYAMLSourceScanner) skipComment(index *int) bool {
	if s.data[*index] != '#' || (*index > 0 && !isYAMLSourceWhitespace(s.data[*index-1])) {
		return false
	}
	for *index < len(s.data) && s.data[*index] != '\n' {
		*index = *index + 1
	}
	return true
}

func (s *desiredYAMLSourceScanner) startQuotedScalar(character byte) bool {
	if (character != '\'' && character != '"') || s.plainTokenActive {
		return false
	}
	s.quote = character
	return true
}

func (s *desiredYAMLSourceScanner) startBlockScalar(index int) (bool, error) {
	character := s.data[index]
	if (character != '|' && character != '>') || s.plainTokenActive || !isYAMLBlockScalarHeader(s.data, index) {
		return false, nil
	}
	if err := s.addUnits(1); err != nil {
		return true, err
	}
	s.blockScalarParentIndent = s.lineIndent(index)
	s.plainTokenActive = false
	return true, nil
}

func (s *desiredYAMLSourceScanner) addUnits(count int64) error {
	if count > s.maxUnits-s.units {
		return s.limitError()
	}
	s.units += count
	return nil
}

func (s *desiredYAMLSourceScanner) limitError() error {
	return fmt.Errorf("desired-state YAML exceeds the supported source complexity limit of %s", desiredStateYAMLSourceUnitsDisplay(s.maxUnits))
}

func (s *desiredYAMLSourceScanner) lineEnd(index int) int {
	for index < len(s.data) && s.data[index] != '\n' {
		index++
	}
	return index
}

func (s *desiredYAMLSourceScanner) lineIndent(index int) int {
	lineStart := index
	for lineStart > 0 && s.data[lineStart-1] != '\n' {
		lineStart--
	}
	indent := 0
	for lineStart+indent < len(s.data) && s.data[lineStart+indent] == ' ' {
		indent++
	}
	return indent
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

func isYAMLSourceWhitespace(character byte) bool {
	switch character {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}

func isYAMLBlockScalarHeader(data []byte, marker int) bool {
	seenChomping := false
	seenIndentation := false
	for index := marker + 1; index < len(data); index++ {
		character := data[index]
		switch {
		case character == '\n':
			return true
		case character == '+' || character == '-':
			if seenChomping {
				return false
			}
			seenChomping = true
		case character >= '1' && character <= '9':
			if seenIndentation {
				return false
			}
			seenIndentation = true
		case isYAMLSourceWhitespace(character):
			for index < len(data) && data[index] != '\n' && isYAMLSourceWhitespace(data[index]) {
				index++
			}
			return index == len(data) || data[index] == '\n' || data[index] == '#'
		default:
			return false
		}
	}
	return true
}

func desiredStateYAMLSourceUnitsDisplay(limit int64) string {
	if limit == maxDesiredStateYAMLSourceUnits {
		return desiredStateYAMLSourceUnitsDisplayValue
	}
	return fmt.Sprintf("%d YAML source complexity units", limit)
}
