package com.autojobsearch.ui.viewmodels

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.autojobsearch.data.models.HHStatusResponse
import com.autojobsearch.data.repositories.HHRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

@HiltViewModel
class HHConnectViewModel @Inject constructor(
    private val repository: HHRepository,
    private val authRepository: AuthRepository
) : ViewModel() {

    private val _authState = MutableStateFlow(HHAuthState())
    val authState: StateFlow<HHAuthState> = _authState.asStateFlow()

    private val _hhStatus = MutableStateFlow(HHStatus())
    val hhStatus: StateFlow<HHStatus> = _hhStatus.asStateFlow()

    private val _isLoading = MutableStateFlow(false)
    val isLoading: StateFlow<Boolean> = _isLoading.asStateFlow()

    private val _errorMessage = MutableStateFlow<String?>(null)
    val errorMessage: StateFlow<String?> = _errorMessage.asStateFlow()

    fun getAuthUrl() {
        viewModelScope.launch {
            _isLoading.value = true
            _errorMessage.value = null

            try {
                val response = repository.getHHAuthUrl()
                _authState.value = _authState.value.copy(
                    authUrl = response.authUrl,
                    state = response.state
                )
            } catch (e: Exception) {
                _errorMessage.value = "Не удалось получить URL авторизации: ${e.message}"
            } finally {
                _isLoading.value = false
            }
        }
    }

    fun exchangeCode(code: String, state: String) {
        viewModelScope.launch {
            _isLoading.value = true
            _errorMessage.value = null

            try {
                val response = repository.exchangeHHAuthCode(code, state)
                _authState.value = _authState.value.copy(
                    isConnected = true,
                    userInfo = response.userInfo,
                    expiresAt = response.tokensExpireAt
                )

                // Обновляем статус
                checkHHStatus()
            } catch (e: Exception) {
                _errorMessage.value = "Не удалось подключить HH.ru: ${e.message}"
            } finally {
                _isLoading.value = false
            }
        }
    }

    fun checkHHStatus() {
        viewModelScope.launch {
            _isLoading.value = true
            _errorMessage.value = null

            try {
                val status = repository.getHHStatus()
                _hhStatus.value = HHStatus.fromResponse(status)
            } catch (e: Exception) {
                _errorMessage.value = "Не удалось проверить статус HH.ru"
            } finally {
                _isLoading.value = false
            }
        }
    }

    fun disconnectHH() {
        viewModelScope.launch {
            _isLoading.value = true
            _errorMessage.value = null

            try {
                repository.disconnectHH()
                _authState.value = HHAuthState()
                _hhStatus.value = HHStatus()
            } catch (e: Exception) {
                _errorMessage.value = "Не удалось отключить HH.ru: ${e.message}"
            } finally {
                _isLoading.value = false
            }
        }
    }

    fun clearError() {
        _errorMessage.value = null
    }
}

data class HHAuthState(
    val authUrl: String? = null,
    val state: String? = null,
    val isConnected: Boolean = false,
    val userInfo: HHUserInfo? = null,
    val expiresAt: String? = null
)

data class HHStatus(
    val connected: Boolean = false,
    val expiresAt: String? = null,
    val isExpired: Boolean = false,
    val userInfo: HHUserInfo? = null,
    val minutesLeft: Int = 0
) {
    companion object {
        fun fromResponse(response: HHStatusResponse): HHStatus {
            return HHStatus(
                connected = response.connected,
                expiresAt = response.expiresAt,
                isExpired = response.isExpired,
                userInfo = response.userInfo,
                minutesLeft = response.minutesLeft ?: 0
            )
        }
    }
}

data class HHUserInfo(
    val firstName: String? = null,
    val lastName: String? = null,
    val email: String? = null,
    val resumesCount: Int = 0
)