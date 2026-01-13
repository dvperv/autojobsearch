package com.autojobsearch.data.api

import retrofit2.Response
import retrofit2.http.*

interface ApiService {

    // HH.ru авторизация
    @GET("api/hh/auth-url")
    suspend fun getHHAuthUrl(): HHAuthUrlResponse

    @POST("api/hh/connect")
    suspend fun connectHHAccount(
        @Body request: HHConnectRequest
    ): HHConnectResponse

    @GET("api/hh/status")
    suspend fun getHHStatus(): HHStatusResponse

    @POST("api/hh/disconnect")
    suspend fun disconnectHHAccount(): Response<Unit>

    @GET("api/hh/resumes")
    suspend fun getHHResumes(): List<HHResumeResponse>

    // Автоматизация
    @GET("api/automation/status")
    suspend fun getAutomationStatus(): AutomationStatusResponse

    @POST("api/automation/start")
    suspend fun startAutomation(
        @Body request: StartAutomationRequest
    ): AutomationResponse

    @POST("api/automation/stop")
    suspend fun stopAutomation(): Response<Unit>

    @POST("api/automation/run-now")
    suspend fun runAutomationNow(): AutomationNowResponse

    @PUT("api/automation/settings")
    suspend fun updateAutomationSettings(
        @Body request: UpdateSettingsRequest
    ): Response<Unit>

    @PUT("api/automation/schedule")
    suspend fun updateAutomationSchedule(
        @Body request: UpdateScheduleRequest
    ): Response<Unit>

    @GET("api/automation/applications")
    suspend fun getApplications(
        @Query("page") page: Int = 1,
        @Query("limit") limit: Int = 20,
        @Query("status") status: String? = null
    ): ApplicationsResponse

    @GET("api/automation/invitations")
    suspend fun getInvitations(): List<InvitationResponse>

    // Резюме
    @POST("api/resumes/upload")
    suspend fun uploadResume(
        @Body request: UploadResumeRequest
    ): ResumeResponse

    @GET("api/resumes")
    suspend fun getResumes(): List<ResumeResponse>

    @DELETE("api/resumes/{id}")
    suspend fun deleteResume(@Path("id") id: String): Response<Unit>

    // Аутентификация
    @POST("api/auth/register")
    suspend fun register(@Body request: RegisterRequest): AuthResponse

    @POST("api/auth/login")
    suspend fun login(@Body request: LoginRequest): AuthResponse

    @POST("api/auth/refresh")
    suspend fun refreshToken(@Body request: RefreshTokenRequest): AuthResponse

    @POST("api/auth/logout")
    suspend fun logout(): Response<Unit>
}

// Модели запросов
data class HHConnectRequest(
    val authorizationCode: String,
    val state: String
)

data class StartAutomationRequest(
    val schedule: ScheduleRequest,
    val settings: SettingsRequest
)

data class ScheduleRequest(
    val enabled: Boolean,
    val frequency: String,
    val timeOfDay: String,
    val daysOfWeek: List<Int>
)

data class SettingsRequest(
    val positions: List<String>,
    val salaryMin: Int,
    val salaryMax: Int,
    val locations: List<String>,
    val experience: String,
    val employment: String,
    val schedule: String
)

data class UpdateSettingsRequest(
    val positions: List<String>,
    val salaryMin: Int,
    val salaryMax: Int,
    val location: String
)

data class UpdateScheduleRequest(
    val timeOfDay: String,
    val daysOfWeek: List<Int>
)

data class UploadResumeRequest(
    val fileName: String,
    val fileContent: String, // base64
    val fileType: String
)

data class RegisterRequest(
    val email: String,
    val password: String,
    val firstName: String,
    val lastName: String
)

data class LoginRequest(
    val email: String,
    val password: String
)

data class RefreshTokenRequest(
    val refreshToken: String
)

// Модели ответов
data class HHAuthUrlResponse(
    val authUrl: String,
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