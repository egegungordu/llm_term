package chat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"llm_term/pkg/types"

	"github.com/rivo/tview"
)

func StreamChat(text string, chatView *tview.TextView, app *tview.Application, onComplete func()) {
	request := types.ChatRequest{
		Model:       "llama3.2",
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

	resp, err := http.Post("http://167.235.207.146:11434/api/chat", "application/json", bytes.NewBuffer(jsonData))
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