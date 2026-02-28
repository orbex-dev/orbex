package compose

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// ComposeFile represents a parsed docker-compose.yml.
type ComposeFile struct {
	Version  string              `yaml:"version"`
	Services map[string]*Service `yaml:"services"`
}

// Service represents a single service in a compose file.
type Service struct {
	Image       string            `yaml:"image"`
	Build       *BuildConfig      `yaml:"build"`
	Command     interface{}       `yaml:"command"` // string or []string
	Entrypoint  interface{}       `yaml:"entrypoint"`
	Environment interface{}       `yaml:"environment"` // map or list
	DependsOn   interface{}       `yaml:"depends_on"`  // list or map
	Ports       []string          `yaml:"ports"`
	Volumes     []string          `yaml:"volumes"`
	Labels      map[string]string `yaml:"labels"`

	// Parsed fields (populated after Parse)
	Name       string            `yaml:"-"`
	ParsedCmd  []string          `yaml:"-"`
	ParsedEnv  map[string]string `yaml:"-"`
	ParsedDeps []string          `yaml:"-"`
}

// BuildConfig represents the build configuration for a service.
type BuildConfig struct {
	Context    string `yaml:"context"`
	Dockerfile string `yaml:"dockerfile"`
}

// Parse parses a docker-compose.yml file.
func Parse(data []byte) (*ComposeFile, error) {
	var cf ComposeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("invalid compose file: %w", err)
	}

	if len(cf.Services) == 0 {
		return nil, fmt.Errorf("no services defined in compose file")
	}

	// Post-process each service
	for name, svc := range cf.Services {
		svc.Name = name

		// Parse command
		svc.ParsedCmd = parseStringOrList(svc.Command)

		// Parse environment
		svc.ParsedEnv = parseEnvironment(svc.Environment)

		// Parse depends_on
		svc.ParsedDeps = parseDependsOn(svc.DependsOn)
	}

	return &cf, nil
}

// StartOrder returns services in dependency order (dependencies first).
func (cf *ComposeFile) StartOrder() ([]string, error) {
	visited := map[string]bool{}
	visiting := map[string]bool{}
	var order []string

	var visit func(name string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		if visiting[name] {
			return fmt.Errorf("circular dependency detected for service %s", name)
		}
		visiting[name] = true

		svc, ok := cf.Services[name]
		if !ok {
			return fmt.Errorf("service %s not found", name)
		}

		for _, dep := range svc.ParsedDeps {
			if err := visit(dep); err != nil {
				return err
			}
		}

		delete(visiting, name)
		visited[name] = true
		order = append(order, name)
		return nil
	}

	// Sort service names for deterministic ordering
	var names []string
	for name := range cf.Services {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if err := visit(name); err != nil {
			return nil, err
		}
	}

	return order, nil
}

// MainService returns the name of the main service.
// Priority: label orbex.main=true > first without depends_on > first service.
func (cf *ComposeFile) MainService() string {
	// Check for orbex.main label
	for name, svc := range cf.Services {
		if svc.Labels["orbex.main"] == "true" {
			return name
		}
	}

	// First service without depends_on
	order, _ := cf.StartOrder()
	for _, name := range order {
		if len(cf.Services[name].ParsedDeps) == 0 {
			// Skip services that look like infrastructure (db, redis, etc.)
			// The main service usually depends on something
			continue
		}
	}

	// Fallback: the last in start order (most likely the main app)
	if len(order) > 0 {
		return order[len(order)-1]
	}

	// Final fallback: first service
	for name := range cf.Services {
		return name
	}
	return ""
}

// parseStringOrList converts a YAML value that could be string or []string.
func parseStringOrList(v interface{}) []string {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case string:
		return []string{"sh", "-c", val}
	case []interface{}:
		var result []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

// parseEnvironment converts YAML environment which can be map or list format.
func parseEnvironment(v interface{}) map[string]string {
	env := map[string]string{}
	if v == nil {
		return env
	}

	switch val := v.(type) {
	case map[string]interface{}:
		for k, v := range val {
			env[k] = fmt.Sprint(v)
		}
	case []interface{}:
		for _, item := range val {
			if s, ok := item.(string); ok {
				parts := splitFirst(s, '=')
				if len(parts) == 2 {
					env[parts[0]] = parts[1]
				} else {
					env[parts[0]] = ""
				}
			}
		}
	}
	return env
}

// parseDependsOn handles both list and map formats of depends_on.
func parseDependsOn(v interface{}) []string {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case []interface{}:
		var deps []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				deps = append(deps, s)
			}
		}
		return deps
	case map[string]interface{}:
		var deps []string
		for name := range val {
			deps = append(deps, name)
		}
		sort.Strings(deps)
		return deps
	}
	return nil
}

// splitFirst splits a string at the first occurrence of sep.
func splitFirst(s string, sep byte) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}
