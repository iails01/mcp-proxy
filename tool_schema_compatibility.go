package main

import "github.com/mark3labs/mcp-go/mcp"

const allToolsPattern = "*"

type toolSchemaCompatibilityChange struct {
	Property string
}

func applyToolSchemaCompatibility(tool mcp.Tool, config *ToolSchemaCompatibilityConfig) (mcp.Tool, []toolSchemaCompatibilityChange) {
	if config == nil {
		return tool, nil
	}

	filter := config.RemoveOptionalEmptyStringEnumValues
	if filter == nil || !filter.Enabled || !toolMatchesFilter(tool.Name, filter.Tools) {
		return tool, nil
	}

	required := make(map[string]struct{}, len(tool.InputSchema.Required))
	for _, property := range tool.InputSchema.Required {
		required[property] = struct{}{}
	}

	var changes []toolSchemaCompatibilityChange
	var properties map[string]any
	for propertyName, rawProperty := range tool.InputSchema.Properties {
		if _, isRequired := required[propertyName]; isRequired {
			continue
		}

		property, ok := rawProperty.(map[string]any)
		if !ok {
			continue
		}

		filteredEnum, removed, remaining := removeEmptyStringEnumValue(property["enum"])
		if !removed || remaining == 0 {
			continue
		}

		if properties == nil {
			properties = cloneMap(tool.InputSchema.Properties)
		}
		adaptedProperty := cloneMap(property)
		adaptedProperty["enum"] = filteredEnum
		if defaultValue, ok := adaptedProperty["default"].(string); ok && defaultValue == "" {
			delete(adaptedProperty, "default")
		}
		properties[propertyName] = adaptedProperty
		changes = append(changes, toolSchemaCompatibilityChange{Property: propertyName})
	}

	if len(changes) > 0 {
		tool.InputSchema.Properties = properties
	}
	return tool, changes
}

func toolMatchesFilter(toolName string, tools []string) bool {
	for _, configuredTool := range tools {
		if configuredTool == allToolsPattern || configuredTool == toolName {
			return true
		}
	}
	return false
}

func removeEmptyStringEnumValue(value any) (any, bool, int) {
	switch enum := value.(type) {
	case []any:
		filtered := make([]any, 0, len(enum))
		removed := false
		for _, item := range enum {
			if stringValue, ok := item.(string); ok && stringValue == "" {
				removed = true
				continue
			}
			filtered = append(filtered, item)
		}
		return filtered, removed, len(filtered)
	case []string:
		filtered := make([]string, 0, len(enum))
		removed := false
		for _, item := range enum {
			if item == "" {
				removed = true
				continue
			}
			filtered = append(filtered, item)
		}
		return filtered, removed, len(filtered)
	default:
		return value, false, 0
	}
}

func cloneMap(source map[string]any) map[string]any {
	clone := make(map[string]any, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}
