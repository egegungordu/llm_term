package chat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"llm_term/pkg/types"

	"github.com/joho/godotenv"
	"github.com/rivo/tview"
)

// Maximum number of messages to keep in history
const maxHistorySize = 100

type Chat struct {
	history []types.Message
	cancelChan chan struct{}
	mu sync.Mutex
	isStreaming bool
}

func New() *Chat {
	return &Chat{
		history: make([]types.Message, 0),
		cancelChan: make(chan struct{}),
	}
}

func (c *Chat) Cancel() {
	c.mu.Lock()
	if c.isStreaming {
		close(c.cancelChan)
		c.isStreaming = false
	}
	c.mu.Unlock()
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

func (c *Chat) StreamChat(text string, chatView *tview.TextView, app *tview.Application, onResponse func(types.ChatResponse), onComplete func()) {
	// Create new cancel channel for this stream
	c.mu.Lock()
	c.cancelChan = make(chan struct{})
	c.isStreaming = true
	c.mu.Unlock()

	// Ensure we mark streaming as done when we exit
	defer func() {
		c.mu.Lock()
		c.isStreaming = false
		c.mu.Unlock()
		if onComplete != nil {
			onComplete()
		}
	}()

	endpoint, model, err := getConfig()
	if err != nil {
		fmt.Fprintf(chatView, "[red]Configuration Error: %v\n[yellow]Please set the required environment variables in your .env file.[white]\n", err)
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
	app.QueueUpdateDraw(func() {
		fmt.Fprintf(chatView, "[green]AI:[white] ")
	})
	
	var assistantMessage types.Message
	assistantMessage.Role = "assistant"
	
	streamLoop:
	for {
		select {
		case <-c.cancelChan:
			app.QueueUpdateDraw(func() {
				fmt.Fprintf(chatView, "\n[yellow]Response cancelled by user[white]\n")
			})
			return
		default:
			var response types.ChatResponse
			if err := decoder.Decode(&response); err != nil {
				if err == io.EOF {
					break streamLoop
				}
				fmt.Fprintf(chatView, "[red]Error: %v\n", err)
				return
			}

			app.QueueUpdateDraw(func() {
				fmt.Fprintf(chatView, "%s", response.Message.Content)
			})

			assistantMessage.Content += response.Message.Content
			
			if onResponse != nil {
				onResponse(response)
			}

			if response.Done {
				break streamLoop
			}
		}
	}
	
	// Add the complete assistant message to history
	c.addToHistory(assistantMessage)
	
	app.QueueUpdateDraw(func() {
		fmt.Fprintf(chatView, "\n")
	})
} 