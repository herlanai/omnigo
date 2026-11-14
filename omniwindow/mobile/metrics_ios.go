//go:build ios

package mobile

/*
#cgo CFLAGS: -x objective-c -fblocks
#cgo LDFLAGS: -framework UIKit
#import <UIKit/UIKit.h>
#import <dispatch/dispatch.h>
typedef struct {double scale, top, right, bottom, left;} OmniMetrics;
static OmniMetrics omniMobileMetrics(int dark) {
 __block OmniMetrics result;
 void (^read)(void) = ^{
  UIWindow *window = nil;
  for (UIWindow *candidate in UIApplication.sharedApplication.windows) {
   if (candidate.isKeyWindow) { window = candidate; break; }
  }
  if (window) {window.overrideUserInterfaceStyle = dark ? UIUserInterfaceStyleDark : UIUserInterfaceStyleLight;}
  UIScreen *screen = window ? window.screen : UIScreen.mainScreen;
  UIEdgeInsets insets = window ? window.safeAreaInsets : UIEdgeInsetsZero;
  result = (OmniMetrics){screen.scale, insets.top, insets.right, insets.bottom, insets.left};
 };
 if (NSThread.isMainThread) { read(); } else { dispatch_sync(dispatch_get_main_queue(), read); }
 return result;
}
*/
import "C"

func nativeMetrics(_ float64, dark bool) metrics {
	value := C.int(0)
	if dark {
		value = 1
	}
	m := C.omniMobileMetrics(value)
	return metrics{float64(m.scale), float64(m.top), float64(m.right), float64(m.bottom), float64(m.left)}
}
