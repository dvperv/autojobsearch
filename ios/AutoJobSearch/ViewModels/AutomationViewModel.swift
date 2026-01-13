import Foundation
import Combine
import FirebaseMessaging

class AutomationViewModel: ObservableObject {
    @Published var automationState: AutomationState?
    @Published var isLoading = false
    @Published var errorMessage: String?

    private let apiService = APIService.shared
    private var cancellables = Set<AnyCancellable>()

    init() {
        loadAutomationState()
    }

    func loadAutomationState() {
        isLoading = true
        errorMessage = nil

        apiService.getAutomationStatus()
        .receive(on: DispatchQueue.main)
        .sink { [weak self] completion in
            self?.isLoading = false
            if case .failure(let error) = completion {
                self?.errorMessage = error.localizedDescription
            }
        } receiveValue: { [weak self] response in
            self?.automationState = AutomationState(from: response)
        }
        .store(in: &cancellables)
    }

    func startAutomation() {
        isLoading = true
        errorMessage = nil

        let request = StartAutomationRequest(
            schedule: StartAutomationRequest.Schedule(
                enabled: true,
                frequency: "daily",
                timeOfDay: "08:00",
                daysOfWeek: [1, 2, 3, 4, 5]
            ),
            settings: StartAutomationRequest.Settings(
                positions: ["Android Developer", "Kotlin Developer"],
                salaryMin: 50000,
                salaryMax: 200000,
                locations: ["Москва", "удаленно"],
                experience: "between1And3",
                employment: "full",
                schedule: "fullDay"
            )
        )

        apiService.startAutomation(request: request)
        .receive(on: DispatchQueue.main)
        .sink { [weak self] completion in
            self?.isLoading = false
            if case .failure(let error) = completion {
                self?.errorMessage = error.localizedDescription
            }
        } receiveValue: { [weak self] response in
            self?.automationState = AutomationState(from: response)
        }
        .store(in: &cancellables)
    }

    func stopAutomation() {
        isLoading = true
        errorMessage = nil

        apiService.stopAutomation()
        .receive(on: DispatchQueue.main)
        .sink { [weak self] completion in
            self?.isLoading = false
            if case .failure(let error) = completion {
                self?.errorMessage = error.localizedDescription
            }
        } receiveValue: { [weak self] in
            self?.automationState?.isActive = false
        }
        .store(in: &cancellables)
    }

    func runAutomationNow() {
        isLoading = true
        errorMessage = nil

        apiService.runAutomationNow()
        .receive(on: DispatchQueue.main)
        .sink { [weak self] completion in
            self?.isLoading = false
            if case .failure(let error) = completion {
                self?.errorMessage = error.localizedDescription
            }
        } receiveValue: { [weak self] result in
            self?.automationState?.vacanciesFound += result.vacanciesFound
            self?.automationState?.applicationsSent += result.applicationsSent
            self?.automationState?.lastRun = result.timestamp
        }
        .store(in: &cancellables)
    }

    func updatePositions(_ positions: [String]) {
        automationState?.positions = positions
    }

    func updateSalaryMin(_ salary: Int) {
        automationState?.salaryMin = salary
    }

    func updateSalaryMax(_ salary: Int) {
        automationState?.salaryMax = salary
    }

    func updateLocation(_ location: String) {
        automationState?.location = location
    }

    func saveSettings() {
        guard let state = automationState else { return }

        let request = UpdateSettingsRequest(
            positions: state.positions,
            salaryMin: state.salaryMin,
            salaryMax: state.salaryMax,
            location: state.location
        )

        apiService.updateAutomationSettings(request: request)
        .receive(on: DispatchQueue.main)
        .sink { [weak self] completion in
            if case .failure(let error) = completion {
                self?.errorMessage = error.localizedDescription
            }
        } receiveValue: { _ in }
        .store(in: &cancellables)
    }

    func updateTimeOfDay(_ time: String) {
        automationState?.timeOfDay = time
    }

    func updateDaysOfWeek(_ days: [Int]) {
        automationState?.daysOfWeek = days
    }

    func saveSchedule() {
        guard let state = automationState else { return }

        let request = UpdateScheduleRequest(
            timeOfDay: state.timeOfDay,
            daysOfWeek: state.daysOfWeek
        )

        apiService.updateAutomationSchedule(request: request)
        .receive(on: DispatchQueue.main)
        .sink { [weak self] completion in
            if case .failure(let error) = completion {
                self?.errorMessage = error.localizedDescription
            }
        } receiveValue: { _ in }
        .store(in: &cancellables)
    }

    func clearError() {
        errorMessage = nil
    }
}

// Модели данных
struct AutomationState {
    var isActive: Bool = false
    var lastRun: String?
    var nextRun: String?
    var vacanciesFound: Int = 0
    var applicationsSent: Int = 0
    var invitationsReceived: Int = 0
    var matchRate: Double = 0.0
    var positions: [String] = []
    var salaryMin: Int = 0
    var salaryMax: Int = 0
    var location: String = ""
    var timeOfDay: String = "08:00"
    var daysOfWeek: [Int] = [1, 2, 3, 4, 5]

    init(from response: AutomationStatusResponse) {
        self.isActive = response.status == "active"
        self.lastRun = response.lastRun
        self.nextRun = response.nextRun
        self.vacanciesFound = response.stats.vacanciesFound
        self.applicationsSent = response.stats.applicationsSent
        self.invitationsReceived = response.stats.invitationsReceived
        self.matchRate = response.stats.matchRate
        self.positions = response.settings.positions
        self.salaryMin = response.settings.salaryMin
        self.salaryMax = response.settings.salaryMax
        self.location = response.settings.location
        self.timeOfDay = response.schedule.timeOfDay
        self.daysOfWeek = response.schedule.daysOfWeek
    }
}

struct AutomationResult {
    let vacanciesFound: Int
    let applicationsSent: Int
    let timestamp: String
}