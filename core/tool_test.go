package core

import (
	"context"
	"encoding/json"
	"testing"
)

func TestBaseTool_GetSummary(t *testing.T) {
	tests := []struct {
		name            string
		summaryTemplate string
		input           map[string]interface{}
		want            string
	}{
		{
			name:            "simple template with string field",
			summaryTemplate: "Send {{.amount}} {{.currency}} to {{.recipient}}",
			input: map[string]interface{}{
				"amount":    "50.00",
				"currency":  "USD",
				"recipient": "@alice",
			},
			want: "Send 50.00 USD to @alice",
		},
		{
			name:            "template with integer field",
			summaryTemplate: "Execute contract call on chain {{.chain_id}} to {{.to}}",
			input: map[string]interface{}{
				"chain_id": 42161,
				"to":       "0xaf88d065e77c8cC2239327C5EDb3A432268e5831",
			},
			want: "Execute contract call on chain 42161 to 0xaf88d065e77c8cC2239327C5EDb3A432268e5831",
		},
		{
			name:            "template with missing field",
			summaryTemplate: "Deposit {{.amount}} {{.currency}} into savings",
			input: map[string]interface{}{
				"amount": "100",
			},
			want: "Deposit 100 <no value> into savings",
		},
		{
			name:            "empty template",
			summaryTemplate: "",
			input: map[string]interface{}{
				"amount": "100",
			},
			want: "",
		},
		{
			name:            "no template variables",
			summaryTemplate: "Confirm this action",
			input: map[string]interface{}{
				"amount": "100",
			},
			want: "Confirm this action",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := NewBaseTool(ToolDefinition{
				ToolName:        "test_tool",
				SummaryTemplate: tt.summaryTemplate,
			}, nil)

			inputBytes, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Failed to marshal input: %v", err)
			}

			got := tool.GetSummary(inputBytes)
			if got != tt.want {
				t.Errorf("GetSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBaseTool_GetSummary_InvalidJSON(t *testing.T) {
	tool := NewBaseTool(ToolDefinition{
		ToolName:        "test_tool",
		SummaryTemplate: "Send {{.amount}} to {{.recipient}}",
	}, nil)

	// Invalid JSON should return template as-is
	got := tool.GetSummary(json.RawMessage(`invalid json`))
	want := "Send {{.amount}} to {{.recipient}}"
	if got != want {
		t.Errorf("GetSummary() with invalid JSON = %q, want %q", got, want)
	}
}

func TestBaseTool_Execute_PassesAllToolParams(t *testing.T) {
	var captured *ToolParams

	tool := NewBaseTool(ToolDefinition{
		ToolName: "test_tool",
	}, func(ctx context.Context, params *ToolParams) (*ToolResult, error) {
		captured = params
		return &ToolResult{Success: true}, nil
	})

	input := json.RawMessage(`{"key":"value"}`)
	params := &ToolParams{
		UserID:         "user-123",
		Input:          input,
		ConfirmationID: "conf-456",
		RequestID:      "req-789",
		ConversationID: "conv-aaa",
		MessageID:      "msg-bbb",
	}

	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if !result.Success {
		t.Fatal("Execute() returned unsuccessful result")
	}
	if captured == nil {
		t.Fatal("handler was not called")
	}
	if captured.UserID != "user-123" {
		t.Errorf("UserID = %q, want %q", captured.UserID, "user-123")
	}
	if captured.ConfirmationID != "conf-456" {
		t.Errorf("ConfirmationID = %q, want %q", captured.ConfirmationID, "conf-456")
	}
	if captured.RequestID != "req-789" {
		t.Errorf("RequestID = %q, want %q", captured.RequestID, "req-789")
	}
	if captured.ConversationID != "conv-aaa" {
		t.Errorf("ConversationID = %q, want %q", captured.ConversationID, "conv-aaa")
	}
	if captured.MessageID != "msg-bbb" {
		t.Errorf("MessageID = %q, want %q", captured.MessageID, "msg-bbb")
	}
	if string(captured.Input) != string(input) {
		t.Errorf("Input = %s, want %s", captured.Input, input)
	}
}

func TestBaseTool_GetSummary_InvalidTemplate(t *testing.T) {
	tool := NewBaseTool(ToolDefinition{
		ToolName:        "test_tool",
		SummaryTemplate: "Send {{.amount} to {{.recipient}}", // Missing closing brace
	}, nil)

	input := map[string]interface{}{
		"amount":    "50.00",
		"recipient": "@alice",
	}
	inputBytes, _ := json.Marshal(input)

	// Invalid template should return template as-is
	got := tool.GetSummary(inputBytes)
	want := "Send {{.amount} to {{.recipient}}"
	if got != want {
		t.Errorf("GetSummary() with invalid template = %q, want %q", got, want)
	}
}
