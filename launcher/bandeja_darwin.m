// Ponte com o AppKit. Fica num .m à parte porque o arquivo Go com //export
// não pode ter definições C no preâmbulo.
#import <Cocoa/Cocoa.h>
#import <UserNotifications/UserNotifications.h>
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

// Delegate das notificações: mostra o aviso mesmo com o app à frente e manda
// o clique para o Go.
API_AVAILABLE(macos(11.0))
@interface DelegadoNotificacoes : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation DelegadoNotificacoes
- (void)userNotificationCenter:(UNUserNotificationCenter *)centro
       willPresentNotification:(UNNotification *)n
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions))fim {
  fim(UNNotificationPresentationOptionBanner | UNNotificationPresentationOptionList);
}

- (void)userNotificationCenter:(UNUserNotificationCenter *)centro
didReceiveNotificationResponse:(UNNotificationResponse *)r
         withCompletionHandler:(void (^)(void))fim {
  id consulta = r.notification.request.content.userInfo[@"consulta"];
  if ([consulta isKindOfClass:[NSString class]]) {
    notificacaoClicada((char *)[consulta UTF8String]);
  }
  fim();
}
@end

// notificar põe o aviso na Central de Notificações. Na primeira vez instala o
// delegate e pede a permissão (só alerta); o aviso sai depois da resposta.
// Sem permissão ou com erro, só registra no log. Fora de um .app (go run, go
// test) o UNUserNotificationCenter derruba o processo: nem tenta.
void notificar(const char *titulo, const char *texto, const char *consulta) {
  if (@available(macOS 11.0, *)) {
    if ([[NSBundle mainBundle] bundleIdentifier] == nil) {
      NSLog(@"IA de Bolso: sem notificação fora do .app");
      return;
    }
    UNMutableNotificationContent *c = [UNMutableNotificationContent new];
    c.title = [NSString stringWithUTF8String:titulo] ?: @"";
    c.body = [NSString stringWithUTF8String:texto] ?: @"";
    c.userInfo = @{@"consulta" : [NSString stringWithUTF8String:consulta] ?: @""};
    UNNotificationRequest *pedido =
        [UNNotificationRequest requestWithIdentifier:[[NSUUID UUID] UUIDString] content:c trigger:nil];

    UNUserNotificationCenter *centro = [UNUserNotificationCenter currentNotificationCenter];
    void (^mandar)(void) = ^{
      [centro addNotificationRequest:pedido
               withCompletionHandler:^(NSError *erro) {
                 if (erro != nil) {
                   NSLog(@"IA de Bolso: notificação não saiu: %@", erro);
                 }
               }];
    };

    static DelegadoNotificacoes *delegado; // o delegate é weak no centro: segura aqui
    static dispatch_once_t umaVez;
    __block BOOL primeira = NO;
    dispatch_once(&umaVez, ^{
      primeira = YES;
      delegado = [DelegadoNotificacoes new];
      centro.delegate = delegado;
      [centro requestAuthorizationWithOptions:UNAuthorizationOptionAlert
                            completionHandler:^(BOOL permitiu, NSError *erro) {
                              if (!permitiu || erro != nil) {
                                NSLog(@"IA de Bolso: notificações sem permissão: %@", erro);
                                return;
                              }
                              mandar();
                            }];
    });
    if (!primeira) {
      mandar();
    }
  }
}
