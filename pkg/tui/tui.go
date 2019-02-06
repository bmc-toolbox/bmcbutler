package tui

import (
	"sync"

	"github.com/jroimartin/gocui"
	"github.com/sirupsen/logrus"
)

// UserInterface holds attributes to setup the terminal ui
type UserInterface struct {
	displayName string
	gui         *gocui.Gui
	wg          *sync.WaitGroup
	logger      *logrus.Logger
	stopChan    <-chan struct{}
}

// NewUserInterface returns a new terminal user interface
func NewUserInterface(stopChan <-chan struct{}, wg *sync.WaitGroup, logger *logrus.Logger) (*UserInterface, error) {

	// Get a GUI from gocui
	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		return nil, err
	}

	// Create a UserInterface with the GUI
	ui := &UserInterface{
		displayName: "bmcbutler",
		gui:         g,
		wg:          wg,
		logger:      logger,
	}

	g.SetManagerFunc(ui.views)

	return ui, nil

}

// Run starts the UserInterface
func (ui *UserInterface) Run() error {
	defer ui.gui.Close()

	// Run the main gocui loop as a routine
	ui.wg.Add(1)
	go func() {
		err := ui.gui.MainLoop()
		if err != nil && err != gocui.ErrQuit {
			logrus.WithError(err).Error("Error running gocui")
		}
		ui.wg.Done()
	}()

	ui.wg.Wait()
	return nil
}

// Stop tells the UserInterface to stop
func (ui *UserInterface) Stop(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
