package wesl

// engine runs the embedded wesl JavaScript. Implementations own their
// JavaScript runtime and need not be safe for concurrent use.
type engine interface {
	// link calls the wesl link() function with params (a JSON-like value built
	// from maps, slices, strings, numbers and bools) and returns the linked WGSL.
	link(params map[string]any) (string, error)
}
