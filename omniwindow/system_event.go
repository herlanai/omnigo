package omniwindow

type SystemEventKind string

const (
	SystemDisplaysChanged   SystemEventKind = "displaysChanged"
	SystemAppearanceChanged SystemEventKind = "appearanceChanged"
	SystemWillSleep         SystemEventKind = "willSleep"
	SystemDidWake           SystemEventKind = "didWake"
	SystemScreensDidSleep   SystemEventKind = "screensDidSleep"
	SystemScreensDidWake    SystemEventKind = "screensDidWake"
	SystemSessionActive     SystemEventKind = "sessionActive"
	SystemSessionInactive   SystemEventKind = "sessionInactive"
)

// SystemEvent reports an OS notification, not a guarantee that state changed.
// State notifications are coalesced before UI dispatch. Sequence is per host;
// Dropped counts this subscription's discarded buffered events. Consumers can
// refresh current state when a gap matters. This is not a durable event log.
type SystemEvent struct {
	Kind              SystemEventKind
	Sequence, Dropped uint64
	Screen            *ScreenState
	Appearance        *AppearanceState
	Err               error
}
