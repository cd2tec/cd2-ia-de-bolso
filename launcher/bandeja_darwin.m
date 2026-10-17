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

    // O delegate do Wails (conferido na v2.10.1) não trata o "reabrir" (dois
    // cliques no .app já aberto). Acrescenta o método, sem trocar o delegate.
    // Se uma versão nova do Wails passar a ter o método, o class_addMethod
    // recusa e o reopen fica com o do Wails: registra no log.
    Class c = [[NSApp delegate] class];
    SEL s = @selector(applicationShouldHandleReopen:hasVisibleWindows:);
    IMP imp = imp_implementationWithBlock(^BOOL(id self, NSApplication *app, BOOL visiveis) {
      appReaberto();
      return YES;
    });
    // BOOL é bool no arm64 e signed char no x86_64: a codificação vem do @encode.
    NSString *tipos = [NSString stringWithFormat:@"%s@:@%s", @encode(BOOL), @encode(BOOL)];
    if (!class_addMethod(c, s, imp, [tipos UTF8String])) {
      NSLog(@"IA de Bolso: reabrir não instalado em %@ (o delegate já tem o método)", NSStringFromClass(c));
    }

    // O X da janela (HideWindowOnClose) vira [NSApp hide:], sem aviso ao Go:
    // a notificação de app escondido é o jeito de saber que a janela fechou.
    [[NSNotificationCenter defaultCenter] addObserverForName:NSApplicationDidHideNotification
                                                      object:nil
                                                       queue:nil
                                                  usingBlock:^(NSNotification *n) { appEscondido(); }];

    bandejaNaThreadPrincipal();
  });
}

void trazerParaFrente(void) {
  dispatch_async(dispatch_get_main_queue(), ^{
    [NSApp activateIgnoringOtherApps:YES];
  });
}
