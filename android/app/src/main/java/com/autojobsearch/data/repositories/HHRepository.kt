package com.autojobsearch.data.repositories

import com.autojobsearch.data.api.ApiService
import com.autojobsearch.data.models.*
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class HHRepository @Inject constructor(
    private val apiService: ApiService,
    private val authRepository: AuthRepository
) {

    suspend fun getHHAuthUrl(): HHAuthUrlResponse {
        return apiService.getHHAuthUrl()
    }

    suspend fun exchangeHHAuthCode(code: String, state: String): HHConnectResponse {
        val request = HHConnectRequest(
            authorizationCode = code,
            state = state
        )
        return apiService.connectHHAccount(request)
    }

    suspend fun getHHStatus(): HHStatusResponse {
        return apiService.getHHStatus()
    }

    suspend fun disconnectHH() {
        apiService.disconnectHHAccount()
    }

    suspend fun refreshHHTokens(): HHStatusResponse {
        // Это вызовет автоматическое обновление токенов на сервере
        return apiService.getHHStatus()
    }

    suspend fun getHHResumes(): List<HHResumeResponse> {
        return apiService.getHHResumes()
    }
}

data class HHAuthUrlResponse(
    val authUrl: String,
    val state: String
)

data class HHConnectRequest(
    val authorizationCode: String,
    val state: String
)

data class HHConnectResponse(
    val message: String,
    val userInfo: HHUserInfoResponse,
    val tokensExpireAt: String
)

data class HHStatusResponse(
    val connected: Boolean,
    val expiresAt: String?,
    val isExpired: Boolean,
    val userInfo: HHUserInfoResponse?,
    val minutesLeft: Int?
)

data class HHUserInfoResponse(
    val firstName: String?,
    val lastName: String?,
    val email: String?,
    val resumesCount: Int
)

data class HHResumeResponse(
    val id: String,
    val title: String,
    val createdAt: String,
    val updatedAt: String
)