import SwiftUI
import WebKit

struct HHConnectView: View {
    @EnvironmentObject var viewModel: HHConnectViewModel
    @Environment(\.dismiss) var dismiss
    @State private var showingWebView = false
    @State private var authURL = ""

    var body: some View {
        NavigationView {
            ScrollView {
                VStack(spacing: 20) {
                    // Статус подключения
                    ConnectionStatusCard()

                    // Преимущества
                    BenefitsCard()

                    // Кнопки действий
                    ActionButtons()

                    // Информация о безопасности
                    SecurityInfo()
                }
                .padding()
            }
            .navigationTitle("Подключение HH.ru")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .navigationBarTrailing) {
                    Button("Готово") {
                        dismiss()
                    }
                }
            }
        }
        .sheet(isPresented: $showingWebView) {
            NavigationView {
                WebView(url: URL(string: authURL)!)
                .navigationTitle("Авторизация HH.ru")
                .navigationBarTitleDisplayMode(.inline)
                .toolbar {
                    ToolbarItem(placement: .navigationBarLeading) {
                        Button("Отмена") {
                            showingWebView = false
                        }
                    }
                }
            }
        }
        .onAppear {
            Task {
                await viewModel.checkHHStatus()
            }
        }
    }

    // MARK: - Компоненты

    private func ConnectionStatusCard() -> some View {
        CardView {
            VStack(alignment: .leading, spacing: 12) {
                HStack {
                    VStack(alignment: .leading, spacing: 4) {
                        Text(viewModel.isConnected ? "HH.ru подключен" : "HH.ru не подключен")
                        .font(.headline)
                        .foregroundColor(viewModel.isConnected ? .green : .gray)

                        if viewModel.isConnected, let userInfo = viewModel.userInfo {
                            Text("\(userInfo.firstName ?? "") \(userInfo.lastName ?? "")")
                            .font(.subheadline)
                            .foregroundColor(.secondary)

                            if let expiresAt = viewModel.expiresAt {
                                Text("Действителен до: \(expiresAt)")
                                .font(.caption)
                                .foregroundColor(.secondary)
                            }
                        }
                    }

                    Spacer()

                    if viewModel.isConnected {
                        Image(systemName: "checkmark.circle.fill")
                        .foregroundColor(.green)
                        .font(.title2)
                    }
                }
            }
        }
    }

    private func BenefitsCard() -> some View {
        CardView {
            VStack(alignment: .leading, spacing: 16) {
                Text("Преимущества подключения")
                .font(.headline)

                BenefitRow(
                    icon: "bolt.fill",
                    title: "Автоотклики от вашего имени",
                    description: "Отклики отправляются через ваш аккаунт HH.ru"
                )

                BenefitRow(
                    icon: "lock.fill",
                    title: "Безопасность",
                    description: "Мы не храним ваш пароль от HH.ru"
                )

                BenefitRow(
                    icon: "arrow.triangle.2.circlepath",
                    title: "Синхронизация резюме",
                    description: "Используем ваши резюме с HH.ru для откликов"
                )

                BenefitRow(
                    icon: "bell.fill",
                    title: "Уведомления",
                    description: "Получайте уведомления о приглашениях"
                )
            }
        }
    }

    private func ActionButtons() -> some View {
        VStack(spacing: 12) {
            if viewModel.isConnected {
                Button(action: {
                    Task {
                        await viewModel.disconnectHH()
                    }
                }) {
                    HStack {
                        Image(systemName: "link.slash")
                        Text("Отключить HH.ru")
                    }
                    .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
                .tint(.red)
            } else {
                Button(action: {
                    Task {
                        await getAuthUrl()
                    }
                }) {
                    HStack {
                        if viewModel.isLoading {
                            ProgressView()
                            .progressViewStyle(CircularProgressViewStyle())
                            .scaleEffect(0.8)
                        } else {
                            Image(systemName: "link")
                        }
                        Text("Подключить HH.ru")
                    }
                    .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
                .disabled(viewModel.isLoading)
            }

            Text("Подключив HH.ru, вы соглашаетесь с условиями использования")
            .font(.caption)
            .foregroundColor(.secondary)
            .multilineTextAlignment(.center)
        }
    }

    private func SecurityInfo() -> some View {
        CardView(backgroundColor: Color(.systemGray6)) {
            VStack(alignment: .leading, spacing: 8) {
                Text("Безопасность данных")
                .font(.subheadline)
                .fontWeight(.semibold)

                Text("• Мы используем официальный OAuth протокол HH.ru\n• Ваши данные защищены шифрованием\n• Вы можете отключить доступ в любой момент")
                .font(.caption)
                .foregroundColor(.secondary)
            }
        }
    }

    // MARK: - Вспомогательные методы

    private func getAuthUrl() async {
        await viewModel.getAuthUrl()
        if let url = viewModel.authUrl {
            authURL = url
            showingWebView = true
        }
    }
}

// MARK: - Компоненты

struct CardView<Content: View>: View {
    let backgroundColor: Color
    let content: Content

    init(backgroundColor: Color = Color(.systemBackground), @ViewBuilder content: () -> Content) {
        self.backgroundColor = backgroundColor
        self.content = content()
    }

    var body: some View {
        content
        .padding()
        .background(backgroundColor)
        .cornerRadius(12)
        .shadow(color: .black.opacity(0.05), radius: 5, x: 0, y: 2)
    }
}

struct BenefitRow: View {
    let icon: String
    let title: String
    let description: String

    var body: some View {
        HStack(spacing: 12) {
            Image(systemName: icon)
            .foregroundColor(.blue)
            .frame(width: 24)

            VStack(alignment: .leading, spacing: 2) {
                Text(title)
                .font(.subheadline)
                .fontWeight(.medium)

                Text(description)
                .font(.caption)
                .foregroundColor(.secondary)
                .lineLimit(2)
            }
        }
    }
}

struct WebView: UIViewRepresentable {
    let url: URL

    func makeUIView(context: Context) -> WKWebView {
        let webView = WKWebView()
        webView.navigationDelegate = context.coordinator
        webView.load(URLRequest(url: url))
        return webView
    }

    func updateUIView(_ uiView: WKWebView, context: Context) {}

    func makeCoordinator() -> Coordinator {
        Coordinator()
    }

    class Coordinator: NSObject, WKNavigationDelegate {
        func webView(_ webView: WKWebView, decidePolicyFor navigationAction: WKNavigationAction, decisionHandler: @escaping (WKNavigationActionPolicy) -> Void) {
            // Ловим callback URL с кодом авторизации
            if let url = navigationAction.request.url,
            url.absoluteString.contains("code="),
            url.absoluteString.contains("state=") {

                // Извлекаем код и state
                if let components = URLComponents(url: url, resolvingAgainstBaseURL: false),
                let code = components.queryItems?.first(where: { $0.name == "code" })?.value,
                let state = components.queryItems?.first(where: { $0.name == "state" })?.value {

                    // Отправляем код на обмен
                    NotificationCenter.default.post(
                        name: NSNotification.Name("HH_OAuth_Callback"),
                        object: nil,
                        userInfo: ["code": code, "state": state]
                    )
                }

                decisionHandler(.cancel)
                return
            }

            decisionHandler(.allow)
        }
    }
}