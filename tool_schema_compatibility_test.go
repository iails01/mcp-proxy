package main

import (
	"reflect"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestApplyToolSchemaCompatibilityRemovesOptionalEmptyStringEnumValues(t *testing.T) {
	tool := mcp.Tool{
		Name: "kubernetes_event_summary",
		InputSchema: mcp.ToolInputSchema{
			Type:     "object",
			Required: []string{"cluster"},
			Properties: map[string]any{
				"cluster": map[string]any{"type": "string"},
				"sortBy": map[string]any{
					"type":        "string",
					"description": "Sort field",
					"default":     "",
					"enum":        []any{"", "count", "name"},
				},
				"type": map[string]any{
					"type":    "string",
					"default": "",
					"enum":    []string{"", "Warning", "Normal"},
				},
				"format": map[string]any{
					"type":    "string",
					"default": "json",
					"enum":    []any{"json", "table", "yaml"},
				},
			},
		},
	}
	originalProperties := cloneMap(tool.InputSchema.Properties)

	adapted, changes := applyToolSchemaCompatibility(tool, &ToolSchemaCompatibilityConfig{
		RemoveOptionalEmptyStringEnumValues: &OptionalEmptyStringEnumFilterConfig{
			Enabled: true,
			Tools:   []string{"kubernetes_event_summary"},
		},
	})

	if got, want := len(changes), 2; got != want {
		t.Fatalf("expected %d changes, got %d", want, got)
	}
	if changes[0].Property != "sortBy" && changes[1].Property != "sortBy" {
		t.Fatalf("expected sortBy change, got %#v", changes)
	}
	if changes[0].Property != "type" && changes[1].Property != "type" {
		t.Fatalf("expected type change, got %#v", changes)
	}

	sortBy := adapted.InputSchema.Properties["sortBy"].(map[string]any)
	if _, exists := sortBy["default"]; exists {
		t.Fatalf("expected empty default to be removed, got %#v", sortBy)
	}
	if want := []any{"count", "name"}; !reflect.DeepEqual(sortBy["enum"], want) {
		t.Fatalf("unexpected sortBy enum: got %#v want %#v", sortBy["enum"], want)
	}
	if got := sortBy["description"]; got != "Sort field" {
		t.Fatalf("expected unrelated schema fields to be preserved, got %#v", got)
	}

	eventType := adapted.InputSchema.Properties["type"].(map[string]any)
	if want := []string{"Warning", "Normal"}; !reflect.DeepEqual(eventType["enum"], want) {
		t.Fatalf("unexpected type enum: got %#v want %#v", eventType["enum"], want)
	}
	if !reflect.DeepEqual(adapted.InputSchema.Properties["format"], tool.InputSchema.Properties["format"]) {
		t.Fatal("expected unaffected property to remain unchanged")
	}
	if !reflect.DeepEqual(tool.InputSchema.Properties, originalProperties) {
		t.Fatal("expected original tool schema to remain unchanged")
	}
}

func TestApplyToolSchemaCompatibilityRespectsScopeAndSafetyGuards(t *testing.T) {
	tests := []struct {
		name       string
		toolName   string
		required   []string
		enum       []any
		filter     *OptionalEmptyStringEnumFilterConfig
		wantChange bool
	}{
		{
			name:       "matching wildcard",
			toolName:   "kubernetes_top",
			enum:       []any{"", "cpu.util"},
			filter:     &OptionalEmptyStringEnumFilterConfig{Enabled: true, Tools: []string{allToolsPattern}},
			wantChange: true,
		},
		{
			name:     "disabled filter",
			toolName: "kubernetes_top",
			enum:     []any{"", "cpu.util"},
			filter:   &OptionalEmptyStringEnumFilterConfig{Tools: []string{allToolsPattern}},
		},
		{
			name:     "unlisted tool",
			toolName: "kubernetes_top",
			enum:     []any{"", "cpu.util"},
			filter:   &OptionalEmptyStringEnumFilterConfig{Enabled: true, Tools: []string{"cluster_list"}},
		},
		{
			name:     "empty tool list",
			toolName: "kubernetes_top",
			enum:     []any{"", "cpu.util"},
			filter:   &OptionalEmptyStringEnumFilterConfig{Enabled: true},
		},
		{
			name:     "required property",
			toolName: "kubernetes_top",
			required: []string{"sortBy"},
			enum:     []any{"", "cpu.util"},
			filter:   &OptionalEmptyStringEnumFilterConfig{Enabled: true, Tools: []string{allToolsPattern}},
		},
		{
			name:     "empty enum after filtering",
			toolName: "kubernetes_top",
			enum:     []any{""},
			filter:   &OptionalEmptyStringEnumFilterConfig{Enabled: true, Tools: []string{allToolsPattern}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := mcp.Tool{
				Name: tt.toolName,
				InputSchema: mcp.ToolInputSchema{
					Type:     "object",
					Required: tt.required,
					Properties: map[string]any{
						"sortBy": map[string]any{"type": "string", "default": "", "enum": tt.enum},
					},
				},
			}

			adapted, changes := applyToolSchemaCompatibility(tool, &ToolSchemaCompatibilityConfig{
				RemoveOptionalEmptyStringEnumValues: tt.filter,
			})

			if got := len(changes) > 0; got != tt.wantChange {
				t.Fatalf("change mismatch: got %t want %t", got, tt.wantChange)
			}
			if !tt.wantChange && !reflect.DeepEqual(adapted, tool) {
				t.Fatalf("expected tool to remain unchanged, got %#v", adapted)
			}
		})
	}
}
