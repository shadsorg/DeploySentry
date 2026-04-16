package flags

// YAMLExport is the top-level structure for the exported YAML config file.
type YAMLExport struct {
	Version      int               `yaml:"version" json:"version"`
	Project      string            `yaml:"project" json:"project"`
	Application  string            `yaml:"application" json:"application"`
	ExportedAt   string            `yaml:"exported_at" json:"exported_at"`
	Environments []YAMLEnvironment `yaml:"environments" json:"environments"`
	Flags        []YAMLFlag        `yaml:"flags" json:"flags"`
}

// YAMLEnvironment represents an environment entry in the export.
type YAMLEnvironment struct {
	ID           string `yaml:"id" json:"id"`
	Name         string `yaml:"name" json:"name"`
	IsProduction bool   `yaml:"is_production" json:"is_production"`
}

// YAMLFlag represents a single feature flag in the export.
type YAMLFlag struct {
	Key          string                 `yaml:"key" json:"key"`
	Name         string                 `yaml:"name" json:"name"`
	FlagType     string                 `yaml:"flag_type" json:"flag_type"`
	Category     string                 `yaml:"category" json:"category"`
	DefaultValue string                 `yaml:"default_value" json:"default_value"`
	IsPermanent  bool                   `yaml:"is_permanent" json:"is_permanent"`
	ExpiresAt    string                 `yaml:"expires_at,omitempty" json:"expires_at,omitempty"`
	Environments map[string]YAMLFlagEnv `yaml:"environments" json:"environments"`
	Rules        []YAMLRule             `yaml:"rules,omitempty" json:"rules,omitempty"`
}

// YAMLFlagEnv represents the per-environment state of a flag.
type YAMLFlagEnv struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Value   string `yaml:"value" json:"value"`
}

// YAMLRule represents a targeting rule in the export.
type YAMLRule struct {
	Attribute    string          `yaml:"attribute" json:"attribute"`
	Operator     string          `yaml:"operator" json:"operator"`
	TargetValues []string        `yaml:"target_values" json:"target_values"`
	Value        string          `yaml:"value" json:"value"`
	Priority     int             `yaml:"priority" json:"priority"`
	Environments map[string]bool `yaml:"environments" json:"environments"`
}
