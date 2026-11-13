//go:build darwin && (amd64 || arm64)

package host

import (
	"github.com/ebitengine/purego/objc"
	"github.com/herlanai/omnigo/omniwindow"
)

// nativeAppearance runs on the UI thread. The override belongs to NSApplication;
// never write AppleInterfaceStyle or modify the user's desktop preferences.
func nativeAppearance(app objc.ID, source omniwindow.AppearanceSource) (omniwindow.AppearanceState, error) {
	if source != "" {
		var appearance objc.ID
		if source != omniwindow.AppearanceSystem {
			name := "NSAppearanceNameAqua"
			if source == omniwindow.AppearanceDark {
				name = "NSAppearanceNameDarkAqua"
			}
			appearance = objc.ID(objc.GetClass("NSAppearance")).Send(sel("appearanceNamed:"), stringID(name))
		}
		app.Send(sel("setAppearance:"), appearance)
	}
	state := omniwindow.AppearanceState{Source: omniwindow.AppearanceSystem}
	if override := app.Send(sel("appearance")); override != 0 {
		state.Source = omniwindow.AppearanceLight
		if appearanceDark(override) {
			state.Source = omniwindow.AppearanceDark
		}
	}
	state.Dark = appearanceDark(app.Send(sel("effectiveAppearance")))
	workspace := objc.ID(objc.GetClass("NSWorkspace")).Send(sel("sharedWorkspace"))
	state.ReduceMotion = workspace.Send(sel("accessibilityDisplayShouldReduceMotion")) != 0
	state.ReduceTransparency = workspace.Send(sel("accessibilityDisplayShouldReduceTransparency")) != 0
	state.HighContrast = workspace.Send(sel("accessibilityDisplayShouldIncreaseContrast")) != 0
	return state, nil
}

func appearanceDark(appearance objc.ID) bool {
	names := objc.ID(objc.GetClass("NSMutableArray")).Send(sel("array"))
	names.Send(sel("addObject:"), stringID("NSAppearanceNameAqua"))
	names.Send(sel("addObject:"), stringID("NSAppearanceNameDarkAqua"))
	match := appearance.Send(sel("bestMatchFromAppearancesWithNames:"), names)
	if match == 0 {
		return false
	}
	return objc.Send[string](match, sel("UTF8String")) == "NSAppearanceNameDarkAqua"
}
