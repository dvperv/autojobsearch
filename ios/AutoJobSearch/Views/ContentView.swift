import SwiftUI

struct ContentView: View {
    @EnvironmentObject var authViewModel: AuthViewModel
    @EnvironmentObject var hhViewModel: HHConnectViewModel
    @State private var selectedTab = 0
    @State private var showingHHConnect = false

    var body: some View {
        Group {
            if authViewModel.isLoading {
                SplashScreen()
            } else if authViewModel.isAuthenticated {
                mainTabView
            } else {
                WelcomeView()
            }
        }
        .onAppear {
            checkHHStatus()
        }
        .sheet(isPresented: $showingHHConnect) {
            HHConnectView()
            .environmentObject(hhViewModel)
        }
    }

    private var mainTabView: some View {
        TabView(selection: $selectedTab) {
            AutomationView()
            .tabItem {
                Image(systemName: "bolt.fill")
                Text("Автоматизация")
            }
            .tag(0)

            ApplicationsView()
            .tabItem {
                Image(systemName: "doc.text.fill")
                Text("Отклики")
            }
            .tag(1)

            SearchSettingsView()
            .tabItem {
                Image(systemName: "magnifyingglass")
                Text("Поиск")
            }
            .tag(2)

            ProfileView()
            .tabItem {
                Image(systemName: "person.fill")
                Text("Профиль")
            }
            .tag(3)
        }
        .overlay(alignment: .top) {
            if !hhViewModel.isConnected {
                HHConnectBanner(showingHHConnect: $showingHHConnect)
            }
        }
    }

    private func checkHHStatus() {
        Task {
            await hhViewModel.checkHHStatus()

            // Если пользователь аутентифицирован, но HH.ru не подключен,
            // показываем баннер через 2 секунды
            if authViewModel.isAuthenticated && !hhViewModel.isConnected {
                DispatchQueue.main.asyncAfter(deadline: .now() + 2) {
                    showingHHConnect = true
                }
            }
        }
    }
}

struct HHConnectBanner: View {
    @Binding var showingHHConnect: Bool

    var body: some View {
        Button(action: { showingHHConnect = true }) {
            HStack(spacing: 12) {
                Image(systemName: "exclamationmark.triangle.fill")
                .foregroundColor(.orange)

                VStack(alignment: .leading, spacing: 4) {
                    Text("Требуется подключение HH.ru")
                    .font(.subheadline)
                    .fontWeight(.semibold)

                    Text("Нажмите для подключения")
                    .font(.caption)
                    .foregroundColor(.secondary)
                }

                Spacer()

                Image(systemName: "chevron.right")
                .foregroundColor(.secondary)
            }
            .padding()
            .background(Color(.systemBackground))
            .cornerRadius(12)
            .shadow(color: .black.opacity(0.1), radius: 5, x: 0, y: 2)
            .padding(.horizontal)
            .padding(.top, 8)
        }
        .buttonStyle(PlainButtonStyle())
    }
}