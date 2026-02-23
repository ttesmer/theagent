package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"io"
	"os/exec"
	"net/http"
	"encoding/json"
	"bytes"
	//"path/filepath"

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
		ID			 string   `json:"id"` 			// e.g. "gen-188832332-odwijod23"
		Provider string   `json:"provider"` // e.g. "Moonshot AI"
		Model 	 string   `json:"model"`    // e.g. "moonshotai/kimi-k2.5"
		Object   string   `json:"object"`   // e.g. "chat.completion" (not sure what else)
		Created  int64    `json:"created"`  // UNIX timestamp in seconds (date -r <date>)
    Choices  []Choice `json:"choices"`
		Usage    struct {
			PromptTokens     int     `json:"prompt_tokens"`
			CompletionTokens int     `json:"completion_tokens"`
			TotalTokens      int     `json:"total_tokens"`
			Cost             float32 `json:"cost"` // other dtype maybe
			// other fields, maybe add later
		}`json:"usage"`
		SystemFingerprint string `json:"system_fingerprint"` // no idea about this
		Error    *OpenRouterError `json:"error,omitempty"`
}

type OpenRouterError struct {
	Message  string `json:"message"`
	Code     int    `json:"code"`
	Metadata struct {
		Raw string `json:"raw"`
		ProviderName string `json:"provider_name"`
	} `json:"metadata"`
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
	Role         string     `json:"role"` // "user", "assistant" or "tool"(!)
	ToolName   	 string     `json:"name,omitempty"`
	ToolCallId 	 string     `json:"tool_call_id,omitempty"`
	Content      string     `json:"content"`
	Refusal      bool       `json:"refusal,omitempty"`
	Reasoning    string     `json:"reasoning,omitempty"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
}

func NewUserMessage(input string) Message {
	return Message{
		Content: input,
		Role: "user",
	}
}


func (c *Client) Generate(ctx context.Context, msg MessageBody) (OpenRouterResponse, error){
	API_URL := "https://openrouter.ai/api/v1/chat/completions"
	API_KEY := os.Getenv("OPENROUTER_API_KEY")
	if API_KEY == "" {
		return OpenRouterResponse{}, fmt.Errorf("No API Key!\n")
	}

	reqBodyBytes, _ := json.Marshal(msg)

	// Debug json request body
	if os.Getenv("DEBUG") == "1" { 
		var prettyReq bytes.Buffer
		json.Indent(&prettyReq, reqBodyBytes, "", "  ")
		fmt.Printf("DEBUG:\nRequest body:\n%s\n", prettyReq.String()) 
	}

	reqBody := bytes.NewReader(reqBodyBytes)

	req, err := http.NewRequest("POST", API_URL, reqBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+API_KEY)

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return OpenRouterResponse{}, err
	}

	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)

	// Debug http response body
	if os.Getenv("DEBUG") == "1" {
		var prettyResp bytes.Buffer
		err = json.Indent(&prettyResp, responseBody, "", "  ")
		if err != nil { fmt.Printf("JSON indenting failed") }
		fmt.Printf("DEBUG:\nResponse body:\n%s\n", prettyResp.String()) 
	}

	var result OpenRouterResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
			return OpenRouterResponse{}, err
	}

	// OpenRouter error handling
	if result.Error != nil {
		respError := result.Error
		return OpenRouterResponse{}, fmt.Errorf(
			"OpenRouter: %s (code: %d, provider: %s)\nRaw message from provider:\n%s\n",
			respError.Message, respError.Code, respError.Metadata.ProviderName, respError.Metadata.Raw)
	}
	return result, err
}

func (a *Agent) runInference(ctx context.Context, conversation []Message) (OpenRouterResponse, error) {
	// put this into different file 
	// along with executeTool function
	// and Tool struct
	// and FunctionCall struct
	// then just import `tools` and `executeTool`
	// et voila, just add a tool definition here
	// and a new pattern match in executeTool
	// and you get a new tool!
	var tools = []Tool{
		{
			Type: "function",
			Function: FunctionDef{
				Name: "run_command",
				Description: "Execute a shell command",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{
							"type": "string", 
							"description": "Command to run",
						},
					},
					"required": []string{"command"},
				},
			},
		},
		//{
		//	Type: "function",
		//	Function: FunctionDef{
		//		Name: "display_to_user",
		//		Description: "Render your gathered information in Markdown to user by plugging it into `display_info_user` argument as a string",
		//		Parameters: map[string]any{
		//			"type": "object",
		//			"properties": map[string]any{
		//				"display_info_user": map[string]any{
		//					"type": "string", 
		//					"description": "Markdown information to render to the user",
		//				},
		//				"display_info_subagent": map[string]any{
		//					"type": "string", 
		//					"description": "Metadata the subagent responsible for displaying the data might be interested in (optional)",
		//				},
		//			},
		//			"required": []string{"display_info"},
		//		},
		//	},
		//},
	}
	model := os.Getenv("MODEL")
	if model == "" {
		model = "moonshotai/kimi-k2.5"
	}
	response, err := a.client.Generate(ctx, MessageBody{
		Model: model,
		Messages: conversation,
		//MaxTokens: int64(1024),
		Tools: tools,
	})
	return response, err
}

func (a *Agent) Run(ctx context.Context) error {
	conversation := []Message{}

	fmt.Println("Chat with The Agent (use 'ctrl-c' to quit)")
	var doneUsingTools = true
	for {
		if doneUsingTools {
			fmt.Print("\u001b[38;5;216mYou\u001b[0m: ")
			userInput, ok := a.getUserMessage()
			if !ok { break }
			userMessage := NewUserMessage(userInput)
			conversation = append(conversation, userMessage)
		}

		httpResponse, err := a.runInference(ctx, conversation)
		if err != nil { return err }

		llmMessage := httpResponse.Choices[0].Message
		conversation = append(conversation, llmMessage)

		// print reasoning trace
		if llmMessage.Reasoning != "" {
			fmt.Printf(
				"\u001b[38;5;79mThe Agent's thoughts\u001b[0m: \u001b[38;5;245m%s\u001b[0m\n", 
				llmMessage.Reasoning,
			)
		}

		switch httpResponse.Choices[0].FinishReason {
		case "tool_calls":
			// loop over tool calls
			doneUsingTools = false
			toolCalls := llmMessage.ToolCalls
			for _, toolCall := range toolCalls {
				toolMessage := handleToolCall(conversation, toolCall, a)
				conversation = append(conversation, toolMessage)
			}
		case "stop":
			// respond if no more tool calls
			fmt.Printf(
				"\u001b[38;5;81mThe Agent\u001b[0m: %s\n", 
				llmMessage.Content,
			)
			doneUsingTools = true
		} 
	}
	return nil
}

func handleToolCall(conversation []Message, toolCall ToolCall, a *Agent) Message {
	toolResult := ""
	if toolCall.Function.Name == "run_command" {
		printShellCommand(toolCall)
		approved, refusalNote := getUserApproval(a)
		if approved {
			toolResult = executeTool(toolCall.Function)
		} else {
			toolResult = fmt.Sprintf(
				"User refused tool call %s with note: \"%s\"", 
				toolCall.ID, refusalNote,
			)
		}
	}
	toolMsg := Message{
		Role: "tool",
		ToolName: toolCall.Function.Name,
		ToolCallId: toolCall.ID,
		Content: toolResult,
	}
	return toolMsg
}

func executeTool(function FunctionCall) string {
	switch function.Name {
	case "run_command":
		var params struct{ Command string }
		json.Unmarshal([]byte(function.Arguments), &params)
		cmd := exec.Command("sh", "-c", params.Command)
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return fmt.Sprintf("Error: %v\n", err)
		}
		fmt.Printf("%s", out)
		return string(out)
	case "display_to_user":
		var params struct{ 
			displayInfoUser string
		  displayInfoSubagent string
		}
		json.Unmarshal([]byte(function.Arguments), &params)
		fmt.Printf("For user display: %s ", params.displayInfoUser)
		fmt.Printf("For subagent: %s ", params.displayInfoSubagent)
		return "Passed display info along to subagent"
	}
	fmt.Printf("\u001b[38;5;80mSystem\u001b[0m: ")
	fmt.Printf("Function %s is not implemented or not in the cases.\n", function.Name)
	return "unknown tool"
}

func printShellCommand(toolCall ToolCall) {
		var shell struct { Command string }
		json.Unmarshal([]byte(toolCall.Function.Arguments), &shell)
		fmt.Printf("\u001b[38;5;80m$\u001b[0m %s ", shell.Command)
}

func getUserApproval(a *Agent) (bool, string) {
	for {
		userInput, ok := a.getUserMessage()
		if !ok { 
			return false, "" 
		}
		switch userInput {
		case "":
			return true, ""
		case "n":
			fmt.Printf("\u001b[38;5;80mSystem\u001b[0m: Add a note for agent about refusal?\n")
			fmt.Print("\u001b[38;5;216mYou\u001b[0m: ")
			refusalNote, ok := a.getUserMessage()
			if !ok {
				return false, ""
			}
			return false, refusalNote
		default:
			continue
		}
	}
}
