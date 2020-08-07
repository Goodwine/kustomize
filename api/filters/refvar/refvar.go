package refvar

import (
	"fmt"
	"strconv"

	"sigs.k8s.io/kustomize/api/filters/fieldspec"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"

	expansion2 "sigs.k8s.io/kustomize/api/internal/accumulator/expansion"
)

// Filter updates $(VAR) style variables with values.
// The fieldSpecs are the places to look for occurrences of $(VAR).
type Filter struct {
	MappingFunc func(string) interface{} `json:"mappingFunc,omitempty" yaml:"mappingFunc,omitempty"`
	FieldSpec   types.FieldSpec          `json:"fieldSpec,omitempty" yaml:"fieldSpec,omitempty"`
}

func (f Filter) Filter(nodes []*yaml.RNode) ([]*yaml.RNode, error) {
	return kio.FilterAll(yaml.FilterFunc(f.run)).Filter(nodes)
}

func (f Filter) run(node *yaml.RNode) (*yaml.RNode, error) {
	err := node.PipeE(fieldspec.Filter{
		FieldSpec: f.FieldSpec,
		SetValue:  f.set,
	})
	return node, err
}

func (f Filter) set(node *yaml.RNode) error {
	if yaml.IsMissingOrNull(node) {
		return nil
	}
	switch node.YNode().Kind {
	case yaml.ScalarNode:
		return f.setScalar(node)
	case yaml.MappingNode:
		return f.setMap(node)
	case yaml.SequenceNode:
		return f.setSeq(node)
	default:
		return fmt.Errorf("invalid type encountered %v", node.YNode().Kind)
	}
}

func updateNodeValue(node *yaml.Node, newValue interface{}) {
	switch newValue := newValue.(type) {
	case int64:
		node.Value = strconv.FormatInt(newValue, 10)
		node.Tag = yaml.NodeTagInt
	case bool:
		node.SetString(strconv.FormatBool(newValue))
		node.Tag = yaml.NodeTagBool
	case float64:
		node.SetString(strconv.FormatFloat(newValue, 'f', -1, 64))
		node.Tag = yaml.NodeTagFloat
	default:
		node.SetString(newValue.(string))
		node.Tag = yaml.NodeTagString
	}
	node.Style = 0
}

func (f Filter) setScalar(node *yaml.RNode) error {
	if node.YNode().Kind != yaml.ScalarNode || node.YNode().Tag != yaml.StringTag {
		// Only process string values
		return nil
	}
	v := expansion2.Expand(node.YNode().Value, f.MappingFunc)
	updateNodeValue(node.YNode(), v)
	return nil
}

func (f Filter) setMap(node *yaml.RNode) error {
	contents := node.YNode().Content
	for i := 0; i < len(contents); i += 2 {
		if contents[i].Kind != yaml.ScalarNode || contents[i].Tag != yaml.StringTag {
			return fmt.Errorf("invalid map key: %s, type: %s", contents[i].Value, contents[i].Tag)
		}
		if contents[i+1].Kind != yaml.ScalarNode || contents[i+1].Tag != yaml.StringTag {
			// value is not a string
			continue
		}
		newValue := expansion2.Expand(contents[i+1].Value, f.MappingFunc)
		updateNodeValue(contents[i+1], newValue)
	}
	return nil
}

func (f Filter) setSeq(node *yaml.RNode) error {
	for _, item := range node.YNode().Content {
		if item.Kind != yaml.ScalarNode || item.Tag != yaml.StringTag {
			// value is not a string
			return fmt.Errorf("invalid value type expect a string")
		}
		newValue := expansion2.Expand(item.Value, f.MappingFunc)
		updateNodeValue(item, newValue)
	}
	return nil
}