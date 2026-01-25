// Example: Basic Nim agent server with a custom tool.
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/becomeliminal/nim-go-sdk/server"
	"github.com/becomeliminal/nim-go-sdk/tools"
)

func main() {
	// Get API key from environment
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable is required")
	}

	// Create server
	srv, err := server.New(server.Config{
		AnthropicKey: anthropicKey,
		SystemPrompt: `You are a helpful assistant with access to a weather tool.
When users ask about weather, use the get_weather tool.
Be conversational and helpful.`,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Add a custom tool
	weatherTool := tools.New("get_weather").
		Description("Get the current weather for a location").
		Schema(tools.ObjectSchema(map[string]interface{}{
			"location": tools.StringProperty("The city name (e.g., 'San Francisco')"),
		}, "location")).
		HandlerFunc(func(ctx context.Context, input json.RawMessage) (interface{}, error) {
			var params struct {
				Location string `json:"location"`
			}
			json.Unmarshal(input, &params)

			// In a real app, call a weather API
			return map[string]interface{}{
				"location":    params.Location,
				"temperature": "72°F",
				"conditions":  "Sunny",
				"humidity":    "45%",
			}, nil
		}).
		Build()

	srv.AddTool(weatherTool)

	// Run server
	log.Println("Starting server on :8080")
	log.Println("Connect via WebSocket at ws://localhost:8080/ws")
	if err := srv.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
