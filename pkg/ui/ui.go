package ui

import (
	"fmt"
	"time"

	"llm_term/pkg/chat"
	"llm_term/pkg/types"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type UI struct {
	app         *tview.Application
	chatView    *tview.TextView
	inputField  *tview.InputField
	keybindView *tview.TextView
	currentMode types.Mode
	modeKeybinds map[types.Mode][]types.KeyBinding
	isAIResponding bool
	spinnerFrames []string
	currentSpinnerFrame int
	stopSpinner chan bool
}

func New() *UI {
	ui := &UI{
		app:         tview.NewApplication(),
		currentMode: types.InputMode,
		isAIResponding: false,
		spinnerFrames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		currentSpinnerFrame: 0,
		stopSpinner: make(chan bool),
	}

	ui.modeKeybinds = map[types.Mode][]types.KeyBinding{
		types.NormalMode: {
			{"q", "quit"},
			{"i", "enter input mode"},
		},
		types.InputMode: {
			{"Esc", "enter normal mode"},
			{"Enter", "send message"},
		},
	}

	ui.setupViews()
	ui.setupHandlers()
	return ui
}

func (ui *UI) setupViews() {
	// Create chat text view
	ui.chatView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true)
	ui.chatView.SetBorder(true).
		SetTitle("Chat").
		SetTitleAlign(tview.AlignLeft)

	// Create input field
	ui.inputField = tview.NewInputField().
		SetLabel("> ").
		SetFieldWidth(0)
	ui.inputField.SetBorder(false)

	// Create mode and keybinds display
	ui.keybindView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	ui.keybindView.SetBorder(false)
}

func (ui *UI) setupHandlers() {
	// Handle input
	ui.inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter && ui.currentMode == types.InputMode && !ui.isAIResponding {
			text := ui.inputField.GetText()
			if text == "" {
				return
			}
			
			fmt.Fprintf(ui.chatView, "[yellow]You:[white] %s\n", text)
			ui.inputField.SetText("")
			ui.chatView.ScrollToEnd()
			
			// Set responding flag and update UI
			ui.isAIResponding = true
			ui.updateModeState()
			ui.startSpinner()
			
			// Call the streaming chat function
			go chat.StreamChat(text, ui.chatView, ui.app, func() {
				ui.app.QueueUpdateDraw(func() {
					ui.stopSpinner <- true
					ui.isAIResponding = false
					ui.updateModeState()
				})
			})
		}
	})

	// Global key handler
	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if ui.isAIResponding {
			return nil // Block all input while AI is responding
		}
		
		switch ui.currentMode {
		case types.NormalMode:
			switch event.Rune() {
			case 'q':
				ui.app.Stop()
				return nil
			case 'i':
				ui.currentMode = types.InputMode
				ui.updateKeybindDisplay()
				return nil
			}
		case types.InputMode:
			if event.Key() == tcell.KeyEscape {
				ui.currentMode = types.NormalMode
				ui.updateKeybindDisplay()
				return nil
			}
		}
		return event
	})
}

func (ui *UI) updateModeState() {
	if ui.currentMode == types.NormalMode || ui.isAIResponding {
		ui.inputField.SetBackgroundColor(tcell.ColorDefault)
		ui.inputField.SetFieldBackgroundColor(tcell.ColorDefault)
		ui.app.SetFocus(nil) // Remove focus from input field
		if ui.isAIResponding {
			ui.inputField.SetLabel(fmt.Sprintf("[yellow]AI responding %s[white] > ", ui.spinnerFrames[ui.currentSpinnerFrame]))
		} else {
			ui.inputField.SetLabel("> ")
		}
	} else {
		ui.inputField.SetBackgroundColor(tcell.ColorDefault)
		ui.inputField.SetFieldBackgroundColor(tcell.ColorDefault)
		ui.inputField.SetLabel("> ")
		ui.app.SetFocus(ui.inputField)
	}
}

func (ui *UI) updateKeybindDisplay() {
	ui.keybindView.Clear()
	modeText := "[yellow]Mode: "
	if ui.currentMode == types.NormalMode {
		modeText += "NORMAL[white]"
	} else {
		modeText += "INPUT[white]"
	}

	fmt.Fprintf(ui.keybindView, "%s    ", modeText)
	
	// Display keybinds for current mode
	binds := ui.modeKeybinds[ui.currentMode]
	for i, bind := range binds {
		fmt.Fprintf(ui.keybindView, "[green]%s[white]:%s", bind.Key, bind.Description)
		if i < len(binds)-1 {
			fmt.Fprintf(ui.keybindView, " | ")
		}
	}
	ui.updateModeState()
}

func (ui *UI) startSpinner() {
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		
		for {
			select {
			case <-ui.stopSpinner:
				return
			case <-ticker.C:
				ui.app.QueueUpdateDraw(func() {
					ui.currentSpinnerFrame = (ui.currentSpinnerFrame + 1) % len(ui.spinnerFrames)
					if ui.isAIResponding {
						ui.inputField.SetLabel(fmt.Sprintf("[yellow]AI responding %s[white] > ", ui.spinnerFrames[ui.currentSpinnerFrame]))
					}
				})
			}
		}
	}()
}

func (ui *UI) Run() error {
	// Create flex container for layout
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow)

	// Add chat view with weight 1
	flex.AddItem(ui.chatView, 0, 1, false)

	// Add input field
	flex.AddItem(ui.inputField, 1, 0, true)

	// Add keybind display
	flex.AddItem(ui.keybindView, 1, 0, false)

	// Center the entire flex container
	centered := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().
				SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(flex, 0, 3, true).
				AddItem(nil, 0, 1, false),
			0, 3, true,
		).
		AddItem(nil, 0, 1, false)

	// Initial setup
	ui.updateKeybindDisplay()

	return ui.app.SetRoot(centered, true).EnableMouse(true).Run()
} 