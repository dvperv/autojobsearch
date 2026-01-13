import SwiftUI
import Firebase
import FirebaseMessaging

@main
struct AutoJobSearchApp: App {
    @UIApplicationDelegateAdaptor(AppDelegate.self) var delegate
    @StateObject private var authViewModel = AuthViewModel()
    @StateObject private var hhViewModel = HHConnectViewModel()

    init() {
        FirebaseApp.configure()
        setupAppearance()
    }

    var body: some Scene {
        WindowGroup {
            ContentView()
            .environmentObject(authViewModel)
            .environmentObject(hhViewModel)
            .environmentObject(AutomationViewModel())
            .onOpenURL { url in
                // Обработка OAuth callback от HH.ru
                handleOAuthCallback(url: url)
            }
        }
    }

    private func setupAppearance() {
        // Настройка внешнего вида приложения
        UINavigationBar.appearance().tintColor = UIColor(named: "PrimaryColor")
        UITabBar.appearance().tintColor = UIColor(named: "PrimaryColor")
    }

    private func handleOAuthCallback(url: URL) {
        // Извлечение кода авторизации из URL
        guard let components = URLComponents(url: url, resolvingAgainstBaseURL: false),
        let code = components.queryItems?.first(where: { $0.name == "code" })?.value,
        let state = components.queryItems?.first(where: { $0.name == "state" })?.value else {
            return
        }

        // Обмен кода на токены
        Task {
            await hhViewModel.exchangeCode(code: code, state: state)
        }
    }
}

class AppDelegate: NSObject, UIApplicationDelegate {
    func application(_ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey : Any]? = nil) -> Bool {

        // Настройка уведомлений
        UNUserNotificationCenter.current().delegate = self

        let authOptions: UNAuthorizationOptions = [.alert, .badge, .sound]
        UNUserNotificationCenter.current().requestAuthorization(
            options: authOptions) { granted, error in
            if granted {
                DispatchQueue.main.async {
                    application.registerForRemoteNotifications()
                }
            }
        }

        // Настройка Firebase Messaging
        Messaging.messaging().delegate = self

        return true
    }

    func application(_ application: UIApplication,
    didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data) {
        Messaging.messaging().apnsToken = deviceToken
    }
}

extension AppDelegate: UNUserNotificationCenterDelegate {
    func userNotificationCenter(_ center: UNUserNotificationCenter,
    willPresent notification: UNNotification,
    withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void) {
        completionHandler([[.banner, .sound]])
    }

    func userNotificationCenter(_ center: UNUserNotificationCenter,
    didReceive response: UNNotificationResponse,
    withCompletionHandler completionHandler: @escaping () -> Void) {
        let userInfo = response.notification.request.content.userInfo

        // Обработка нажатия на уведомление
        if let automationId = userInfo["automation_id"] as? String {
            NotificationCenter.default.post(
                name: NSNotification.Name("NavigateToAutomation"),
                object: nil,
                userInfo: ["automation_id": automationId]
            )
        }

        completionHandler()
    }
}

extension AppDelegate: MessagingDelegate {
    func messaging(_ messaging: Messaging, didReceiveRegistrationToken fcmToken: String?) {
        guard let fcmToken = fcmToken else { return }

        // Отправка FCM токена на сервер
        Task {
            await sendFCMTokenToServer(token: fcmToken)
        }
    }

    private func sendFCMTokenToServer(token: String) async {
        guard let authToken = await AuthManager.shared.getAccessToken() else { return }

        let url = URL(string: "\(Config.baseURL)/api/notifications/register-device")!
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("Bearer \(authToken)", forHTTPHeaderField: "Authorization")
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")

        let body: [String: Any] = [
            "device_token": token,
            "platform": "ios",
            "device_id": UIDevice.current.identifierForVendor?.uuidString ?? ""
        ]

        request.httpBody = try? JSONSerialization.data(withJSONObject: body)

        do {
            let (_, response) = try await URLSession.shared.data(for: request)
            guard let httpResponse = response as? HTTPURLResponse,
            httpResponse.statusCode == 200 else {
                print("Failed to register device token")
                return
            }
            print("Device token registered successfully")
        } catch {
            print("Error registering device token: \(error)")
        }
    }
}