package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderMultiLineGraph(data []float64, width, height int, maxVal float64, theme Theme) string {
	if width < 1 || height < 1 {
		return ""
	}

	gridStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Muted))

	rows := make([][]string, height)
	for i := range rows {
		rows[i] = make([]string, width)
		for j := range rows[i] {
			if i == height-1 {
				rows[i][j] = gridStyle.Render("─")
			} else if i%2 == 0 {
				rows[i][j] = gridStyle.Render("╌")
			} else {
				rows[i][j] = " "
			}
		}
	}

	blocks := []string{" ", " ", "▂", "▃", "▄", "▅", "▆", "█"}

	gradient := []string{
		theme.Success,
		theme.Success,
		theme.Accent,
		theme.Warning,
		theme.Error,
	}

	rowChars := make([][]string, height)
	for y := 0; y < height; y++ {
		colorIdx := (y * len(gradient)) / height
		if colorIdx >= len(gradient) {
			colorIdx = len(gradient) - 1
		}
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(gradient[colorIdx]))

		rowChars[y] = make([]string, len(blocks))
		for k, b := range blocks {
			rowChars[y][k] = style.Render(b)
		}
	}

	if len(data) > 0 {
		colsPerPoint := float64(width) / float64(len(data))

		for i, val := range data {
			if val < 0 {
				val = 0
			}

			pct := val / maxVal
			if pct > 1.0 {
				pct = 1.0
			}
			totalSubBlocks := pct * float64(height) * 8.0

			startCol := int(float64(i) * colsPerPoint)
			endCol := int(float64(i+1) * colsPerPoint)
			if endCol > width {
				endCol = width
			}

			for col := startCol; col < endCol; col++ {
				for y := 0; y < height; y++ {
					rowIndex := height - 1 - y
					rowValue := totalSubBlocks - float64(y*8)

					var charIndex int
					if rowValue <= 0 {
						charIndex = 0
					} else if rowValue >= 8 {
						charIndex = 7
					} else {
						charIndex = int(rowValue)
					}

					if charIndex > 0 {
						rows[rowIndex][col] = rowChars[y][charIndex]
					}
				}
			}
		}
	}

	var graphBuilder strings.Builder
	for i, row := range rows {
		graphBuilder.WriteString(strings.Join(row, ""))
		if i < height-1 {
			graphBuilder.WriteRune('\n')
		}
	}

	return graphBuilder.String()
}

func renderGraphBox(width, height int, history []float64, theme Theme, st styles, totalDownloaded int64) string {
	if width < 1 || height < 1 {
		return ""
	}

	contentHeight := height - BorderFrameHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	graphContentHeight := contentHeight - 2
	if graphContentHeight < 3 {
		graphContentHeight = 3
	}

	var graphData []float64
	if len(history) > 60 {
		graphData = history[len(history)-60:]
	} else {
		graphData = history
	}

	currentSpeedBps := 0.0
	if len(history) > 0 {
		currentSpeedBps = history[len(history)-1]
	}

	topSpeedBps := 0.0
	for _, s := range history {
		if s > topSpeedBps {
			topSpeedBps = s
		}
	}
	if currentSpeedBps > topSpeedBps {
		topSpeedBps = currentSpeedBps
	}

	maxSpeed := 0.0
	for _, v := range graphData {
		if v > maxSpeed {
			maxSpeed = v
		}
	}

	if maxSpeed == 0 {
		maxSpeed = 1000000.0
	} else {
		maxSpeed = maxSpeed * 1.2
		if maxSpeed < 1000000.0 {
			maxSpeed = 1000000.0
		}
		mb := maxSpeed / 1000000.0
		if mb >= 5 {
			mb = float64(int((mb+4.99)/5) * 5)
		} else {
			mb = float64(int(mb + 0.99))
		}
		maxSpeed = mb * 1000000.0
	}

	axisWidth := 10
	hideGraphStats := width < 50
	
	var graphAreaWidth int
	if hideGraphStats {
		graphAreaWidth = width - BorderFrameWidth - axisWidth - 2
	} else {
		graphAreaWidth = width - BorderFrameWidth - 22 - axisWidth - 2
	}
	if graphAreaWidth < 10 {
		graphAreaWidth = 10
	}

	buildAxisLines := func(h int, axisStyle lipgloss.Style) []string {
		label := func(v float64) string {
			if v <= 0 {
				return "0 MiB/s"
			}
			return humanSpeed(v)
		}

		axisLines := make([]string, h)
		for i := range axisLines {
			axisLines[i] = axisStyle.Render("")
		}

		type axisMark struct {
			num int
			den int
		}

		marks := []axisMark{
			{num: 1, den: 1},
			{num: 1, den: 2},
			{num: 0, den: 1},
		}
		if h >= 9 {
			marks = []axisMark{
				{num: 1, den: 1},
				{num: 4, den: 5},
				{num: 3, den: 5},
				{num: 2, den: 5},
				{num: 1, den: 5},
				{num: 0, den: 1},
			}
		}

		for _, mark := range marks {
			row := 0
			if h > 1 {
				row = ((mark.den-mark.num)*(h-1) + mark.den/2) / mark.den
			}
			value := maxSpeed * float64(mark.num) / float64(mark.den)
			axisLines[row] = axisStyle.Render(label(value))
		}

		return axisLines
	}

	var graphWithAxis string
	axisStyle := lipgloss.NewStyle().Width(axisWidth).Foreground(lipgloss.Color(theme.Accent)).Align(lipgloss.Right)
	axisLines := buildAxisLines(graphContentHeight, axisStyle)
	axisColumn := lipgloss.NewStyle().
		Height(graphContentHeight).
		Align(lipgloss.Right).
		Render(strings.Join(axisLines, "\n"))

	graphVisual := renderMultiLineGraph(graphData, graphAreaWidth, graphContentHeight, maxSpeed, theme)

	if hideGraphStats {
		graphWithAxis = lipgloss.JoinHorizontal(lipgloss.Top, graphVisual, axisColumn)
	} else {
		speedMbps := currentSpeedBps * 8 / 1000000.0
		topMbps := topSpeedBps * 8 / 1000000.0

		valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Accent)).Bold(true)
		labelStyleStats := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Foreground))
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Muted))

		speedStr := "0 MiB/s"
		if currentSpeedBps > 0 {
			speedStr = humanSpeed(currentSpeedBps)
		}
		topStr := "0 MiB/s"
		if topSpeedBps > 0 {
			topStr = humanSpeed(topSpeedBps)
		}

		statsContent := lipgloss.JoinVertical(lipgloss.Left,
			fmt.Sprintf("%s %s", valueStyle.Render("▼"), valueStyle.Render(speedStr)),
			dimStyle.Render(fmt.Sprintf("  (%.0f Mbps)", speedMbps)),
			"",
			fmt.Sprintf("%s %s", labelStyleStats.Render("Top:"), valueStyle.Render(topStr)),
			dimStyle.Render(fmt.Sprintf("  (%.0f Mbps)", topMbps)),
			"",
			fmt.Sprintf("%s %s", labelStyleStats.Render("Total:"), valueStyle.Render(humanBytes(totalDownloaded))),
		)

		statsBoxStyle := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(lipgloss.Color(theme.Muted)).
			Padding(0, 1).
			Width(22).
			Height(graphContentHeight)
		statsBox := statsBoxStyle.Render(statsContent)

		graphWithAxis = lipgloss.JoinHorizontal(lipgloss.Top, statsBox, graphVisual, axisColumn)
	}

	innerContent := lipgloss.JoinVertical(lipgloss.Left, "", graphWithAxis, "")
	return RenderBtopBox(st.Label.Render(" Network Activity "), "", innerContent, width, height, theme.Accent)
}
