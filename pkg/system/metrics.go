package system

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

type Metrics struct {
	CPUUsage    float64
	MemoryUsage float64
	MemUsedGB   float64
	MemTotalGB  float64
	Model       string
	TokenSpeed  float64
	stopChan    chan bool
}

func New() *Metrics {
	// Initialize with model from environment
	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "No model set"
	}

	return &Metrics{
		stopChan: make(chan bool),
		Model:    model,
	}
}

func (m *Metrics) SetModelMetrics(model string, tokenSpeed float64) {
	m.Model = model
	m.TokenSpeed = tokenSpeed
}

func (m *Metrics) Start() {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
				select {
				case <-m.stopChan:
					return
				case <-ticker.C:
					m.update()
				}
		}
	}()
}

func (m *Metrics) Stop() {
	m.stopChan <- true
}

func (m *Metrics) update() {
	// Get CPU usage
	cpuPercent, err := cpu.Percent(0, false)
	if err == nil && len(cpuPercent) > 0 {
		m.CPUUsage = cpuPercent[0]
	}

	// Get memory usage
	memStats, err := mem.VirtualMemory()
	if err == nil {
		m.MemoryUsage = memStats.UsedPercent
		m.MemUsedGB = float64(memStats.Used) / (1024 * 1024 * 1024)  // Convert to GB
		m.MemTotalGB = float64(memStats.Total) / (1024 * 1024 * 1024) // Convert to GB
	}
}

func (m *Metrics) GetMetricsText() string {
	return fmt.Sprintf("CPU: %.1f%% | MEM: %.1f%%", m.CPUUsage, m.MemoryUsage)
}

func (m *Metrics) GetFormattedMetrics(height int) string {
	const barWidth = 12
	const barChar = "█"
	const emptyChar = "░"

	cpuBars := int(m.CPUUsage * float64(barWidth) / 100)
	memBars := int(m.MemoryUsage * float64(barWidth) / 100)

	if cpuBars > barWidth {
		cpuBars = barWidth
	}
	if memBars > barWidth {
		memBars = barWidth
	}

	var result strings.Builder

	// Model and token speed if available
	if m.Model != "" {
		result.WriteString("  ") // Add same padding as metrics
		result.WriteString(fmt.Sprintf("[blue]%s[white] (%.1f tok/s)\n", m.Model, m.TokenSpeed))
		result.WriteString("\n  ") // Add padding for next line
	}
	
	// CPU bar (with padding)
	result.WriteString("CPU")
	result.WriteString(fmt.Sprintf(" [red]%s[white]%s",
		strings.Repeat(barChar, cpuBars),
		strings.Repeat(emptyChar, barWidth-cpuBars)))
	result.WriteString(fmt.Sprintf(" %.0f%%\n", m.CPUUsage))

	// Memory bar (with padding)
	result.WriteString("  MEM")
	result.WriteString(fmt.Sprintf(" [yellow]%s[white]%s",
		strings.Repeat(barChar, memBars),
		strings.Repeat(emptyChar, barWidth-memBars)))
	result.WriteString(fmt.Sprintf(" %.0f%%", m.MemoryUsage))

	return result.String()
} 