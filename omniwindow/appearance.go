package omniwindow

// AppearanceSource selects an application override or follows the desktop.
type AppearanceSource string

const (
	AppearanceSystem AppearanceSource = "system"
	AppearanceLight  AppearanceSource = "light"
	AppearanceDark   AppearanceSource = "dark"
)

// AppearanceState describes the application's effective native appearance and
// desktop accessibility preferences. It does not change Go control themes or
// arbitrary frontend CSS; applications must apply those using their own state.
type AppearanceState struct {
	Source             AppearanceSource `json:"source"`
	Dark               bool             `json:"dark"`
	ReduceMotion       bool             `json:"reduceMotion"`
	ReduceTransparency bool             `json:"reduceTransparency"`
	HighContrast       bool             `json:"highContrast"`
}
