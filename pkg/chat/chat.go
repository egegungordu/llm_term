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

func init() {
	// Load .env file if it exists
	godotenv.Load()
}

func getConfig() (endpoint string, model string) {
	endpoint = os.Getenv("LLM_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://167.235.207.146:11434/api/chat" // fallback default
	}
	
	model = os.Getenv("LLM_MODEL")
	if model == "" {
		model = "llama3.2" // fallback default
	}
	
	return endpoint, model
}

func StreamChat(text string, chatView *tview.TextView, app *tview.Application, onComplete func()) {
	endpoint, model := getConfig()
	
	request := types.ChatRequest{
		Model:       model,
		Temperature: 1,
		Messages: []types.Message{
			{
				Role:    "user",
				Content: text,
			},
		},
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

		if response.Done {
			break
		}
	}
	
	app.QueueUpdateDraw(func() {
		fmt.Fprintf(chatView, "\n")
		chatView.ScrollToEnd()
	})
	
	if onComplete != nil {
		onComplete()
	}
} 