package tui

import (
	"fmt"

	"github.com/jroimartin/gocui"
)

const (
	viewNameKeys    = "Keys"
	viewNameStatus  = "Status"
	viewNameLog     = "Log"
	boldTextColor   = "\033[34;1m"
	normalTextColor = "\033[34;2m"
	resetTextColor  = "\033[0m"
)

func (ui *UserInterface) views(g *gocui.Gui) error {
	ui.viewStatus(g)
	ui.viewLog(g)
	return nil
}

// viewKeys renders the keys view.
func (ui *UserInterface) viewKeys(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	v, err := g.SetView(viewNameKeys, 0, maxY-3, maxX-1, maxY-1)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}

	v.Title = viewNameKeys

	fmt.Fprintf(v, "%sQuit: %sq", normalTextColor, boldTextColor)
	fmt.Fprintf(v, "%s, Stop: %ss", normalTextColor, boldTextColor)
	fmt.Fprint(v, resetTextColor)

	return nil
}

// viewStatus renders the status view.
func (ui *UserInterface) viewStatus(g *gocui.Gui) error {
	maxX, _ := g.Size()

	v, err := g.SetView(viewNameStatus, 0, 0, maxX-1, 3)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}

	v.Title = fmt.Sprintf("%s (%s)", viewNameStatus, ui.displayName)

	return nil
}

// viewLog renders the log view
func (ui *UserInterface) viewLog(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	v, err := g.SetView(viewNameLog, 0, 8, maxX-1, maxY-5)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}

	v.Title = viewNameLog
	v.Autoscroll = true

	// get logrus to log to this view
	ui.logger.SetOutput(v)

	return nil
}
