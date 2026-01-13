package com.autojobsearch.data.repositories

import com.autojobsearch.data.api.ApiService
import com.autojobsearch.data.models.AutomationResponse
import com.autojobsearch.data.models.AutomationStatusResponse
import com.autojobsearch.data.models.StartAutomationRequest
import com.autojobsearch.data.models.UpdateSettingsRequest
import com.autojobsearch.ui.viewmodels.AutomationResult
import com.autojobsearch.ui.viewmodels.AutomationState
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class AutomationRepository @Inject constructor(
    private val apiService: ApiService,
    private val authRepository: AuthRepository
) {

    suspend fun getAutomationStatus(): AutomationState {
        val response = apiService.getAutomationStatus()

        return AutomationState(
            isActive = response.status == "active",
            lastRun = response.lastRun,
            nextRun = response.nextRun,
            vacanciesFound = response.stats.vacanciesFound,
            applicationsSent = response.stats.applicationsSent,
            invitationsReceived = response.stats.invitationsReceived,
            matchRate = response.stats.matchRate,
            positions = response.settings.positions,
            salaryMin = response.settings.salaryMin,
            salaryMax = response.settings.salaryMax,
            location = response.settings.location,
            timeOfDay = response.schedule.timeOfDay,
            daysOfWeek = response.schedule.daysOfWeek
        )
    }

    suspend fun startAutomation(): AutomationState {
        val request = StartAutomationRequest(
            schedule = StartAutomationRequest.Schedule(
                enabled = true,
                frequency = "daily",
                timeOfDay = "08:00",
                daysOfWeek = listOf(1, 2, 3, 4, 5)
            ),
            settings = StartAutomationRequest.Settings(
                positions = listOf("Android Developer", "Kotlin Developer"),
                salaryMin = 50000,
                salaryMax = 200000,
                locations = listOf("Москва", "удаленно"),
                experience = "between1And3",
                employment = "full",
                schedule = "fullDay"
            )
        )

        val response = apiService.startAutomation(request)

        return AutomationState(
            isActive = true,
            lastRun = response.lastRun,
            nextRun = response.nextRun,
            vacanciesFound = response.stats.vacanciesFound,
            applicationsSent = response.stats.applicationsSent,
            invitationsReceived = response.stats.invitationsReceived,
            matchRate = response.stats.matchRate,
            positions = response.settings.positions,
            salaryMin = response.settings.salaryMin,
            salaryMax = response.settings.salaryMax,
            location = response.settings.location,
            timeOfDay = response.schedule.timeOfDay,
            daysOfWeek = response.schedule.daysOfWeek
        )
    }

    suspend fun stopAutomation() {
        apiService.stopAutomation()
    }

    suspend fun runAutomationNow(): AutomationResult {
        val response = apiService.runAutomationNow()

        return AutomationResult(
            vacanciesFound = response.vacanciesFound,
            applicationsSent = response.applicationsSent,
            timestamp = response.timestamp
        )
    }

    suspend fun updateSettings(
        positions: List<String>,
        salaryMin: Int,
        salaryMax: Int,
        location: String
    ) {
        val request = UpdateSettingsRequest(
            positions = positions,
            salaryMin = salaryMin,
            salaryMax = salaryMax,
            location = location
        )

        apiService.updateAutomationSettings(request)
    }

    suspend fun updateSchedule(
        timeOfDay: String,
        daysOfWeek: List<Int>
    ) {
        val request = UpdateSettingsRequest.Schedule(
            timeOfDay = timeOfDay,
            daysOfWeek = daysOfWeek
        )

        apiService.updateAutomationSchedule(request)
    }

    suspend fun getApplications(page: Int = 1, limit: Int = 20): List<Application> {
        return apiService.getApplications(page, limit)
    }

    suspend fun getInvitations(): List<Invitation> {
        return apiService.getInvitations()
    }
}