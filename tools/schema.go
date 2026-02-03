package tools

// Schema helpers for building JSON Schema definitions.

// ObjectSchema creates an object schema with the given properties.
func ObjectSchema(properties map[string]interface{}, required ...string) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// StringProperty creates a string property with optional description.
func StringProperty(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "string",
		"description": description,
	}
}

// StringEnumProperty creates a string property with allowed values.
func StringEnumProperty(description string, values ...string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "string",
		"description": description,
		"enum":        values,
	}
}

// NumberProperty creates a number property with optional description.
func NumberProperty(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "number",
		"description": description,
	}
}

// IntegerProperty creates an integer property with optional description.
func IntegerProperty(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "integer",
		"description": description,
	}
}

// BooleanProperty creates a boolean property with optional description.
func BooleanProperty(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "boolean",
		"description": description,
	}
}

// ArrayProperty creates an array property with the given item type.
func ArrayProperty(description string, itemType map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":        "array",
		"description": description,
		"items":       itemType,
	}
}

// WithThought adds a thought parameter to an existing schema.
// If requireThought is true, "thought" is added to the required array.
func WithThought(schema map[string]interface{}, requireThought bool) map[string]interface{} {
	// Clone schema
	result := make(map[string]interface{})
	for k, v := range schema {
		result[k] = v
	}

	// Get or create properties map
	props, ok := result["properties"].(map[string]interface{})
	if !ok {
		props = make(map[string]interface{})
		result["properties"] = props
	}

	// Add thought property
	props["thought"] = StringProperty(
		"Your reasoning about why you're using this tool and what you expect to accomplish. " +
			"For write operations, explain your decision-making process.",
	)

	// Add to required array if needed
	if requireThought {
		required, ok := result["required"].([]string)
		if !ok {
			required = []string{}
		}
		result["required"] = append(required, "thought")
	}

	return result
}

// BuildSchemaWithThought creates an ObjectSchema and adds thought support in one call.
func BuildSchemaWithThought(properties map[string]interface{}, requireThought bool, required ...string) map[string]interface{} {
	schema := ObjectSchema(properties, required...)
	return WithThought(schema, requireThought)
}
