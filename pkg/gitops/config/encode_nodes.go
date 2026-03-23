package config

import (
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func appendOptionalStringField(node *yaml.Node, key string, value OptionalString) {
	if !value.Present {
		return
	}
	if value.Null {
		appendMapField(node, key, nullNode())
		return
	}
	appendMapField(node, key, stringNode(value.Value))
}

func appendOptionalInt64Field(node *yaml.Node, key string, value OptionalInt64) {
	if !value.Present {
		return
	}
	if value.Null {
		appendMapField(node, key, nullNode())
		return
	}
	appendMapField(node, key, int64Node(value.Value))
}

func appendOptionalBoolField(node *yaml.Node, key string, value OptionalBool) {
	if !value.Present {
		return
	}
	if value.Null {
		appendMapField(node, key, nullNode())
		return
	}
	appendMapField(node, key, boolNode(value.Value))
}

func appendManagedRepositoryStringField(node *yaml.Node, key string, value OptionalString, fallback string) {
	if value.Present {
		appendOptionalStringField(node, key, value)
		return
	}
	if strings.TrimSpace(fallback) == "" {
		return
	}
	appendMapField(node, key, stringNode(fallback))
}

func appendManagedRepositoryBoolField(node *yaml.Node, key string, value OptionalBool, fallback bool) {
	if value.Present {
		appendOptionalBoolField(node, key, value)
		return
	}
	if !fallback {
		return
	}
	appendMapField(node, key, boolNode(fallback))
}

func appendMapField(node *yaml.Node, key string, value *yaml.Node) {
	node.Content = append(node.Content, stringNode(key), value)
}

func mapNode() *yaml.Node {
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}
}

func sequenceNode(items []*yaml.Node) *yaml.Node {
	if items == nil {
		items = []*yaml.Node{}
	}
	return &yaml.Node{
		Kind:    yaml.SequenceNode,
		Tag:     "!!seq",
		Content: items,
	}
}

func stringSequenceNode(values []string) *yaml.Node {
	items := make([]*yaml.Node, 0, len(values))
	for _, value := range values {
		items = append(items, stringNode(strings.TrimSpace(value)))
	}
	return sequenceNode(items)
}

func stringNode(value string) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: value,
	}
}

func int64Node(value int64) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!int",
		Value: strconv.FormatInt(value, 10),
	}
}

func boolNode(value bool) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!bool",
		Value: strconv.FormatBool(value),
	}
}

func nullNode() *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!null",
		Value: "null",
	}
}
