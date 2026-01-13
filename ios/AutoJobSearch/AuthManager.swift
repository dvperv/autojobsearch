import Foundation
import Security
import LocalAuthentication

actor AuthManager {
    static let shared = AuthManager()

    private let keychain = KeychainService.shared
    private let biometricAuth = BiometricAuthService()

    private let accessTokenKey = "com.autojobsearch.access_token"
    private let refreshTokenKey = "com.autojobsearch.refresh_token"
    private let expiresAtKey = "com.autojobsearch.expires_at"
    private let userKey = "com.autojobsearch.user"

    // MARK: - Токены

    func getTokens() -> AuthTokens? {
        guard let accessToken = keychain.getString(accessTokenKey),
        let refreshToken = keychain.getString(refreshTokenKey),
        let expiresAtString = keychain.getString(expiresAtKey),
        let expiresAt = ISO8601DateFormatter().date(from: expiresAtString) else {
            return nil
        }

        return AuthTokens(
            accessToken: accessToken,
            refreshToken: refreshToken,
            expiresAt: expiresAt
        )
    }

    func saveTokens(_ tokens: AuthTokens) {
        keychain.set(tokens.accessToken, forKey: accessTokenKey)
        keychain.set(tokens.refreshToken, forKey: refreshTokenKey)

        let expiresAtString = ISO8601DateFormatter().string(from: tokens.expiresAt)
        keychain.set(expiresAtString, forKey: expiresAtKey)
    }

    func clearTokens() {
        keychain.delete(accessTokenKey)
        keychain.delete(refreshTokenKey)
        keychain.delete(expiresAtKey)
        keychain.delete(userKey)
    }

    func isAccessTokenValid() -> Bool {
        guard let expiresAtString = keychain.getString(expiresAtKey),
        let expiresAt = ISO8601DateFormatter().date(from: expiresAtString) else {
            return false
        }

        // Добавляем буфер в 5 минут
        return expiresAt > Date().addingTimeInterval(5 * 60)
    }

    func getAccessToken() -> String? {
        return keychain.getString(accessTokenKey)
    }

    // MARK: - Пользователь

    func saveUser(_ user: UserResponse) {
        if let data = try? JSONEncoder().encode(user) {
            keychain.set(data, forKey: userKey)
        }
    }

    func getUser() -> UserResponse? {
        guard let data = keychain.getData(userKey) else {
            return nil
        }

        return try? JSONDecoder().decode(UserResponse.self, from: data)
    }

    // MARK: - Биометрия

    func enableBiometricAuth() async throws {
        try await biometricAuth.enableBiometricAuth()
    }

    func authenticateWithBiometrics() async -> Bool {
        return await biometricAuth.authenticate()
    }
}

// MARK: - Keychain Service

class KeychainService {
    static let shared = KeychainService()

    private init() {}

    func set(_ value: String, forKey key: String) {
        guard let data = value.data(using: .utf8) else { return }

        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrAccount as String: key,
            kSecValueData as String: data,
            kSecAttrAccessible as String: kSecAttrAccessibleWhenUnlockedThisDeviceOnly
        ]

        // Удаляем старые данные
        SecItemDelete(query as CFDictionary)

        // Добавляем новые
        SecItemAdd(query as CFDictionary, nil)
    }

    func set(_ data: Data, forKey key: String) {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrAccount as String: key,
            kSecValueData as String: data,
            kSecAttrAccessible as String: kSecAttrAccessibleWhenUnlockedThisDeviceOnly
        ]

        SecItemDelete(query as CFDictionary)
        SecItemAdd(query as CFDictionary, nil)
    }

    func getString(_ key: String) -> String? {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrAccount as String: key,
            kSecReturnData as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne
        ]

        var result: AnyObject?
        let status = SecItemCopyMatching(query as CFDictionary, &result)

        guard status == errSecSuccess,
        let data = result as? Data,
        let string = String(data: data, encoding: .utf8) else {
            return nil
        }

        return string
    }

    func getData(_ key: String) -> Data? {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrAccount as String: key,
            kSecReturnData as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne
        ]

        var result: AnyObject?
        let status = SecItemCopyMatching(query as CFDictionary, &result)

        guard status == errSecSuccess,
        let data = result as? Data else {
            return nil
        }

        return data
    }

    func delete(_ key: String) {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrAccount as String: key
        ]

        SecItemDelete(query as CFDictionary)
    }
}

// MARK: - Biometric Auth Service

actor BiometricAuthService {
    private let context = LAContext()
    private var error: NSError?

    func enableBiometricAuth() async throws {
        guard context.canEvaluatePolicy(.deviceOwnerAuthenticationWithBiometrics, error: &error) else {
            throw BiometricError.notAvailable
        }

        // Сохраняем настройку в UserDefaults
        UserDefaults.standard.set(true, forKey: "biometric_auth_enabled")
    }

    func authenticate() async -> Bool {
        let reason = "Войдите в AutoJobSearch"

        do {
            return try await context.evaluatePolicy(.deviceOwnerAuthenticationWithBiometrics, localizedReason: reason)
        } catch {
            print("Biometric authentication failed: \(error)")
            return false
        }
    }
}

enum BiometricError: Error {
    case notAvailable
    case authenticationFailed
}