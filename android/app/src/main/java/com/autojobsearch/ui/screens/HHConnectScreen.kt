package com.autojobsearch.ui.screens

import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavController
import com.autojobsearch.ui.viewmodels.HHConnectViewModel
import kotlinx.coroutines.launch

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun HHConnectScreen(navController: NavController) {
    val viewModel: HHConnectViewModel = hiltViewModel()
    val scope = rememberCoroutineScope()
    val context = LocalContext.current

    val authState by viewModel.authState.collectAsStateWithLifecycle()
    val isLoading by viewModel.isLoading.collectAsStateWithLifecycle()
    val errorMessage by viewModel.errorMessage.collectAsStateWithLifecycle()
    val hhStatus by viewModel.hhStatus.collectAsStateWithLifecycle()

    var showWebView by remember { mutableStateOf(false) }
    var authUrl by remember { mutableStateOf("") }

    LaunchedEffect(Unit) {
        viewModel.checkHHStatus()
    }

    LaunchedEffect(authState.authUrl) {
        authState.authUrl?.let { url ->
            authUrl = url
            showWebView = true
        }
    }

    LaunchedEffect(authState.isConnected) {
        if (authState.isConnected) {
            navController.popBackStack()
        }
    }

    Scaffold(
        topBar = {
            CenterAlignedTopAppBar(
                title = { Text("Подключение HH.ru") },
                navigationIcon = {
                    IconButton(onClick = { navController.popBackStack() }) {
                        Icon(Icons.Default.ArrowBack, contentDescription = "Назад")
                    }
                }
            )
        }
    ) { paddingValues ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(paddingValues)
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(20.dp)
        ) {
            if (showWebView && authUrl.isNotEmpty()) {
                // WebView для OAuth авторизации HH.ru
                AndroidView(
                    factory = { context ->
                        WebView(context).apply {
                            webViewClient = object : WebViewClient() {
                                override fun shouldOverrideUrlLoading(
                                    view: WebView?,
                                    url: String
                                ): Boolean {
                                    // Ловим callback URL с кодом авторизации
                                    if (url.contains("code=") && url.contains("state=")) {
                                        val code = extractCodeFromUrl(url)
                                        val state = extractStateFromUrl(url)

                                        scope.launch {
                                            viewModel.exchangeCode(code, state)
                                            showWebView = false
                                        }
                                        return true
                                    }
                                    return false
                                }
                            }
                            loadUrl(authUrl)
                        }
                    },
                    modifier = Modifier.fillMaxSize()
                )
            } else {
                // Статус подключения
                Card(
                    modifier = Modifier.fillMaxWidth(),
                    shape = RoundedCornerShape(12.dp),
                    colors = CardDefaults.cardColors(
                        containerColor = if (hhStatus.connected)
                            Color(0xFFE8F5E9) else Color(0xFFF5F5F5)
                    )
                ) {
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(16.dp),
                        verticalArrangement = Arrangement.spacedBy(12.dp)
                    ) {
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Column {
                                Text(
                                    text = if (hhStatus.connected)
                                        "HH.ru подключен" else "HH.ru не подключен",
                                    fontSize = 18.sp,
                                    fontWeight = FontWeight.Bold,
                                    color = if (hhStatus.connected)
                                        Color(0xFF2E7D32) else Color(0xFF616161)
                                )

                                if (hhStatus.connected) {
                                    hhStatus.userInfo?.let { userInfo ->
                                        Text(
                                            text = "${userInfo.firstName} ${userInfo.lastName}",
                                            fontSize = 14.sp,
                                            color = Color(0xFF757575)
                                        )
                                    }

                                    hhStatus.expiresAt?.let { expiresAt ->
                                        Text(
                                            text = "Действителен до: $expiresAt",
                                            fontSize = 12.sp,
                                            color = Color(0xFF757575)
                                        )
                                    }
                                }
                            }

                            if (hhStatus.connected) {
                                Badge(
                                    containerColor = Color(0xFF4CAF50),
                                    content = { Text("✓") }
                                )
                            }
                        }
                    }
                }

                // Преимущества подключения
                Card(
                    modifier = Modifier.fillMaxWidth(),
                    shape = RoundedCornerShape(12.dp)
                ) {
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(16.dp),
                        verticalArrangement = Arrangement.spacedBy(12.dp)
                    ) {
                        Text(
                            text = "Преимущества подключения HH.ru",
                            fontSize = 16.sp,
                            fontWeight = FontWeight.Bold
                        )

                        FeatureItem(
                            icon = Icons.Default.AutoAwesome,
                            title = "Автоотклики от вашего имени",
                            description = "Отклики отправляются через ваш аккаунт HH.ru"
                        )

                        FeatureItem(
                            icon = Icons.Default.Security,
                            title = "Безопасность",
                            description = "Мы не храним ваш пароль от HH.ru"
                        )

                        FeatureItem(
                            icon = Icons.Default.Sync,
                            title = "Синхронизация резюме",
                            description = "Используем ваши резюме с HH.ru для откликов"
                        )

                        FeatureItem(
                            icon = Icons.Default.Notifications,
                            title = "Уведомления",
                            description = "Получайте уведомления о приглашениях на собеседования"
                        )
                    }
                }

                // Кнопка подключения/отключения
                Column(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(12.dp)
                ) {
                    if (hhStatus.connected) {
                        Button(
                            onClick = {
                                scope.launch {
                                    viewModel.disconnectHH()
                                }
                            },
                            modifier = Modifier.fillMaxWidth(),
                            colors = ButtonDefaults.buttonColors(
                                containerColor = Color(0xFFD32F2F)
                            ),
                            enabled = !isLoading
                        ) {
                            if (isLoading) {
                                CircularProgressIndicator(
                                    modifier = Modifier.size(20.dp),
                                    strokeWidth = 2.dp,
                                    color = Color.White
                                )
                            } else {
                                Icon(
                                    Icons.Default.LinkOff,
                                    contentDescription = "Отключить"
                                )
                                Spacer(modifier = Modifier.width(8.dp))
                                Text("Отключить HH.ru")
                            }
                        }
                    } else {
                        Button(
                            onClick = {
                                scope.launch {
                                    viewModel.getAuthUrl()
                                }
                            },
                            modifier = Modifier.fillMaxWidth(),
                            enabled = !isLoading
                        ) {
                            if (isLoading) {
                                CircularProgressIndicator(
                                    modifier = Modifier.size(20.dp),
                                    strokeWidth = 2.dp,
                                    color = Color.White
                                )
                            } else {
                                Icon(
                                    Icons.Default.Link,
                                    contentDescription = "Подключить"
                                )
                                Spacer(modifier = Modifier.width(8.dp))
                                Text("Подключить HH.ru")
                            }
                        }
                    }

                    Text(
                        text = "Подключив HH.ru, вы соглашаетесь с условиями использования",
                        fontSize = 12.sp,
                        color = Color(0xFF757575),
                        textAlign = TextAlign.Center
                    )
                }
            }
        }

        // Обработка ошибок
        errorMessage?.let { message ->
            LaunchedEffect(message) {
                // Показать Snackbar
            }
        }
    }
}

@Composable
fun FeatureItem(
    icon: ImageVector,
    title: String,
    description: String
) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Icon(
            imageVector = icon,
            contentDescription = title,
            tint = MaterialTheme.colorScheme.primary,
            modifier = Modifier.size(24.dp)
        )

        Column(
            verticalArrangement = Arrangement.spacedBy(4.dp)
        ) {
            Text(
                text = title,
                fontSize = 14.sp,
                fontWeight = FontWeight.Medium
            )
            Text(
                text = description,
                fontSize = 12.sp,
                color = Color(0xFF757575),
                lineHeight = 14.sp
            )
        }
    }
}

// Вспомогательные функции для извлечения параметров из URL
private fun extractCodeFromUrl(url: String): String {
    return url.substringAfter("code=").substringBefore("&")
}

private fun extractStateFromUrl(url: String): String {
    return url.substringAfter("state=").substringBefore("&")
}