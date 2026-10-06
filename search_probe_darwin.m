//go:build darwin && cgo

#import <AppKit/AppKit.h>
#include <stdlib.h>
#include <string.h>

// Character input is distinct from public virtual-key codes: '[' is ASCII 91,
// which the common key probe reserves for Command. Construct actual owned-view
// NSEvents with their characters, preserving punctuation and UTF-16 pairs.
const char *gocode_search_test_text(const char *title,const char *text) {
    const char *readback=getenv("GODESKTOP_READBACK");
    if(![NSThread isMainThread] || !readback || strcmp(readback,"1")) return "Native text probe requires UI thread and owned readback";
    NSString *name=[NSString stringWithUTF8String:title];
    NSWindow *owned=nil;for(NSWindow *window in NSApp.windows){if([window.title isEqualToString:name]){owned=window;break;}}
    if(!owned || !owned.contentView) return "Owned native text window is unavailable";
    NSString *characters=[NSString stringWithUTF8String:text];if(!characters)return "Native probe text is not UTF-8";
    for(NSUInteger i=0;i<characters.length;){
        NSUInteger length=1;unichar first=[characters characterAtIndex:i];
        if(first>=0xd800&&first<=0xdbff&&i+1<characters.length){unichar second=[characters characterAtIndex:i+1];if(second>=0xdc00&&second<=0xdfff)length=2;}
        NSString *part=[characters substringWithRange:NSMakeRange(i,length)];i+=length;
        NSEvent *event=[NSEvent keyEventWithType:NSEventTypeKeyDown location:NSZeroPoint modifierFlags:0 timestamp:NSProcessInfo.processInfo.systemUptime windowNumber:owned.windowNumber context:nil characters:part charactersIgnoringModifiers:part isARepeat:NO keyCode:0];
        if(!event)return "Cannot construct owned native character event";
        [owned.contentView keyDown:event];
    }
    return NULL;
}
