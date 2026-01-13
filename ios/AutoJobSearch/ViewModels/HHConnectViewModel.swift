import Foundation
import Combine

@MainActor
class HHConnectViewModel: ObservableObject {
    @Published var isConnected = false
    @Published var isLoading = false
    @Published var errorMessage: String?
    @Published var authUrl: String?
    @Published var userInfo: HHUserInfo?
    @Published var expiresAt: String?

    private let apiService = APIService.shared
    private var cancellables = Set<AnyCancellable>()

    // MARK: - Проверка статуса

    func checkHHStatus() async {
        isLoading = true
        errorMessage = nil

        do {
            let status = try await apiService.getHHStatus()
            updateFromStatus(status)
        } catch {
            errorMessage = "Не удалось проверить статус HH.ru: \(error.localizedDescription)"
        }

        isLoading = false
    }

    // MARK: - Получение URL авторизации

    func getAuthUrl() async {
        isLoading = true
        errorMessage = nil

        do {
            let response = try await apiService.getHHAuthUrl()
            authUrl = response.authUrl
        } catch {
            errorMessage = "Не удалось получить URL авторизации: \(error.localizedDescription)"
        }

        isLoading = false
    }

    // MARK: - Обмен кода на токены

    func exchangeCode(code: String, state: String) async {
        isLoading = true
        errorMessage = nil

        do {
            let request = HHConnectRequest(authorizationCode: code, state: state)
            let response = try await apiService.connectHHAccount(request: request)

            userInfo = HHUserInfo(
                firstName: response.userInfo.firstName,
                lastName: response.userInfo.lastName,
                email: response.userInfo.email,
                resumesCount: response.userInfo.resumesCount
            )
            expiresAt = response.tokensExpireAt
            isConnected = true

            // Сохраняем в UserDefaults
            UserDefaults.standard.set(true, forKey: "hh_connected")

        } catch {
            errorMessage = "Не удалось подключить HH.ru: \(error.localizedDescription)"
        }

        isLoading = false
    }

    // MARK: - Отключение

    func disconnectHH() async {
        isLoading = true
        errorMessage = nil

        do {
            try await apiService.disconnectHHAccount()

            // Сброс состояния
            isConnected = false
            userInfo = nil
            expiresAt = nil

            // Очистка UserDefaults
            UserDefaults.standard.removeObject(forKey: "hh_connected")

        } catch {
            errorMessage = "Не удалось отключить HH.ru: \(error.localizedDescription)"
        }

        isLoading = false
    }

    // MARK: - Вспомогательные методы

    private func updateFromStatus(_ status: HHStatusResponse) {
        isConnected = status.connected
        expiresAt = status.expiresAt

        if let userInfoResponse = status.userInfo {
            userInfo = HHUserInfo(
                firstName: userInfoResponse.firstName,
                lastName: userInfoResponse.lastName,
                email: userInfoResponse.email,
                resumesCount: userInfoResponse.resumesCount
            )
        }
    }

    func clearError() {
        errorMessage = nil
    }
}

// MARK: - Модели данных

struct HHUserInfo {
    let firstName: String?
    let lastName: String?
    let email: String?
    let resumesCount: Int
}

struct HHConnectRequest: Codable {
    let authorizationCode: String
    let state: String
}

struct HHAuthUrlResponse: Codable {
    let authUrl: String
    let state: String
}

struct HHConnectResponse: Codable {
    let message: String
    let userInfo: HHUserInfoResponse
    let tokensExpireAt: String
}

struct HHUserInfoResponse: Codable {
    let firstName: String?
    let lastName: String?
    let email: String?
    let resumesCount: Int
}

struct HHStatusResponse: Codable {
    let connected: Bool
    let expiresAt: String?
    let isExpired: Bool
    let userInfo: HHUserInfoResponse?
    let minutesLeft: Int?
}