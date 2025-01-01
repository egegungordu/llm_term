package ui

import (
	"fmt"
	"strings"
	"time"

	"llm_term/pkg/chat"
	"llm_term/pkg/system"
	"llm_term/pkg/types"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type UI struct {
	app         *tview.Application
	chatView    *tview.TextView
	inputField  *tview.InputField
	keybindView *tview.TextView
	metricsView *tview.TextView
	currentMode types.Mode
	modeKeybinds map[types.Mode][]types.KeyBinding
	isAIResponding bool
	spinnerFrames []string
	currentSpinnerFrame int
	stopSpinner chan bool
	chat        *chat.Chat
	autoScroll  bool
	metrics     *system.Metrics
	// Performance metrics
	totalTokens int
	totalDuration float64
	currentModel string
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
		autoScroll:  true,
		metrics:     system.New(),
	}

	ui.modeKeybinds = map[types.Mode][]types.KeyBinding{
		types.NormalMode: {
			{Key: "q", Description: "quit"},
			{Key: "i", Description: "enter input mode"},
			{Key: "j", Description: "scroll down"},
			{Key: "k", Description: "scroll up"},
			{Key: "gg", Description: "scroll to top"},
			{Key: "G", Description: "scroll to bottom"},
			{Key: "Ctrl+D", Description: "scroll down half page"},
			{Key: "Ctrl+U", Description: "scroll up half page"},
		},
		types.InputMode: {
			{Key: "Esc", Description: "enter normal mode"},
			{Key: "Enter", Description: "send message"},
		},
		types.ResponseMode: {
			{Key: "q", Description: "quit"},
			{Key: "j", Description: "scroll down"},
			{Key: "k", Description: "scroll up"},
			{Key: "gg", Description: "scroll to top"},
			{Key: "G", Description: "scroll to bottom"},
			{Key: "Ctrl+D", Description: "scroll down half page"},
			{Key: "Ctrl+U", Description: "scroll up half page"},
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
		SetFieldWidth(0).
		SetFieldBackgroundColor(tcell.ColorDefault).
		SetFieldTextColor(tcell.ColorDefault)
	ui.inputField.SetBorder(false)

	// Create metrics view
	ui.metricsView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	ui.metricsView.SetBorder(false)

	// Create mode and keybinds display
	ui.keybindView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetWordWrap(true)
	ui.keybindView.SetBorder(false)
}

func (ui *UI) setupHandlers() {
	lastKeyTime := time.Now()
	lastKey := ' '

	handleScrollCommand := func(event *tcell.EventKey) *tcell.EventKey {
		// Handle Ctrl+D and Ctrl+U
		if event.Key() == tcell.KeyCtrlD {
			_, _, _, height := ui.chatView.GetInnerRect()
			row, _ := ui.chatView.GetScrollOffset()
			ui.chatView.ScrollTo(row+height/2, 0)
			ui.autoScroll = false
			return nil
		} else if event.Key() == tcell.KeyCtrlU {
			_, _, _, height := ui.chatView.GetInnerRect()
			row, _ := ui.chatView.GetScrollOffset()
			newRow := row - height/2
			if newRow < 0 {
				newRow = 0
			}
			ui.chatView.ScrollTo(newRow, 0)
			ui.autoScroll = false
			return nil
		}

		switch event.Rune() {
		case 'g':
			// Check if this is a double 'g' within 500ms
			if lastKey == 'g' && time.Since(lastKeyTime) < 500*time.Millisecond {
				ui.chatView.ScrollToBeginning()
				ui.autoScroll = false
				lastKey = ' ' // Reset last key
				return nil
			}
			lastKey = 'g'
			lastKeyTime = time.Now()
			return nil
		case 'G':
			ui.chatView.ScrollToEnd()
			ui.autoScroll = true
			return nil
		case 'j': // Scroll down
			row, _ := ui.chatView.GetScrollOffset()
			ui.chatView.ScrollTo(row+1, 0)
			ui.autoScroll = false
			return nil
		case 'k': // Scroll up
			row, _ := ui.chatView.GetScrollOffset()
			if row > 0 {
				ui.chatView.ScrollTo(row-1, 0)
				ui.autoScroll = false
			}
			return nil
		case 'q':
			ui.app.Stop()
			return nil
		}
		lastKey = event.Rune()
		lastKeyTime = time.Now()
		return event
	}

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
			
			ui.autoScroll = true // Reset auto-scroll when sending message
			fmt.Fprintf(ui.chatView, "[yellow]You:[white] %s\n", text)
			ui.inputField.SetText("")
			ui.chatView.ScrollToEnd()
			
			// Set responding flag and update UI
			ui.isAIResponding = true
			ui.setMode(types.ResponseMode)
			ui.startSpinner()
			
			// Call the streaming chat function with response handler
			go ui.chat.StreamChat(text, ui.chatView, ui.app, 
				func(response types.ChatResponse) {
					ui.updatePerformanceMetrics(response)
				},
				func() {
					ui.handleResponseComplete()
				},
			)
		}
	})

	// Add mouse handler for scroll wheel
	ui.chatView.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action == tview.MouseScrollUp || action == tview.MouseScrollDown {
			ui.autoScroll = false
		}
		return action, event
	})

	// Add change handler for chat view to handle manual scrolling
	ui.chatView.SetChangedFunc(func() {
		if ui.autoScroll {
			ui.chatView.ScrollToEnd()
		}
	})

	// Global key handler
	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Handle Ctrl+C globally
		if event.Key() == tcell.KeyCtrlC {
			if ui.isAIResponding {
				// Cancel AI response
				ui.chat.Cancel()
				return nil
			}
			// In other modes, ignore Ctrl+C
			return nil
		}

		switch ui.currentMode {
		case types.ResponseMode:
			return handleScrollCommand(event)
		case types.NormalMode:
			if event.Rune() == 'i' {
				ui.setMode(types.InputMode)
				ui.autoScroll = true // Reset auto-scroll when entering input mode
				return nil
			}
			return handleScrollCommand(event)
		case types.InputMode:
			if event.Key() == tcell.KeyEscape {
				ui.setMode(types.NormalMode)
				return nil
			}
		}
		return event
	})

	// Add keybind for Ctrl+C to response mode
	ui.modeKeybinds[types.ResponseMode] = append(ui.modeKeybinds[types.ResponseMode],
		types.KeyBinding{Key: "Ctrl+C", Description: "cancel response"})
}

func (ui *UI) updateModeState() {
	if ui.currentMode == types.NormalMode || ui.currentMode == types.ResponseMode {
		ui.inputField.SetBackgroundColor(tcell.ColorDefault)
		ui.inputField.SetFieldBackgroundColor(tcell.ColorDefault)
		ui.app.SetFocus(nil) // Remove focus from input field
		ui.inputField.SetDisabled(true) // Disable input when not in input mode
	} else {
		ui.inputField.SetBackgroundColor(tcell.ColorDefault)
		ui.inputField.SetFieldBackgroundColor(tcell.ColorDefault)
		ui.inputField.SetDisabled(false) // Enable input in input mode
		ui.app.SetFocus(ui.inputField)
	}
}

func (ui *UI) setMode(mode types.Mode) {
	ui.currentMode = mode
	ui.updateModeState()
	ui.updateKeybindDisplay()
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
}

func (ui *UI) getModeText() string {
	switch ui.currentMode {
	case types.ResponseMode:
		return fmt.Sprintf("[yellow]AI responding %s[white]", ui.spinnerFrames[ui.currentSpinnerFrame])
	case types.NormalMode:
		return "[yellow]NORMAL MODE[white]"
	default:
		return "[yellow]INPUT MODE[white]"
	}
}

func (ui *UI) Run() error {
	// Start system metrics collection
	ui.metrics.Start()
	defer ui.metrics.Stop()

	// Create main flex container for layout
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow)

	// Create horizontal flex for chat and metrics
	contentFlex := tview.NewFlex().
		SetDirection(tview.FlexColumn)

	// Add chat view with weight 4
	contentFlex.AddItem(ui.chatView, 0, 4, false)

	// Add metrics view with fixed width
	contentFlex.AddItem(ui.metricsView, 26, 0, false)

	// Create a flex for input area that matches chat width
	inputAreaFlex := tview.NewFlex().
		SetDirection(tview.FlexColumn)

	// Create input row with mode indicator
	inputFlex := tview.NewFlex().
		AddItem(ui.inputField, 0, 2, true).
		AddItem(tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignRight), 20, 0, false)

	// Add input flex to match chat width
	inputAreaFlex.AddItem(inputFlex, 0, 4, true)
	// Add empty space to match metrics width
	inputAreaFlex.AddItem(nil, 26, 0, false)

	// Add the content flex to main container with more weight
	flex.AddItem(contentFlex, 0, 8, false)
	flex.AddItem(inputAreaFlex, 1, 0, true)

	// Add keybind display with fixed height
	flex.AddItem(ui.keybindView, 6, 0, false)  // Fixed height of 6 lines

	// Update mode text and metrics periodically
	go func() {
		for {
			ui.app.QueueUpdateDraw(func() {
				modeView := inputFlex.GetItem(1).(*tview.TextView)
				modeView.Clear()
				fmt.Fprintf(modeView, "%s", ui.getModeText())

				// Update metrics view with padding
				ui.metricsView.Clear()
				fmt.Fprintf(ui.metricsView, "%s", ui.metrics.GetFormattedMetrics(0))
			})
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Center the entire flex container with less padding
	centered := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().
				SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(flex, 0, 20, true).  // Increased weight from 3 to 20
				AddItem(nil, 0, 1, false),
			0, 20, true,  // Increased weight from 3 to 20
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

func (ui *UI) updateChat(text string) {
	ui.app.QueueUpdateDraw(func() {
		fmt.Fprintf(ui.chatView, "%s", text)
		if ui.autoScroll {
			ui.chatView.ScrollToEnd()
		}
	})
}

func (ui *UI) updatePerformanceMetrics(response types.ChatResponse) {
	if response.Done && response.EvalCount > 0 {
		// Store current model
		ui.currentModel = response.Model
		
		// Total tokens = prompt tokens + completion tokens
		totalTokens := response.PromptEvalCount + response.EvalCount
		
		// Total duration in seconds (convert from nanoseconds)
		totalDurationSecs := float64(response.TotalDuration) / 1e9
		
		// Calculate tokens per second using the formula: totalTokens / totalDurationSecs
		tokPerSec := float64(totalTokens) / totalDurationSecs

		// Update metrics with model info
		ui.metrics.SetModelMetrics(ui.currentModel, tokPerSec)
		
		ui.app.QueueUpdateDraw(func() {
			ui.chatView.SetTitle("Chat")
		})
	}
}

func (ui *UI) handleResponseComplete() {
	ui.app.QueueUpdateDraw(func() {
		ui.stopSpinner <- true
		ui.isAIResponding = false
		ui.setMode(types.InputMode)
		ui.autoScroll = true
		ui.chatView.ScrollToEnd()
	})
} 