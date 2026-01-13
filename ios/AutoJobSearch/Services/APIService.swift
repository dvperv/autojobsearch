import Foundation
import Combine

actor APIService {
    static let shared = APIService()
    private let baseURL = Config.baseURL
    private let session: URLSession
    private var refreshTask: Task<AuthTokens, Error>?

    private init() {
        let configuration = URLSessionConfiguration.default
        configuration.timeoutIntervalForRequest = 30
        configuration.timeoutIntervalForResource = 60
        configuration.httpAdditionalHeaders = [
            "User-Agent": "AutoJobSearch/iOS/1.0",
            "Accept": "application/json",
            "Accept-Language": "ru-RU"
        ]
        session = URLSession(configuration: configuration)
    }

    // MARK: - Базовый метод запроса

    private func request<T: Decodable>(
    path: String,
    method: String = "GET",
    body: Data? = nil,
    requiresAuth: Bool = true
    ) async throws -> T {
        guard let url = URL(string: baseURL + path) else {
            throw APIError.invalidURL
        }

        var request = URLRequest(url: url)
        request.httpMethod = method
        request.httpBody = body
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")

        // Добавление токена авторизации
        if requiresAuth {
            let tokens = try await getValidTokens()
            request.setValue("Bearer \(tokens.accessToken)", forHTTPHeaderField: "Authorization")
        }

        // Выполнение запроса
        let (data, response) = try await session.data(for: request)

        guard let httpResponse = response as? HTTPURLResponse else {
            throw APIError.invalidResponse
        }

        // Проверка статус кода
        guard 200..<300 ~= httpResponse.statusCode else {
            if httpResponse.statusCode == 401 {
                // Неавторизован, возможно токен истек
                throw APIError.unauthorized
            }
            throw APIError.httpError(statusCode: httpResponse.statusCode)
        }

        // Парсинг ответа
        do {
            let decoder = JSONDecoder()
            decoder.dateDecodingStrategy = .iso8601
            return try decoder.decode(T.self, from: data)
        } catch {
            throw APIError.decodingError
        }
    }

    // MARK: - Работа с токенами

    private func getValidTokens() async throws -> AuthTokens {
        // Получение текущих токенов
        guard let tokens = await AuthManager.shared.getTokens() else {
            throw APIError.unauthorized
        }

        // Проверка срока действия access token
        if await AuthManager.shared.isAccessTokenValid() {
            return tokens
        }

        // Попытка обновить токены
        return try await refreshTokens(refreshToken: tokens.refreshToken)
    }

    private func refreshTokens(refreshToken: String) async throws -> AuthTokens {
        // Проверяем, нет ли уже выполняющейся задачи обновления
        if let refreshTask = refreshTask {
            return try await refreshTask.value
        }

        // Создаем новую задачу обновления
        let task = Task<AuthTokens, Error> {
            defer { refreshTask = nil }

            let request = RefreshTokenRequest(refreshToken: refreshToken)
            let body = try JSONEncoder().encode(request)

            var urlRequest = URLRequest(url: URL(string: baseURL + "/api/auth/refresh")!)
            urlRequest.httpMethod = "POST"
            urlRequest.httpBody = body
            urlRequest.setValue("application/json", forHTTPHeaderField: "Content-Type")

            let (data, response) = try await session.data(for: urlRequest)

            guard let httpResponse = response as? HTTPURLResponse,
            httpResponse.statusCode == 200 else {
                throw APIError.unauthorized
            }

            let tokens = try JSONDecoder().decode(AuthTokens.self, from: data)

            // Сохранение новых токенов
            await AuthManager.shared.saveTokens(tokens)

            return tokens
        }

        refreshTask = task
        return try await task.value
    }

    // MARK: - HH.ru API

    func getHHAuthUrl() async throws -> HHAuthUrlResponse {
        return try await request(path: "/api/hh/auth-url", requiresAuth: true)
    }

    func connectHHAccount(request: HHConnectRequest) async throws -> HHConnectResponse {
        let body = try JSONEncoder().encode(request)
        return try await request(path: "/api/hh/connect", method: "POST", body: body)
    }

    func getHHStatus() async throws -> HHStatusResponse {
        return try await request(path: "/api/hh/status")
    }

    func disconnectHHAccount() async throws {
        let _: EmptyResponse = try await request(path: "/api/hh/disconnect", method: "POST")
    }

    func getHHResumes() async throws -> [HHResumeResponse] {
        return try await request(path: "/api/hh/resumes")
    }

    // MARK: - Автоматизация

    func getAutomationStatus() async throws -> AutomationStatusResponse {
        return try await request(path: "/api/automation/status")
    }

    func startAutomation(request: StartAutomationRequest) async throws -> AutomationResponse {
        let body = try JSONEncoder().encode(request)
        return try await request(path: "/api/automation/start", method: "POST", body: body)
    }

    func stopAutomation() async throws -> EmptyResponse {
        return try await request(path: "/api/automation/stop", method: "POST")
    }

    func runAutomationNow() async throws -> AutomationNowResponse {
        return try await request(path: "/api/automation/run-now", method: "POST")
    }

    func updateAutomationSettings(request: UpdateSettingsRequest) async throws -> EmptyResponse {
        let body = try JSONEncoder().encode(request)
        return try await request(path: "/api/automation/settings", method: "PUT", body: body)
    }

    func updateAutomationSchedule(request: UpdateScheduleRequest) async throws -> EmptyResponse {
        let body = try JSONEncoder().encode(request)
        return try await request(path: "/api/automation/schedule", method: "PUT", body: body)
    }

    func getApplications(page: Int = 1, limit: Int = 20, status: String? = nil) async throws -> ApplicationsResponse {
        var path = "/api/automation/applications?page=\(page)&limit=\(limit)"
        if let status = status {
            path += "&status=\(status)"
        }
        return try await request(path: path)
    }

    func getInvitations() async throws -> [InvitationResponse] {
        return try await request(path: "/api/automation/invitations")
    }

    // MARK: - Аутентификация

    func register(request: RegisterRequest) async throws -> AuthResponse {
        let body = try JSONEncoder().encode(request)
        return try await request(path: "/api/auth/register", method: "POST", body: body, requiresAuth: false)
    }

    func login(request: LoginRequest) async throws -> AuthResponse {
        let body = try JSONEncoder().encode(request)
        return try await request(path: "/api/auth/login", method: "POST", body: body, requiresAuth: false)
    }

    func logout() async throws -> EmptyResponse {
        return try await request(path: "/api/auth/logout", method: "POST")
    }
}

// MARK: - Модели данных

struct EmptyResponse: Decodable {}

struct AuthTokens: Codable {
    let accessToken: String
    let refreshToken: String
    let expiresAt: Date
}

struct AuthResponse: Codable {
    let accessToken: String
    let refreshToken: String
    let expiresAt: String
    let user: UserResponse
}

struct UserResponse: Codable {
    let id: String
    let email: String
    let firstName: String
    let lastName: String
}

struct StartAutomationRequest: Codable {
    let schedule: ScheduleRequest
    let settings: SettingsRequest
}

struct ScheduleRequest: Codable {
    let enabled: Bool
    let frequency: String
    let timeOfDay: String
    let daysOfWeek: [Int]
}

struct SettingsRequest: Codable {
    let positions: [String]
    let salaryMin: Int
    let salaryMax: Int
    let locations: [String]
    let experience: String
    let employment: String
    let schedule: String
}

struct UpdateSettingsRequest: Codable {
    let positions: [String]
    let salaryMin: Int
    let salaryMax: Int
    let location: String
}

struct UpdateScheduleRequest: Codable {
    let timeOfDay: String
    let daysOfWeek: [Int]
}

struct AutomationStatusResponse: Codable {
    let status: String
    let schedule: ScheduleResponse
    let settings: SettingsResponse
    let stats: StatsResponse
    let lastRun: String?
    let nextRun: String?
}

struct ScheduleResponse: Codable {
    let timeOfDay: String
    let daysOfWeek: [Int]
}

struct SettingsResponse: Codable {
    let positions: [String]
    let salaryMin: Int
    let salaryMax: Int
    let location: String
}

struct StatsResponse: Codable {
    let vacanciesFound: Int
    let applicationsSent: Int
    let invitationsReceived: Int
    let matchRate: Double
}

struct AutomationNowResponse: Codable {
    let vacanciesFound: Int
    let applicationsSent: Int
    let timestamp: String
}

struct ApplicationsResponse: Codable {
    let applications: [ApplicationResponse]
    let total: Int
    let page: Int
    let limit: Int
    let pages: Int
}

struct ApplicationResponse: Codable, Identifiable {
    let id: String
    let vacancyTitle: String
    let companyName: String
    let status: String
    let matchScore: Double
    let appliedAt: String
    let automated: Bool
}

struct InvitationResponse: Codable, Identifiable {
    let id: String
    let vacancyTitle: String
    let companyName: String
    let receivedAt: String
    let status: String
}

enum APIError: Error {
    case invalidURL
    case invalidResponse
    case httpError(statusCode: Int)
    case unauthorized
    case decodingError
    case networkError
}

// MARK: - Конфигурация

enum Config {
    #if DEBUG
    static let baseURL = "http://localhost:8080"
    #else
    static let baseURL = "https://api.autojobsearch.com"
    #endif
}