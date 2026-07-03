package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	BorderFrameHeight = 2
	BorderFrameWidth  = 2
)

// RenderBtopBox creates a btop-style box with title embedded in the top border.
func RenderBtopBox(leftTitle, rightTitle, content string, width, height int, borderColor string) string {
	const (
		topLeft     = "╭"
		topRight    = "╮"
		bottomLeft  = "╰"
		bottomRight = "╯"
		horizontal  = "─"
		vertical    = "│"
	)
	
	innerWidth := width - BorderFrameWidth
	if innerWidth < 1 {
		innerWidth = 1
	}

	leftTitleWidth := lipgloss.Width(leftTitle)
	rightTitleWidth := lipgloss.Width(rightTitle)

	const minBorderDashes = 1
	maxTitleSpace := innerWidth - minBorderDashes
	if maxTitleSpace <= 0 {
		leftTitle = ""
		leftTitleWidth = 0
		rightTitle = ""
		rightTitleWidth = 0
	} else if leftTitleWidth+rightTitleWidth > maxTitleSpace {
		half := maxTitleSpace / 2
		if leftTitleWidth > half {
			leftTitle = lipgloss.NewStyle().MaxWidth(half).Render(leftTitle)
			leftTitleWidth = lipgloss.Width(leftTitle)
		}
		if leftTitleWidth+rightTitleWidth > maxTitleSpace {
			rightRemaining := maxTitleSpace - leftTitleWidth
			if rightRemaining <= 0 {
				rightTitle = ""
				rightTitleWidth = 0
			} else {
				rightTitle = lipgloss.NewStyle().MaxWidth(rightRemaining).Render(rightTitle)
				rightTitleWidth = lipgloss.Width(rightTitle)
			}
		}
	}

	borderStyler := lipgloss.NewStyle().Foreground(lipgloss.Color(borderColor))
	var topBorder string

	if leftTitle != "" && rightTitle != "" {
		remainingWidth := innerWidth - leftTitleWidth - rightTitleWidth - lipgloss.Width(horizontal)
		if remainingWidth < 0 { remainingWidth = 0 }
		topBorder = borderStyler.Render(topLeft+horizontal) +
			leftTitle +
			borderStyler.Render(strings.Repeat(horizontal, remainingWidth)) +
			rightTitle +
			borderStyler.Render(topRight)
	} else if leftTitle != "" {
		remainingWidth := innerWidth - leftTitleWidth - lipgloss.Width(horizontal)
		if remainingWidth < 0 { remainingWidth = 0 }
		topBorder = borderStyler.Render(topLeft+horizontal) +
			leftTitle +
			borderStyler.Render(strings.Repeat(horizontal, remainingWidth)+topRight)
	} else if rightTitle != "" {
		remainingWidth := innerWidth - rightTitleWidth - lipgloss.Width(horizontal)
		if remainingWidth < 0 { remainingWidth = 0 }
		topBorder = borderStyler.Render(topLeft+strings.Repeat(horizontal, remainingWidth)) +
			rightTitle +
			borderStyler.Render(horizontal+topRight)
	} else {
		topBorder = borderStyler.Render(topLeft + strings.Repeat(horizontal, innerWidth) + topRight)
	}

	bottomBorder := borderStyler.Render(
		bottomLeft + strings.Repeat(horizontal, innerWidth) + bottomRight,
	)

	contentLines := strings.Split(content, "\n")
	innerHeight := height - BorderFrameHeight
	if innerHeight < 0 {
		innerHeight = 0
	}

	truncStyle := lipgloss.NewStyle().MaxWidth(innerWidth)

	var wrappedLines []string
	for i := 0; i < innerHeight; i++ {
		var line string
		if i < len(contentLines) {
			line = contentLines[i]
		}
		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth {
			line = line + strings.Repeat(" ", innerWidth-lineWidth)
		} else if lineWidth > innerWidth {
			line = truncStyle.Render(line)
		}
		wrappedLines = append(wrappedLines, borderStyler.Render(vertical)+line+borderStyler.Render(vertical))
	}

	return lipgloss.JoinVertical(lipgloss.Left, topBorder, strings.Join(wrappedLines, "\n"), bottomBorder)
}
