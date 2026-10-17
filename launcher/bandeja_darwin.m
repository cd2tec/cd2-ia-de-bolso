// Ponte com o AppKit. Fica num .m à parte porque o arquivo Go com //export
// não pode ter definições C no preâmbulo.
#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include "_cgo_export.h"

void agendarNaThreadPrincipal(void) {
  dispatch_async(dispatch_get_main_queue(), ^{
    // O Wails põe a política Regular ao abrir, por cima do LSUIElement do
    // Info.plist: volta para Accessory, sem ícone no Dock.
    [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
    [NSApp activateIgnoringOtherApps:YES];

    // O delegate do Wails não trata o "reabrir" (dois cliques no .app já
    // aberto). Acrescenta o método, sem trocar o delegate.
    Class c = [[NSApp delegate] class];
    SEL s = @selector(applicationShouldHandleReopen:hasVisibleWindows:);
    IMP imp = imp_implementationWithBlock(^BOOL(id self, NSApplication *app, BOOL visiveis) {
      appReaberto();
      return YES;
    });
    class_addMethod(c, s, imp, "c@:@c");

    bandejaNaThreadPrincipal();
  });
}

void trazerParaFrente(void) {
  dispatch_async(dispatch_get_main_queue(), ^{
    [NSApp activateIgnoringOtherApps:YES];
  });
}
