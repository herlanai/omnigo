import Foundation
import CoreGraphics
let pid = Int(CommandLine.arguments[1])!
let windows = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] ?? []
for window in windows {
 if window[kCGWindowOwnerPID as String] as? Int == pid,
    let bounds = window[kCGWindowBounds as String] as? [String: Any],
    let width = bounds["Width"] as? Double, width > 200 {
  print("PASS: visible application window", bounds)
  exit(0)
 }
}
exit(1)
