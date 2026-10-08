package mcp

// intArg coerces one JSON-decoded tool argument onto a config field.
// Numbers arrive as float64; unknown keys are ignored so newer clients stay
// compatible with this server.
func intArg(args map[string]any, key string, dst *int) {
	v, ok := args[key]
	if !ok {
		return
	}
	switch n := v.(type) {
	case float64:
		*dst = int(n)
	case int:
		*dst = n
	}
}

func stringArg(args map[string]any, key string, dst *string) {
	if s, ok := args[key].(string); ok {
		*dst = s
	}
}

func boolArg(args map[string]any, key string, dst *bool) {
	if b, ok := args[key].(bool); ok {
		*dst = b
	}
}
