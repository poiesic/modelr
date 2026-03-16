package verify

import (
	"fmt"
	"strings"

	"github.com/poiesic/modelr/internal/loader"
	"github.com/poiesic/modelr/internal/model"
)

// InferRoles maps a relationship template's resolve bindings to pattern roles
// by matching binding suffixes to known role conventions.
func InferRoles(tmpl loader.RelationshipDef, rel model.Relationship, m *model.SystemModel) (*RoleMap, error) {
	if tmpl.Pattern == "" {
		return nil, fmt.Errorf("relationship template %q has no pattern", tmpl.Name)
	}

	if tmpl.Pattern != PatternFiniteResource && tmpl.Pattern != PatternFinitePooledResource {
		return nil, fmt.Errorf("unknown pattern %q on template %q", tmpl.Pattern, tmpl.Name)
	}

	rm := &RoleMap{Pattern: tmpl.Pattern}

	// Resolve each binding to its numeric value and map to roles
	for varName, binding := range tmpl.Resolve {
		val, err := resolveBindingValue(binding, rel, m)
		if err != nil {
			continue // unresolvable bindings are handled elsewhere
		}
		intVal, ok := toInt(val)
		if !ok {
			continue
		}

		assignRole(rm, varName, binding, intVal)
	}

	if !rm.Valid() {
		return nil, fmt.Errorf("cannot infer all roles for pattern %q on template %q: missing %v",
			tmpl.Pattern, tmpl.Name, rm.Missing())
	}

	return rm, nil
}

func assignRole(rm *RoleMap, _, binding string, val int) {
	parts := strings.SplitN(binding, ".", 2)
	if len(parts) != 2 {
		return
	}
	scope, prop := parts[0], parts[1]

	switch {
	case strings.HasSuffix(prop, "min_instances") && scope == "upstream":
		if rm.MinInstances == 0 {
			rm.MinInstances = val
		}
	case strings.HasSuffix(prop, "max_instances") && scope == "upstream":
		if rm.MaxInstances == 0 {
			rm.MaxInstances = val
		}
	case prop == "max_connections" && scope == "downstream":
		if rm.ResourceCapacity == 0 {
			rm.ResourceCapacity = val
		}
	case prop == "max_pool_size":
		rm.PoolCapacity = val
	case prop == "conn_establish_ms":
		rm.AcquireTime = val
	case strings.HasSuffix(prop, "_ms") && scope == "edge":
		if rm.OperationTime == 0 {
			rm.OperationTime = val
		}
	}
}

func resolveBindingValue(binding string, rel model.Relationship, m *model.SystemModel) (any, error) {
	parts := strings.SplitN(binding, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid binding %q", binding)
	}
	scope, prop := parts[0], parts[1]

	switch scope {
	case "upstream":
		c := findComponent(m, rel.Upstream)
		if c == nil {
			return nil, fmt.Errorf("upstream %q not found", rel.Upstream)
		}
		return c.Properties[prop], nil
	case "downstream":
		c := findComponent(m, rel.Downstream)
		if c == nil {
			return nil, fmt.Errorf("downstream %q not found", rel.Downstream)
		}
		return c.Properties[prop], nil
	case "edge":
		e := findEdge(m, rel.Upstream, rel.Downstream)
		if e == nil {
			return nil, fmt.Errorf("no edge from %q to %q", rel.Upstream, rel.Downstream)
		}
		return e.Properties[prop], nil
	default:
		return nil, fmt.Errorf("unknown scope %q", scope)
	}
}

func findComponent(m *model.SystemModel, name string) *model.Component {
	for i := range m.Components {
		if m.Components[i].Name == name {
			return &m.Components[i]
		}
	}
	return nil
}

func findEdge(m *model.SystemModel, source, target string) *model.Edge {
	for i := range m.Edges {
		if m.Edges[i].Source == source && m.Edges[i].Target == target {
			return &m.Edges[i]
		}
	}
	return nil
}

func toInt(v any) (int, bool) {
	switch v := v.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case nil:
		return 0, false
	default:
		return 0, false
	}
}
