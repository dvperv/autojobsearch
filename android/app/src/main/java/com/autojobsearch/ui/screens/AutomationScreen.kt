package com.autojobsearch.ui.screens

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavController
import com.autojobsearch.ui.viewmodels.AutomationViewModel
import com.autojobsearch.ui.viewmodels.HHConnectViewModel
import kotlinx.coroutines.launch

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun AutomationScreen(navController: NavController) {
    val automationViewModel: AutomationViewModel = hiltViewModel()
    val hhViewModel: HHConnectViewModel = hiltViewModel()
    val scope = rememberCoroutineScope()

    val automationState by automationViewModel.automationState.collectAsStateWithLifecycle()
    val isLoading by automationViewModel.isLoading.collectAsStateWithLifecycle()
    val hhStatus by hhViewModel.hhStatus.collectAsStateWithLifecycle()

    // Проверяем статус HH.ru при открытии экрана
    LaunchedEffect(Unit) {
        hhViewModel.checkHHStatus()
    }

    // Если HH.ru не подключен, показываем уведомление
    if (!hhStatus.connected) {
        Card(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            shape = RoundedCornerShape(12.dp),
            colors = CardDefaults.cardColors(
                containerColor = Color(0xFFFFF3E0)
            ),
            onClick = { navController.navigate("hh_connect") }
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(16.dp),
                horizontalArrangement = Arrangement.spacedBy(12.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Icon(
                    imageVector = Icons.Default.Warning,
                    contentDescription = "Внимание",
                    tint = Color(0xFFF57C00)
                )

                Column {
                    Text(
                        text = "Требуется подключение HH.ru",
                        fontSize = 16.sp,
                        fontWeight = FontWeight.Bold,
                        color = Color(0xFFF57C00)
                    )
                    Text(
                        text = "Для работы автоматизации необходимо подключить ваш аккаунт HH.ru",
                        fontSize = 14.sp,
                        color = Color(0xFF757575)
                    )
                }

                Spacer(modifier = Modifier.weight(1f))

                Icon(
                    imageVector = Icons.Default.ArrowForward,
                    contentDescription = "Перейти",
                    tint = Color(0xFFF57C00)
                )
            }
        }
    }

    Scaffold(
        topBar = {
            CenterAlignedTopAppBar(
                title = { Text("Автоматизация") },
                navigationIcon = {
                    IconButton(onClick = { navController.popBackStack() }) {
                        Icon(Icons.Default.ArrowBack, contentDescription = "Назад")
                    }
                },
                actions = {
                    IconButton(onClick = { navController.navigate("settings") }) {
                        Icon(Icons.Default.Settings, contentDescription = "Настройки")
                    }
                }
            )
        }
    ) { paddingValues ->
        LazyColumn(
            modifier = Modifier
                .fillMaxSize()
                .padding(paddingValues)
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            // Показываем остальной интерфейс только если HH.ru подключен
            if (hhStatus.connected) {
                item {
                    AutomationStatusCard(
                        isActive = automationState?.isActive ?: false,
                        lastRun = automationState?.lastRun,
                        nextRun = automationState?.nextRun,
                        onToggle = { enabled ->
                            scope.launch {
                                if (enabled) {
                                    automationViewModel.startAutomation()
                                } else {
                                    automationViewModel.stopAutomation()
                                }
                            }
                        },
                        isHHConnected = hhStatus.connected
                    )
                }

                // Остальные элементы экрана...
            }
        }
    }
}

@Composable
fun AutomationStatusCard(
    isActive: Boolean,
    lastRun: String?,
    nextRun: String?,
    onToggle: (Boolean) -> Unit,
    isHHConnected: Boolean
) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        colors = CardDefaults.cardColors(
            containerColor = if (isActive && isHHConnected)
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
                        text = if (isActive && isHHConnected)
                            "Автоматизация активна" else "Автоматизация отключена",
                        fontSize = 18.sp,
                        fontWeight = FontWeight.Bold,
                        color = if (isActive && isHHConnected)
                            Color(0xFF2E7D32) else Color(0xFF616161)
                    )

                    if (!isHHConnected) {
                        Text(
                            text = "Требуется подключение HH.ru",
                            fontSize = 14.sp,
                            color = Color(0xFFD32F2F)
                        )
                    } else if (lastRun != null) {
                        Text(
                            text = "Последний запуск: $lastRun",
                            fontSize = 14.sp,
                            color = Color(0xFF757575)
                        )
                    }

                    if (nextRun != null && isActive && isHHConnected) {
                        Text(
                            text = "Следующий запуск: $nextRun",
                            fontSize = 14.sp,
                            color = Color(0xFF757575)
                        )
                    }
                }

                Switch(
                    checked = isActive && isHHConnected,
                    onCheckedChange = { if (isHHConnected) onToggle(it) },
                    enabled = isHHConnected
                )
            }

            if (isActive && isHHConnected) {
                LinearProgressIndicator(
                    modifier = Modifier.fillMaxWidth(),
                    color = Color(0xFF4CAF50)
                )
            }
        }
    }
}