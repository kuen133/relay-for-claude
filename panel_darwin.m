// Kuen: the frosted-glass popover that replaces the plain menu on a left
// click. A borderless panel holds a system blur (NSVisualEffectView) masked to
// the cards the page lays out, with a transparent WKWebView drawing on top.

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#include "panel_darwin.h"

extern void panelMessage(char *json);

@interface KuenPanel : NSPanel
@end
@implementation KuenPanel
- (BOOL)canBecomeKeyWindow { return YES; }
- (void)cancelOperation:(id)sender { [self orderOut:nil]; }
@end

@interface KuenBridge : NSObject <WKScriptMessageHandler, NSWindowDelegate>
@end

static KuenPanel *panel;
static NSVisualEffectView *glass;
static WKWebView *web;
static KuenBridge *bridge;
// The status icon the panel hangs from, in screen coordinates.
static NSPoint anchor;
static CGFloat mainOffset, mainWidth = 360;
static NSDate *hiddenAt;

static NSImage *maskFor(NSSize size, NSArray *cards) {
  return [NSImage imageWithSize:size flipped:YES drawingHandler:^BOOL(NSRect r) {
    [[NSColor blackColor] setFill];
    for (NSDictionary *c in cards) {
      NSRect f = NSMakeRect([c[@"x"] doubleValue], [c[@"y"] doubleValue], [c[@"w"] doubleValue], [c[@"h"] doubleValue]);
      CGFloat radius = [c[@"r"] doubleValue];
      [[NSBezierPath bezierPathWithRoundedRect:f xRadius:radius yRadius:radius] fill];
    }
    return YES;
  }];
}

static NSScreen *screenAt(NSPoint p) {
  for (NSScreen *s in [NSScreen screens]) {
    if (NSPointInRect(p, s.frame)) return s;
  }
  return [NSScreen mainScreen];
}

// place sizes the window so the main card sits centred under the icon,
// kept on the screen.
static void place(CGFloat w, CGFloat h) {
  NSRect vis = screenAt(anchor).visibleFrame;
  CGFloat mainX = anchor.x - mainWidth / 2;
  mainX = MIN(MAX(mainX, NSMinX(vis) + 8 + mainOffset), NSMaxX(vis) - 8 - mainWidth);
  CGFloat x = MIN(MAX(mainX - mainOffset, NSMinX(vis) + 8), NSMaxX(vis) - 8 - w);
  NSRect frame = NSMakeRect(x, NSMaxY(vis) - 6 - h, w, h);
  [panel setFrame:frame display:YES];
  glass.frame = NSMakeRect(0, 0, w, h);
  web.frame = NSMakeRect(0, 0, w, h);
}

static void runJS(NSString *js) {
  [web evaluateJavaScript:js completionHandler:nil];
}

@implementation KuenBridge
- (void)userContentController:(WKUserContentController *)ucc didReceiveScriptMessage:(WKScriptMessage *)msg {
  if (![msg.body isKindOfClass:[NSString class]]) return;
  NSString *body = msg.body;
  NSData *data = [body dataUsingEncoding:NSUTF8StringEncoding];
  NSDictionary *m = [NSJSONSerialization JSONObjectWithData:data options:0 error:nil];
  if ([m[@"cmd"] isEqual:@"layout"]) {
    CGFloat w = [m[@"w"] doubleValue], h = [m[@"h"] doubleValue];
    mainOffset = [m[@"mainX"] doubleValue];
    // Frost 0..1 from the settings slider: a fainter blur lets more of the
    // unblurred desktop through, as Apple's thinner materials do.
    if (m[@"frost"]) glass.alphaValue = 0.15 + 0.85 * [m[@"frost"] doubleValue];
    mainWidth = [m[@"mainW"] doubleValue];
    place(w, h);
    glass.maskImage = maskFor(NSMakeSize(w, h), m[@"cards"]);
    return;
  }
  if ([m[@"cmd"] isEqual:@"close"]) {
    [panel orderOut:nil];
    return;
  }
  panelMessage((char *)[body UTF8String]);
}
- (void)windowDidResignKey:(NSNotification *)n {
  [panel orderOut:nil];
  hiddenAt = [NSDate date];
}
@end

void panelInit(const char *html) {
  NSString *page = [NSString stringWithUTF8String:html];
  dispatch_async(dispatch_get_main_queue(), ^{
    panel = [[KuenPanel alloc] initWithContentRect:NSMakeRect(0, 0, 380, 560)
                                         styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel
                                           backing:NSBackingStoreBuffered
                                             defer:NO];
    panel.opaque = NO;
    panel.backgroundColor = [NSColor clearColor];
    // The page draws its own glow and shadow around the cards.
    panel.hasShadow = NO;
    panel.level = NSPopUpMenuWindowLevel;
    panel.hidesOnDeactivate = NO;
    panel.collectionBehavior = NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorFullScreenAuxiliary;
    panel.appearance = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
    bridge = [KuenBridge new];
    panel.delegate = bridge;

    NSView *root = [[NSView alloc] initWithFrame:panel.contentView.bounds];
    panel.contentView = root;
    glass = [[NSVisualEffectView alloc] initWithFrame:root.bounds];
    // Popover is the most see-through of the dark materials; the page adds the tint.
    glass.material = NSVisualEffectMaterialPopover;
    glass.blendingMode = NSVisualEffectBlendingModeBehindWindow;
    glass.state = NSVisualEffectStateActive;
    [root addSubview:glass];

    WKWebViewConfiguration *cfg = [WKWebViewConfiguration new];
    [cfg.userContentController addScriptMessageHandler:bridge name:@"app"];
    web = [[WKWebView alloc] initWithFrame:root.bounds configuration:cfg];
    [web setValue:@NO forKey:@"drawsBackground"];
    if (@available(macOS 12.0, *)) web.underPageBackgroundColor = [NSColor clearColor];
    [root addSubview:web];
    [web loadHTMLString:page baseURL:nil];
  });
}

void panelToggle(void) {
  dispatch_async(dispatch_get_main_queue(), ^{
    if (panel.visible) {
      [panel orderOut:nil];
      return;
    }
    // A click on the icon while the panel is open first takes focus away,
    // which already hid it; that same click must not reopen it.
    if (hiddenAt && -[hiddenAt timeIntervalSinceNow] < 0.3) return;
    anchor = [NSEvent mouseLocation];
    CGFloat right = NSMaxX(screenAt(anchor).visibleFrame) - anchor.x;
    runJS([NSString stringWithFormat:@"window.opened && window.opened(%f)", right]);
    place(panel.frame.size.width, panel.frame.size.height);
    [NSApp activateIgnoringOtherApps:YES];
    [panel makeKeyAndOrderFront:nil];
  });
}

void panelHide(void) {
  dispatch_async(dispatch_get_main_queue(), ^{ [panel orderOut:nil]; });
}

void panelUpdate(const char *json) {
  NSString *state = [NSString stringWithUTF8String:json];
  dispatch_async(dispatch_get_main_queue(), ^{
    runJS([NSString stringWithFormat:@"window.render && window.render(%@)", state]);
  });
}
