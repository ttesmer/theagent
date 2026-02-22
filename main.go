package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"net/http"
	"encoding/json"
	"bytes"

)

var (
    version = "dev"    // default, overridden at build time
    commit  = "none"
    date    = "unknown"
)

func main() {
  fmt.Printf("The Agent\u2122 %s (commit: %s, built: %s)\n", version, commit, date)
	client := NewClient()

	scanner := bufio.NewScanner(os.Stdin)
	getUserMessage := func() (string, bool) {
		if !scanner.Scan() {
			return "", false
		}
		return scanner.Text(), true
	}
	agent := NewAgent(client, getUserMessage)
	err := agent.Run(context.TODO())
	if err != nil {
		fmt.Print("Error:\n", err.Error())
	}
}

type Agent struct {
	client *Client
	getUserMessage func() (string, bool)
}

type Client struct {}

func NewAgent(client *Client, getUserMessage func() (string, bool)) *Agent {
	return &Agent{
		client: client,
		getUserMessage: getUserMessage,
	}
}

func NewClient() *Client {
	return &Client{}
}


type Choice struct {
    Index        int     `json:"index"`
    FinishReason string  `json:"finish_reason"` // "tool_calls", "stop"
    Message      Message `json:"message"`
}

type OpenRouterResponse struct {
    Choices []Choice `json:"choices"`
}

type MessageBody struct {
	Model 		string    `json:"model"`
	MaxTokens int64     `json:"max_tokens"`
	Messages  []Message `json:"messages"`
	Tools 		[]Tool    `json:"tools,omitempty"`
}

type Tool struct {
	Type 		 string      `json:"type"` // "function"
	Function FunctionDef `json:"function"`
}

// This is for the LLM's context (I think)
type FunctionDef struct { 
	Name 				string `json:"name"`
	Description string `json:"description"`
	Parameters  any 	 `json:"parameters"` // JSON Schema object
}

type ToolCall struct {
	ID 			 string 		  `json:"id"`
	Type 		 string 		  `json:"type"` // "function"
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name 			string `json:"name"`
	Arguments string `json:"arguments"` // JSON string of args
}


type Message struct {
	Content   string     `json:"content"`
	Role      string     `json:"role"` // "user", "assistant" or "tool"(!)
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

func NewUserMessage(input string) Message {
	return Message{
		Content: input,
		Role: "user",
	}
}


func (a *Agent) runInference(ctx context.Context, conversation []Message) ([]Choice, error) {
	var tools = []Tool{
		{
			Type: "function",
			Function: FunctionDef{
				Name: "run_command",
				Description: "Execute a shell command",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{"type": "string", "description": "Command to run"},
					},
					"required": []string{"command"},
				},
			},
		},
	}
	model := os.Getenv("MODEL")
	if model == "" {
		model = "minimax/minimax-m2.5"
	}
	response, err := a.client.Generate(ctx, MessageBody{
		Model: model,
		Messages: conversation,
		MaxTokens: int64(1024),
		Tools: tools,
	})
	return response.Choices, err
}

func (c *Client) Generate(ctx context.Context, msg MessageBody) (OpenRouterResponse, error){
	API_URL := "https://openrouter.ai/api/v1/chat/completions"
	jsonBytes, _ := json.Marshal(msg)
	body := bytes.NewReader(jsonBytes)
	req, err := http.NewRequest("POST", API_URL, body)
	API_KEY := os.Getenv("OPENROUTER_API_KEY")
	if API_KEY == "" {
		return OpenRouterResponse{}, fmt.Errorf("No API Key!\n")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+API_KEY)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return OpenRouterResponse{}, err
	}
	defer resp.Body.Close()

	var result OpenRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return OpenRouterResponse{}, err
	}

	if len(result.Choices) == 0 {
			return OpenRouterResponse{}, fmt.Errorf("no choices in response\n")
	}

	return OpenRouterResponse{
		Choices: result.Choices,
	}, err
}

func (a *Agent) Run(ctx context.Context) error {
	conversation := []Message{}

	fmt.Println("Chat with The Agent (use 'ctrl-c' to quit)")
	var doneUsingTools bool = true
	for {
		if doneUsingTools {
			fmt.Print("\u001b[38;5;216mYou\u001b[0m: ")
			userInput, ok := a.getUserMessage()
			if !ok { break }
			userMessage := NewUserMessage(userInput)
			conversation = append(conversation, userMessage)
		}

		choices, err := a.runInference(ctx, conversation)
		if err != nil {
			return err
		}
		llmMessage := choices[0].Message
		conversation = append(conversation, llmMessage)
		if len(llmMessage.ToolCalls) == 0 {
			fmt.Printf("\u001b[38;5;80mThe Agent\u001b[0m: %s\n", llmMessage.Content)
		  doneUsingTools = true
		} else {
			for _, tool_call := range llmMessage.ToolCalls {
				fmt.Printf("\u001b[38;5;80mSystem\u001b[0m: Agent requests action.\nid: %s\ntype: %s\nname: %s\narguments: %s\n", tool_call.ID, tool_call.Type, tool_call.Function.Name, tool_call.Function.Arguments)
				decided  := false
				for !decided {
					fmt.Printf("\u001b[38;5;80mSystem\u001b[0m: Allow? \u001b[3my/n\u001b[0m\n")
					userInput, ok := a.getUserMessage()
					if !ok { break }
					switch userInput {
					case "n":
							fmt.Printf("\u001b[38;5;80mSystem\u001b[0m: Add a note for agent about refusal?\n")
							fmt.Print("\u001b[38;5;216mYou\u001b[0m: ")
							userInput, ok := a.getUserMessage()
							if !ok { break }
							conversation = append(conversation, 
								Message{
									Role: "tool",
									Content: fmt.Sprintf("User refused tool_call %s (arguments: %s) with this note: \"%s\"", tool_call.Function.Name, tool_call.Function.Arguments, userInput),
								},
							)
							decided = true
						case "y":
							fmt.Printf("\u001b[38;5;80mSystem\u001b[0m: Access granted, executing command.\n")
							result := executeTool(tool_call.Function.Name, tool_call.Function.Arguments)
							toolMsg := Message{
								Role: "tool",
								Content: result,
							}
							conversation = append(conversation, toolMsg)
							decided = true
						default:
							continue
					}
				}
			}
			doneUsingTools = false
		}
	}

	return nil
}

func executeTool(name string, args string) string {
	switch name {
	case "run_command":
		var params struct{ Command string }
		json.Unmarshal([]byte(args), &params) // see above what FunctionCall.Arguments is
		out, _ := exec.Command("sh", "-c", params.Command).Output()
		fmt.Printf("\u001b[38;5;80m$\u001b[0m %s\n%s\n", params.Command, out)
		return string(out)
	}
	fmt.Printf("\u001b[38;5;80mSystem\u001b[0m: Function %s is not implemented or not in the cases.\n", name)
	return "unknown tool"
}
