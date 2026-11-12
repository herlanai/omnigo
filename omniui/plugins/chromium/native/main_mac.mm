// Author: HyperparameterSI
// CEF application event integration follows the CEF cefsimple sample (BSD).
#import <Cocoa/Cocoa.h>
#include "host.h"
#include "include/cef_application_mac.h"
#include "include/cef_command_line.h"
#include "include/wrapper/cef_helpers.h"
#include "include/wrapper/cef_library_loader.h"

@interface GoletChromiumApplication : NSApplication <CefAppProtocol> {
  BOOL handlingSendEvent_;
}
@end
@implementation GoletChromiumApplication
- (BOOL)isHandlingSendEvent { return handlingSendEvent_; }
- (void)setHandlingSendEvent:(BOOL)value { handlingSendEvent_ = value; }
- (void)sendEvent:(NSEvent*)event {
  CefScopedSendingEvent scope;
  [super sendEvent:event];
}
- (void)terminate:(id)sender { CloseGoletChromium(); }
@end

int main(int argc, char* argv[]) {
  CefScopedLibraryLoader library;
  if (!library.LoadInMain()) return 1;
  @autoreleasepool {
    [GoletChromiumApplication sharedApplication];
    [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
    auto args = CefCommandLine::CreateCommandLine();
    args->InitFromArgv(argc, argv);
    CefSettings settings;
    ConfigureGoletChromium(settings, args);
    // Studio nests this application inside its Resources. CEF otherwise assumes
    // the outermost app is the browser and resolves the wrong framework/helper.
    NSString* executable = [[NSString stringWithUTF8String:argv[0]] stringByStandardizingPath];
    NSString* bundle = [[[executable stringByDeletingLastPathComponent] stringByDeletingLastPathComponent] stringByDeletingLastPathComponent];
    NSString* frameworks = [bundle stringByAppendingPathComponent:@"Contents/Frameworks"];
    CefString(&settings.main_bundle_path) = [bundle UTF8String];
    CefString(&settings.framework_dir_path) = [[frameworks stringByAppendingPathComponent:@"Chromium Embedded Framework.framework"] UTF8String];
    CefString(&settings.browser_subprocess_path) = [[frameworks stringByAppendingPathComponent:@"Omni Browser Helper.app/Contents/MacOS/Omni Browser Helper"] UTF8String];
    auto app = CreateGoletChromiumApp();
    if (!CefInitialize(CefMainArgs(argc, argv), settings, app, nullptr)) return CefGetExitCode();
    CefRunMessageLoop();
    CefShutdown();
  }
  return 0;
}
