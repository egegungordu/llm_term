package ui

import (
	"fmt"
	"strings"
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
	chat        *chat.Chat
}

func New() *UI {
	ui := &UI{
		app:         tview.NewApplication(),
		currentMode: types.InputMode,
		isAIResponding: false,
		spinnerFrames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		currentSpinnerFrame: 0,
		stopSpinner: make(chan bool),
		chat:        chat.New(),
	}

	ui.modeKeybinds = map[types.Mode][]types.KeyBinding{
		types.NormalMode: {
			{Key: "q", Description: "quit"},
			{Key: "i", Description: "enter input mode"},
			{Key: "j", Description: "scroll down"},
			{Key: "k", Description: "scroll up"},
			{Key: "Ctrl+D", Description: "scroll down half page"},
			{Key: "Ctrl+U", Description: "scroll up half page"},
		},
		types.InputMode: {
			{Key: "Esc", Description: "enter normal mode"},
			{Key: "Enter", Description: "send message"},
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
		SetWordWrap(true).
		ScrollToEnd()
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
		SetTextAlign(tview.AlignLeft).
		SetWordWrap(true)
	ui.keybindView.SetBorder(false)
}

func (ui *UI) setupHandlers() {
	// Handle input
	ui.inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter && ui.currentMode == types.InputMode {
			// Block new messages while AI is responding
			if ui.isAIResponding {
				return
			}
			
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
			go ui.chat.StreamChat(text, ui.chatView, ui.app, func() {
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
		switch ui.currentMode {
		case types.NormalMode:
			// Handle Ctrl+D and Ctrl+U
			if event.Key() == tcell.KeyCtrlD {
				_, _, _, height := ui.chatView.GetInnerRect()
				row, _ := ui.chatView.GetScrollOffset()
				ui.chatView.ScrollTo(row+height/2, 0)
				return nil
			} else if event.Key() == tcell.KeyCtrlU {
				_, _, _, height := ui.chatView.GetInnerRect()
				row, _ := ui.chatView.GetScrollOffset()
				newRow := row - height/2
				if newRow < 0 {
					newRow = 0
				}
				ui.chatView.ScrollTo(newRow, 0)
				return nil
			}

			switch event.Rune() {
			case 'q':
				ui.app.Stop()
				return nil
			case 'i':
				ui.currentMode = types.InputMode
				ui.updateKeybindDisplay()
				return nil
			case 'j': // Scroll down
				row, _ := ui.chatView.GetScrollOffset()
				ui.chatView.ScrollTo(row+1, 0)
				return nil
			case 'k': // Scroll up
				row, _ := ui.chatView.GetScrollOffset()
				if row > 0 {
					ui.chatView.ScrollTo(row-1, 0)
				}
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
	} else {
		ui.inputField.SetBackgroundColor(tcell.ColorDefault)
		ui.inputField.SetFieldBackgroundColor(tcell.ColorDefault)
		ui.app.SetFocus(ui.inputField)
	}
}

func (ui *UI) updateKeybindDisplay() {
	ui.keybindView.Clear()
	
	// Display keybinds for current mode in a grid
	binds := ui.modeKeybinds[ui.currentMode]
	bindsPerRow := 2  // Reduce to 2 bindings per row for better visibility
	
	// First pass: calculate max widths for alignment
	maxKeyWidth := 0
	maxDescWidth := 0
	for _, bind := range binds {
		if len(bind.Key) > maxKeyWidth {
			maxKeyWidth = len(bind.Key)
		}
		if len(bind.Description) > maxDescWidth {
			maxDescWidth = len(bind.Description)
		}
	}
	
	// Second pass: display bindings in a grid
	for i := 0; i < len(binds); i += bindsPerRow {
		for j := 0; j < bindsPerRow && i+j < len(binds); j++ {
			bind := binds[i+j]
			// Pad the key and description for alignment
			keyPadding := strings.Repeat(" ", maxKeyWidth-len(bind.Key))
			descPadding := strings.Repeat(" ", maxDescWidth-len(bind.Description))
			
			fmt.Fprintf(ui.keybindView, "[green]%s%s[white]:%s%s", 
				bind.Key, 
				keyPadding,
				bind.Description,
				descPadding,
			)
			
			// Add spacing between columns, except for the last column
			if j < bindsPerRow-1 && i+j < len(binds)-1 {
				fmt.Fprintf(ui.keybindView, "        ")  // More spacing between columns
			}
		}
		fmt.Fprintf(ui.keybindView, "\n")  // Always add newline after each row
	}
	
	ui.updateModeState()
}

func (ui *UI) getModeText() string {
	if ui.isAIResponding {
		return fmt.Sprintf("[yellow]AI responding %s[white]", ui.spinnerFrames[ui.currentSpinnerFrame])
	} else if ui.currentMode == types.NormalMode {
		return "[yellow]NORMAL MODE[white]"
	}
	return "[yellow]INPUT MODE[white]"
}

func (ui *UI) Run() error {
	// Create flex container for layout
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow)

	// Add chat view with weight 1
	flex.AddItem(ui.chatView, 0, 1, false)

	// Create input row with mode indicator
	inputFlex := tview.NewFlex().
		AddItem(ui.inputField, 0, 2, true).
		AddItem(tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignRight), 20, 0, false)
	
	// Update mode text periodically
	go func() {
		for {
			ui.app.QueueUpdateDraw(func() {
				modeView := inputFlex.GetItem(1).(*tview.TextView)
				modeView.Clear()
				fmt.Fprintf(modeView, "%s", ui.getModeText())
			})
			time.Sleep(100 * time.Millisecond)
		}
	}()

	flex.AddItem(inputFlex, 1, 0, true)

	// Add keybind display with fixed height
	flex.AddItem(ui.keybindView, 6, 0, false)  // Fixed height of 6 lines

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
				})
			}
		}
	}()
} 