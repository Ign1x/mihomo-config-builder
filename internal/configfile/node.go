package configfile

import (
	"fmt"
	"sort"
	"strconv"

	"gopkg.in/yaml.v3"
)

func ToNode(v any, sortKeys bool) (*yaml.Node, error) {
	n := &yaml.Node{}
	if err := encodeIntoNode(n, v, sortKeys); err != nil {
		return nil, err
	}
	return n, nil
}

func encodeIntoNode(n *yaml.Node, v any, sortKeys bool) error {
	switch t := v.(type) {
	case map[string]any:
		n.Kind = yaml.MappingNode
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		if sortKeys {
			sort.Strings(keys)
		}
		for _, k := range keys {
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}
			valNode := &yaml.Node{}
			if err := encodeIntoNode(valNode, t[k], sortKeys); err != nil {
				return err
			}
			n.Content = append(n.Content, keyNode, valNode)
		}
		return nil
	case []any:
		n.Kind = yaml.SequenceNode
		for _, item := range t {
			itemNode := &yaml.Node{}
			if err := encodeIntoNode(itemNode, item, sortKeys); err != nil {
				return err
			}
			n.Content = append(n.Content, itemNode)
		}
		return nil
	case string:
		n.Kind = yaml.ScalarNode
		n.Tag = "!!str"
		n.Value = t
		return nil
	case bool:
		n.Kind = yaml.ScalarNode
		n.Tag = "!!bool"
		n.Value = strconv.FormatBool(t)
		return nil
	case int:
		n.Kind = yaml.ScalarNode
		n.Tag = "!!int"
		n.Value = strconv.Itoa(t)
		return nil
	case int64:
		n.Kind = yaml.ScalarNode
		n.Tag = "!!int"
		n.Value = strconv.FormatInt(t, 10)
		return nil
	case float64:
		n.Kind = yaml.ScalarNode
		n.Tag = "!!float"
		n.Value = strconv.FormatFloat(t, 'f', -1, 64)
		return nil
	case nil:
		n.Kind = yaml.ScalarNode
		n.Tag = "!!null"
		n.Value = "null"
		return nil
	default:
		return fmt.Errorf("unsupported value type: %T", v)
	}
}
