package omniwindow

// ScreenRect is a rectangle in logical desktop pixels. The primary screen's
// top-left corner is (0,0); displays above or left of it have negative origins.
type ScreenRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Display is a detached snapshot, not a handle owning a native monitor.
// IDs can change after a display is disconnected or the system restarts.
type Display struct {
	ID          int64      `json:"id"`
	Label       string     `json:"label"`
	Bounds      ScreenRect `json:"bounds"`
	WorkArea    ScreenRect `json:"workArea"`
	ScaleFactor float64    `json:"scaleFactor"`
	Rotation    int        `json:"rotation"`
	Internal    bool       `json:"internal"`
	Primary     bool       `json:"primary"`
}

// ScreenState captures monitors and the cursor during one UI queue operation.
// The cursor uses the same desktop coordinates as Bounds and WorkArea.
type ScreenState struct {
	Displays []Display `json:"displays"`
	Cursor   Point     `json:"cursor"`
}
