package com.autojobsearch.ui.viewmodels

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.autojobsearch.data.models.*
import com.autojobsearch.data.repositories.AutomationRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

@HiltViewModel
class AutomationViewModel @Inject constructor(
    private val repository: AutomationRepository
) : ViewModel() {

    private val _automationState = MutableStateFlow<AutomationState?>(null)
    val automationState: StateFlow<AutomationState?> = _automationState.asStateFlow()

    private val _isLoading = MutableStateFlow(false)
    val isLoading: StateFlow<Boolean> = _isLoading.asStateFlow()

    private val _errorMessage = MutableStateFlow<String?>(null)
    val errorMessage: StateFlow<String?> = _errorMessage.asStateFlow()

    init {
        loadAutomationState()
    }

    fun loadAutomationState() {
        viewModelScope.launch {
            _isLoading.value = true
            _errorMessage.value = null

            try {
                val state = repository.getAutomationStatus()
                _automationState.value = state
            } catch (e: Exception) {
                _errorMessage.value = "Failed to load automation state: ${e.message}"
            } finally {
                _isLoading.value = false
            }
        }
    }

    fun startAutomation() {
        viewModelScope.launch {
            _isLoading.value = true
            _errorMessage.value = null

            try {
                val result = repository.startAutomation()
                _automationState.value = result
            } catch (e: Exception) {
                _errorMessage.value = "Failed to start automation: ${e.message}"
            } finally {
                _isLoading.value = false
            }
        }
    }

    fun stopAutomation() {
        viewModelScope.launch {
            _isLoading.value = true
            _errorMessage.value = null

            try {
                repository.stopAutomation()
                _automationState.value = _automationState.value?.copy(isActive = false)
            } catch (e: Exception) {
                _errorMessage.value = "Failed to stop automation: ${e.message}"
            } finally {
                _isLoading.value = false
            }
        }
    }

    fun runAutomationNow() {
        viewModelScope.launch {
            _isLoading.value = true
            _errorMessage.value = null

            try {
                val result = repository.runAutomationNow()
                // Обновляем статистику
                _automationState.value = _automationState.value?.copy(
                    vacanciesFound = (vacanciesFound ?: 0) + result.vacanciesFound,
                    applicationsSent = (applicationsSent ?: 0) + result.applicationsSent,
                    lastRun = result.timestamp
                )
            } catch (e: Exception) {
                _errorMessage.value = "Failed to run automation: ${e.message}"
            } finally {
                _isLoading.value = false
            }
        }
    }

    fun updateSettings(
        positions: List<String>,
        salaryMin: Int,
        salaryMax: Int,
        location: String
    ) {
        viewModelScope.launch {
            _isLoading.value = true
            _errorMessage.value = null

            try {
                repository.updateSettings(
                    positions = positions,
                    salaryMin = salaryMin,
                    salaryMax = salaryMax,
                    location = location
                )

                _automationState.value = _automationState.value?.copy(
                    positions = positions,
                    salaryMin = salaryMin,
                    salaryMax = salaryMax,
                    location = location
                )
            } catch (e: Exception) {
                _errorMessage.value = "Failed to update settings: ${e.message}"
            } finally {
                _isLoading.value = false
            }
        }
    }

    fun updateSchedule(
        timeOfDay: String,
        daysOfWeek: List<Int>
    ) {
        viewModelScope.launch {
            _isLoading.value = true
            _errorMessage.value = null

            try {
                repository.updateSchedule(
                    timeOfDay = timeOfDay,
                    daysOfWeek = daysOfWeek
                )

                _automationState.value = _automationState.value?.copy(
                    timeOfDay = timeOfDay,
                    daysOfWeek = daysOfWeek
                )
            } catch (e: Exception) {
                _errorMessage.value = "Failed to update schedule: ${e.message}"
            } finally {
                _isLoading.value = false
            }
        }
    }

    fun clearError() {
        _errorMessage.value = null
    }
}

data class AutomationState(
    val isActive: Boolean = false,
    val lastRun: String? = null,
    val nextRun: String? = null,
    val vacanciesFound: Int = 0,
    val applicationsSent: Int = 0,
    val invitationsReceived: Int = 0,
    val matchRate: Double = 0.0,
    val positions: List<String> = emptyList(),
    val salaryMin: Int = 0,
    val salaryMax: Int = 0,
    val location: String = "",
    val timeOfDay: String = "08:00",
    val daysOfWeek: List<Int> = listOf(1, 2, 3, 4, 5)
)

data class AutomationResult(
    val vacanciesFound: Int,
    val applicationsSent: Int,
    val timestamp: String
)