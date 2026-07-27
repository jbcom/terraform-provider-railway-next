package changeset

import "sort"

// VariableCollection produces one Railway change set containing every owned
// upsert and deletion. Applying this set is a single logical commit and can
// therefore trigger at most one service deployment.
func VariableCollection(
	serviceName string,
	before map[string]string,
	after map[string]string,
) Set {
	address := "service." + serviceName
	changes := make([]Change, 0, len(before)+len(after))

	keys := make([]string, 0, len(after))
	for key := range after {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := after[key]
		previous, existed := before[key]
		if existed && previous == value {
			continue
		}
		change := Change{
			Kind:         "variable.set",
			Address:      address,
			Variable:     key,
			After:        raw(VariableValue{Type: "literal", Value: value}),
			Path:         "resources." + address + ".variables." + key,
			Summary:      variableSummary(existed, serviceName, key),
			Severity:     "safe",
			DeployEffect: "deploy",
		}
		if existed {
			change.Before = raw(VariableValue{Type: "literal", Value: previous})
		}
		changes = append(changes, change)
	}

	keys = keys[:0]
	for key := range before {
		if _, owned := after[key]; !owned {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		changes = append(changes, Change{
			Kind:         "variable.delete",
			Address:      address,
			Variable:     key,
			Previous:     raw(VariableValue{Type: "literal", Value: before[key]}),
			Path:         "resources." + address + ".variables." + key,
			Summary:      "Delete variable " + serviceName + "." + key,
			Severity:     "destructive",
			DeployEffect: "deploy",
		})
	}

	return Set{Version: Version, Changes: changes, Diagnostics: []Diagnostic{}}
}

func variableSummary(existed bool, serviceName, key string) string {
	if existed {
		return "Update variable " + serviceName + "." + key
	}
	return "Set variable " + serviceName + "." + key
}
