// Example: Nim agent with multiple custom tools.
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/becomeliminal/nim-go-sdk/core"
	"github.com/becomeliminal/nim-go-sdk/server"
	"github.com/becomeliminal/nim-go-sdk/tools"
)

func main() {
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable is required")
	}

	srv, err := server.New(server.Config{
		AnthropicKey: anthropicKey,
		SystemPrompt: `You are a helpful assistant for a task management app.
You can help users manage their tasks and schedule.

Available tools:
- list_tasks: View all tasks
- create_task: Add a new task (requires confirmation)
- complete_task: Mark a task as done
- get_schedule: View today's schedule

Be helpful and proactive about task management.`,
	})
	if err != nil {
		log.Fatal(err)
	}

	// In-memory task storage (use a real database in production)
	taskStore := &TaskStore{
		tasks: make(map[string]*Task),
	}

	// Add tools
	srv.AddTools(
		createListTasksTool(taskStore),
		createCreateTaskTool(taskStore),
		createCompleteTaskTool(taskStore),
		createGetScheduleTool(),
	)

	log.Println("Starting task manager agent on :8080")
	if err := srv.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}

type Task struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	DueDate     string    `json:"due_date,omitempty"`
	Completed   bool      `json:"completed"`
	CreatedAt   time.Time `json:"created_at"`
}

type TaskStore struct {
	tasks  map[string]*Task
	nextID int
}

func (s *TaskStore) Add(task *Task) {
	s.nextID++
	task.ID = string(rune('0' + s.nextID))
	task.CreatedAt = time.Now()
	s.tasks[task.ID] = task
}

func (s *TaskStore) List() []*Task {
	result := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		result = append(result, t)
	}
	return result
}

func (s *TaskStore) Complete(id string) bool {
	if task, ok := s.tasks[id]; ok {
		task.Completed = true
		return true
	}
	return false
}

func createListTasksTool(store *TaskStore) core.Tool {
	return tools.New("list_tasks").
		Description("List all tasks, optionally filtered by completion status").
		Schema(tools.ObjectSchema(map[string]interface{}{
			"completed": tools.BooleanProperty("Filter by completion status"),
		})).
		HandlerFunc(func(ctx context.Context, input json.RawMessage) (interface{}, error) {
			var params struct {
				Completed *bool `json:"completed"`
			}
			json.Unmarshal(input, &params)

			tasks := store.List()
			if params.Completed != nil {
				filtered := make([]*Task, 0)
				for _, t := range tasks {
					if t.Completed == *params.Completed {
						filtered = append(filtered, t)
					}
				}
				tasks = filtered
			}

			return map[string]interface{}{
				"tasks": tasks,
				"count": len(tasks),
			}, nil
		}).
		Build()
}

func createCreateTaskTool(store *TaskStore) core.Tool {
	return tools.New("create_task").
		Description("Create a new task. Requires confirmation.").
		Schema(tools.ObjectSchema(map[string]interface{}{
			"title":       tools.StringProperty("Task title"),
			"description": tools.StringProperty("Optional task description"),
			"due_date":    tools.StringProperty("Optional due date (YYYY-MM-DD)"),
		}, "title")).
		RequiresConfirmation().
		SummaryTemplate("Create task: {{.title}}").
		HandlerFunc(func(ctx context.Context, input json.RawMessage) (interface{}, error) {
			var params struct {
				Title       string `json:"title"`
				Description string `json:"description"`
				DueDate     string `json:"due_date"`
			}
			json.Unmarshal(input, &params)

			task := &Task{
				Title:       params.Title,
				Description: params.Description,
				DueDate:     params.DueDate,
			}
			store.Add(task)

			return map[string]interface{}{
				"success": true,
				"message": "Task created successfully",
				"task":    task,
			}, nil
		}).
		Build()
}

func createCompleteTaskTool(store *TaskStore) core.Tool {
	return tools.New("complete_task").
		Description("Mark a task as completed").
		Schema(tools.ObjectSchema(map[string]interface{}{
			"task_id": tools.StringProperty("ID of the task to complete"),
		}, "task_id")).
		HandlerFunc(func(ctx context.Context, input json.RawMessage) (interface{}, error) {
			var params struct {
				TaskID string `json:"task_id"`
			}
			json.Unmarshal(input, &params)

			if store.Complete(params.TaskID) {
				return map[string]interface{}{
					"success": true,
					"message": "Task marked as completed",
				}, nil
			}
			return map[string]interface{}{
				"success": false,
				"error":   "Task not found",
			}, nil
		}).
		Build()
}

func createGetScheduleTool() core.Tool {
	return tools.New("get_schedule").
		Description("Get today's schedule and upcoming events").
		Schema(tools.ObjectSchema(map[string]interface{}{})).
		HandlerFunc(func(ctx context.Context, input json.RawMessage) (interface{}, error) {
			// Mock schedule data
			return map[string]interface{}{
				"date": time.Now().Format("2006-01-02"),
				"events": []map[string]string{
					{"time": "9:00 AM", "title": "Team standup"},
					{"time": "2:00 PM", "title": "Project review"},
					{"time": "4:00 PM", "title": "1:1 with manager"},
				},
			}, nil
		}).
		Build()
}
