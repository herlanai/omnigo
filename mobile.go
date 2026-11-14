package omnigo

import "github.com/herlanai/omnigo/omniwindow/mobile"

// MobileOptions configures an experimental Go control mobile application.
type MobileOptions = mobile.Options
type MobileContent = mobile.Content

// RunMobile starts the native iOS/Android event loop on the process main thread.
// Desktop applications continue to use Run. No browser implements Go controls.
func RunMobile(options MobileOptions) error { return mobile.Run(options) }
