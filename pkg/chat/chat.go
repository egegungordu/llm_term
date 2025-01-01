package chat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"llm_term/pkg/types"

	"github.com/joho/godotenv"
	"github.com/rivo/tview"
)

// Maximum number of messages to keep in history
const maxHistorySize = 100

type Chat struct {
	history []types.Message
}

func New() *Chat {
	return &Chat{
		history: make([]types.Message, 0),
	}
}

func (c *Chat) addToHistory(message types.Message) {
	c.history = append(c.history, message)
	
	// Trim history if it exceeds max size
	if len(c.history) > maxHistorySize {
		c.history = c.history[len(c.history)-maxHistorySize:]
	}
}

func init() {
	// Load .env file if it exists
	godotenv.Load()
}

func getConfig() (endpoint string, model string, err error) {
	endpoint = os.Getenv("LLM_ENDPOINT")
	if endpoint == "" {
		return "", "", fmt.Errorf("LLM_ENDPOINT environment variable is not set")
	}
	
	model = os.Getenv("LLM_MODEL")
	if model == "" {
		return "", "", fmt.Errorf("LLM_MODEL environment variable is not set")
	}
	
	return endpoint, model, nil
}

func (c *Chat) StreamChat(text string, chatView *tview.TextView, app *tview.Application, onComplete func()) {
	endpoint, model, err := getConfig()
	if err != nil {
		fmt.Fprintf(chatView, "[red]Configuration Error: %v\n[yellow]Please set the required environment variables in your .env file.[white]\n", err)
		if onComplete != nil {
			onComplete()
		}
		return
	}

	userMessage := types.Message{
		Role:    "user",
		Content: text,
	}
	c.addToHistory(userMessage)
	
	request := types.ChatRequest{
		Model:       model,
		Temperature: 1,
		Messages:    c.history,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		fmt.Fprintf(chatView, "[red]Error: %v\n", err)
		return
	}

	resp, err := http.Post(endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Fprintf(chatView, "[red]Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	fmt.Fprintf(chatView, "[green]AI:[white] ")
	
	var assistantMessage types.Message
	assistantMessage.Role = "assistant"
	
	for {
		var response types.ChatResponse
		if err := decoder.Decode(&response); err != nil {
			if err == io.EOF {
				break
			}
			fmt.Fprintf(chatView, "[red]Error: %v\n", err)
			return
		}

		app.QueueUpdateDraw(func() {
			fmt.Fprintf(chatView, "%s", response.Message.Content)
			chatView.ScrollToEnd()
		})

		assistantMessage.Content += response.Message.Content

		if response.Done {
			break
		}
	}
	
	// Add the complete assistant message to history
	c.addToHistory(assistantMessage)
	
	app.QueueUpdateDraw(func() {
		fmt.Fprintf(chatView, "\n")
		chatView.ScrollToEnd()
	})
	
	if onComplete != nil {
		onComplete()
	}
} 