import SwiftUI

struct AutomationView: View {
    @EnvironmentObject var authViewModel: AuthViewModel
    @EnvironmentObject var hhViewModel: HHConnectViewModel
    @EnvironmentObject var automationViewModel: AutomationViewModel
    @State private var showingHHConnect = false

    var body: some View {
        NavigationView {
            Group {
                if hhViewModel.isConnected {
                    AutomationContentView()
                } else {
                    HHConnectRequiredView(showingHHConnect: $showingHHConnect)
                }
            }
            .navigationTitle("Автоматизация")
            .navigationBarTitleDisplayMode(.large)
            .toolbar {
                ToolbarItem(placement: .navigationBarTrailing) {
                    Button(action: {
                        // Настройки
                    }) {
                        Image(systemName: "gear")
                    }
                }
            }
        }
        .sheet(isPresented: $showingHHConnect) {
            HHConnectView()
            .environmentObject(hhViewModel)
        }
        .onAppear {
            Task {
                await automationViewModel.loadAutomationState()
            }
        }
    }
}

struct AutomationContentView: View {
    @EnvironmentObject var automationViewModel: AutomationViewModel

    var body: some View {
        ScrollView {
            VStack(spacing: 20) {
                // Статус автоматизации
                AutomationStatusCard(
                    isActive: automationViewModel.isActive,
                    lastRun: automationViewModel.lastRun,
                    nextRun: automationViewModel.nextRun,
                    onToggle: { enabled in
                        Task {
                            if enabled {
                                await automationViewModel.startAutomation()
                            } else {
                                await automationViewModel.stopAutomation()
                            }
                        }
                    }
                )

                // Статистика
                StatisticsCard(
                    vacanciesFound: automationViewModel.vacanciesFound,
                    applicationsSent: automationViewModel.applicationsSent,
                    invitationsReceived: automationViewModel.invitationsReceived,
                    matchRate: automationViewModel.matchRate
                )

                // Быстрый запуск
                QuickRunCard(
                    isLoading: automationViewModel.isLoading,
                    isActive: automationViewModel.isActive,
                    onRunNow: {
                        Task {
                            await automationViewModel.runAutomationNow()
                        }
                    }
                )

                // Настройки поиска
                SearchSettingsCard(
                    positions: automationViewModel.positions,
                    salaryMin: automationViewModel.salaryMin,
                    salaryMax: automationViewModel.salaryMax,
                    location: automationViewModel.location,
                    onSave: {
                        Task {
                            await automationViewModel.saveSettings()
                        }
                    }
                )

                // Расписание
                ScheduleCard(
                    timeOfDay: automationViewModel.timeOfDay,
                    selectedDays: automationViewModel.selectedDays,
                    onSave: {
                        Task {
                            await automationViewModel.saveSchedule()
                        }
                    }
                )

                // Быстрые действия
                QuickActionsView()
            }
            .padding()
        }
    }
}

struct HHConnectRequiredView: View {
    @Binding var showingHHConnect: Bool

    var body: some View {
        VStack(spacing: 24) {
            Spacer()

            Image(systemName: "link.badge.plus")
            .font(.system(size: 80))
            .foregroundColor(.blue)
            .padding()

            VStack(spacing: 12) {
                Text("Требуется подключение HH.ru")
                .font(.title2)
                .fontWeight(.bold)

                Text("Для работы автоматического поиска и откликов необходимо подключить ваш аккаунт HH.ru")
                .font(.body)
                .foregroundColor(.secondary)
                .multilineTextAlignment(.center)
                .padding(.horizontal)
            }

            VStack(spacing: 16) {
                FeatureRow(
                    icon: "bolt.fill",
                    text: "Автоотклики от вашего имени"
                )

                FeatureRow(
                    icon: "lock.fill",
                    text: "Безопасное подключение"
                )

                FeatureRow(
                    icon: "arrow.triangle.2.circlepath",
                    text: "Синхронизация резюме"
                )
            }
            .padding(.vertical)

            Button(action: { showingHHConnect = true }) {
                HStack {
                    Image(systemName: "link")
                    Text("Подключить HH.ru")
                }
                .frame(maxWidth: .infinity)
            }
            .buttonStyle(.borderedProminent)
            .padding(.horizontal)

            Spacer()
        }
        .padding()
    }
}

struct FeatureRow: View {
    let icon: String
    let text: String

    var body: some View {
        HStack(spacing: 12) {
            Image(systemName: icon)
            .foregroundColor(.blue)
            .frame(width: 24)

            Text(text)
            .font(.subheadline)

            Spacer()
        }
        .padding(.horizontal)
    }
}

// MARK: - Компоненты автоматизации

struct AutomationStatusCard: View {
    let isActive: Bool
    let lastRun: String?
    let nextRun: String?
    let onToggle: (Bool) -> Void

    var body: some View {
        CardView {
            VStack(alignment: .leading, spacing: 12) {
                HStack {
                    VStack(alignment: .leading, spacing: 4) {
                        Text(isActive ? "Автоматизация активна" : "Автоматизация отключена")
                        .font(.headline)
                        .foregroundColor(isActive ? .green : .gray)

                        if let lastRun = lastRun {
                            Text("Последний запуск: \(lastRun)")
                            .font(.caption)
                            .foregroundColor(.secondary)
                        }

                        if isActive, let nextRun = nextRun {
                            Text("Следующий запуск: \(nextRun)")
                            .font(.caption)
                            .foregroundColor(.secondary)
                        }
                    }

                    Spacer()

                    Toggle("", isOn: Binding(
                        get: { isActive },
                        set: { onToggle($0) }
                    ))
                    .labelsHidden()
                }

                if isActive {
                    ProgressView()
                    .progressViewStyle(LinearProgressViewStyle())
                }
            }
        }
    }
}

struct StatisticsCard: View {
    let vacanciesFound: Int
    let applicationsSent: Int
    let invitationsReceived: Int
    let matchRate: Double

    var body: some View {
        CardView {
            VStack(alignment: .leading, spacing: 16) {
                Text("Статистика")
                .font(.headline)

                HStack(spacing: 12) {
                    StatItem(value: "\(vacanciesFound)", label: "Вакансий")
                    StatItem(value: "\(applicationsSent)", label: "Откликов")
                    StatItem(value: "\(invitationsReceived)", label: "Приглашений")
                    StatItem(value: "\(Int(matchRate * 100))%", label: "Совпадение")
                }
            }
        }
    }
}

struct StatItem: View {
    let value: String
    let label: String

    var body: some View {
        VStack(spacing: 4) {
            Text(value)
            .font(.title3)
            .fontWeight(.bold)
            .foregroundColor(.blue)

            Text(label)
            .font(.caption)
            .foregroundColor(.secondary)
            .multilineTextAlignment(.center)
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, 8)
        .background(Color(.systemGray6))
        .cornerRadius(8)
    }
}

struct QuickRunCard: View {
    let isLoading: Bool
    let isActive: Bool
    let onRunNow: () -> Void

    var body: some View {
        CardView(backgroundColor: Color(.systemGray6)) {
            VStack(spacing: 12) {
                Text("Запустить поиск сейчас")
                .font(.headline)

                Text("Немедленный запуск автоматического поиска и откликов")
                .font(.caption)
                .foregroundColor(.secondary)
                .multilineTextAlignment(.center)

                Button(action: onRunNow) {
                    if isLoading {
                        ProgressView()
                        .progressViewStyle(CircularProgressViewStyle())
                    } else {
                        HStack {
                            Image(systemName: "play.fill")
                            Text("Запустить сейчас")
                        }
                    }
                }
                .buttonStyle(.borderedProminent)
                .disabled(!isActive || isLoading)
                .frame(maxWidth: .infinity)
            }
        }
    }
}

struct SearchSettingsCard: View {
    @State private var positions = ""
    @State private var salaryMin = ""
    @State private var salaryMax = ""
    @State private var location = ""
    let onSave: () -> Void

    var body: some View {
        CardView {
            VStack(alignment: .leading, spacing: 16) {
                Text("Настройки поиска")
                .font(.headline)

                VStack(spacing: 12) {
                    TextField("Желаемые должности", text: $positions)
                    .textFieldStyle(RoundedBorderTextFieldStyle())
                    .placeholder(when: positions.isEmpty) {
                        Text("Android Developer, Kotlin Developer")
                        .foregroundColor(.gray)
                    }

                    HStack(spacing: 12) {
                        TextField("От", text: $salaryMin)
                        .textFieldStyle(RoundedBorderTextFieldStyle())
                        .keyboardType(.numberPad)
                        .placeholder(when: salaryMin.isEmpty) {
                            Text("50000")
                            .foregroundColor(.gray)
                        }

                        TextField("До", text: $salaryMax)
                        .textFieldStyle(RoundedBorderTextFieldStyle())
                        .keyboardType(.numberPad)
                        .placeholder(when: salaryMax.isEmpty) {
                            Text("200000")
                            .foregroundColor(.gray)
                        }
                    }

                    TextField("Город", text: $location)
                    .textFieldStyle(RoundedBorderTextFieldStyle())
                    .placeholder(when: location.isEmpty) {
                        Text("Москва, удаленно")
                        .foregroundColor(.gray)
                    }

                    Button("Сохранить настройки", action: onSave)
                    .buttonStyle(.bordered)
                    .frame(maxWidth: .infinity)
                }
            }
        }
    }
}

struct ScheduleCard: View {
    @State private var timeOfDay = "08:00"
    @State private var selectedDays: Set<Int> = [1, 2, 3, 4, 5]
    let onSave: () -> Void

    let days = ["Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"]

    var body: some View {
        CardView {
            VStack(alignment: .leading, spacing: 16) {
                Text("Расписание")
                .font(.headline)

                VStack(spacing: 12) {
                    TextField("Время запуска", text: $timeOfDay)
                    .textFieldStyle(RoundedBorderTextFieldStyle())
                    .placeholder(when: timeOfDay.isEmpty) {
                        Text("08:00")
                        .foregroundColor(.gray)
                    }

                    HStack(spacing: 8) {
                        ForEach(0..<days.count, id: \.self) { index in
                            let day = days[index]
                            Button(action: {
                                if selectedDays.contains(index + 1) {
                                    selectedDays.remove(index + 1)
                                } else {
                                    selectedDays.insert(index + 1)
                                }
                            }) {
                                Text(day)
                                .font(.caption)
                                .padding(.horizontal, 12)
                                .padding(.vertical, 6)
                                .background(selectedDays.contains(index + 1) ? Color.blue : Color(.systemGray5))
                                .foregroundColor(selectedDays.contains(index + 1) ? .white : .primary)
                                .cornerRadius(6)
                            }
                            .buttonStyle(PlainButtonStyle())
                        }
                    }

                    Button("Сохранить расписание", action: onSave)
                    .buttonStyle(.bordered)
                    .frame(maxWidth: .infinity)
                }
            }
        }
    }
}

struct QuickActionsView: View {
    var body: some View {
        HStack(spacing: 12) {
            QuickActionButton(
                icon: "doc.text.fill",
                title: "Отклики",
                action: {}
            )

            QuickActionButton(
                icon: "envelope.fill",
                title: "Приглашения",
                action: {}
            )

            QuickActionButton(
                icon: "chart.bar.fill",
                title: "Статистика",
                action: {}
            )
        }
    }
}

struct QuickActionButton: View {
    let icon: String
    let title: String
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            VStack(spacing: 8) {
                Image(systemName: icon)
                .font(.title2)
                .foregroundColor(.blue)

                Text(title)
                .font(.caption)
                .foregroundColor(.primary)
            }
            .frame(maxWidth: .infinity)
            .padding(.vertical, 12)
            .background(Color(.systemBackground))
            .cornerRadius(12)
            .shadow(color: .black.opacity(0.05), radius: 5, x: 0, y: 2)
        }
        .buttonStyle(PlainButtonStyle())
    }
}

// MARK: - Вспомогательные extension

extension View {
    func placeholder<Content: View>(
    when shouldShow: Bool,
    alignment: Alignment = .leading,
    @ViewBuilder placeholder: () -> Content
    ) -> some View {
        ZStack(alignment: alignment) {
            placeholder().opacity(shouldShow ? 1 : 0)
            self
        }
    }
}