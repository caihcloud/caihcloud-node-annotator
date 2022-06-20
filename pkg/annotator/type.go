package annotator

// ResourceSpec represents single resource.
type ResourceSpec struct {
	// Name of the resource.
	Name string
	// Expression of the metric
	Expr string
	// Weight of the resource.
	Weight float64
	// Threshold of the resource.
	Threshold float64
}

// metrics spec
type Config struct {
	Metrics []ResourceSpec `json:"pluginConfig,omitempty"`
}
