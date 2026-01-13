package com.autojobsearch

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.autojobsearch.ui.screens.*
import com.autojobsearch.ui.theme.AutoJobSearchTheme
import com.autojobsearch.ui.viewmodels.AuthViewModel
import dagger.hilt.android.AndroidEntryPoint

@AndroidEntryPoint
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        setContent {
            AutoJobSearchTheme {
                Surface(
                    modifier = Modifier.fillMaxSize(),
                    color = MaterialTheme.colorScheme.background
                ) {
                    AutoJobSearchApp()
                }
            }
        }
    }
}

@Composable
fun AutoJobSearchApp() {
    val navController = rememberNavController()
    val authViewModel: AuthViewModel = hiltViewModel()
    val authState by authViewModel.authState.collectAsStateWithLifecycle()

    LaunchedEffect(authState.isAuthenticated) {
        if (authState.isAuthenticated) {
            navController.navigate("main") {
                popUpTo("splash") { inclusive = true }
            }
        } else if (!authState.isLoading) {
            navController.navigate("welcome") {
                popUpTo("splash") { inclusive = true }
            }
        }
    }

    NavHost(
        navController = navController,
        startDestination = "splash"
    ) {
        composable("splash") {
            SplashScreen()
        }

        composable("welcome") {
            WelcomeScreen(navController)
        }

        composable("login") {
            LoginScreen(navController)
        }

        composable("register") {
            RegisterScreen(navController)
        }

        composable("main") {
            MainScreen(navController)
        }

        composable("hh_connect") {
            HHConnectScreen(navController)
        }

        composable("hh_auth_webview/{authUrl}") { backStackEntry ->
            val authUrl = backStackEntry.arguments?.getString("authUrl") ?: ""
            HHAuthWebViewScreen(navController, authUrl)
        }

        composable("automation") {
            AutomationScreen(navController)
        }

        composable("resume_upload") {
            ResumeUploadScreen(navController)
        }

        composable("search_settings") {
            SearchSettingsScreen(navController)
        }

        composable("applications") {
            ApplicationsScreen(navController)
        }

        composable("invitations") {
            InvitationsScreen(navController)
        }

        composable("profile") {
            ProfileScreen(navController)
        }

        composable("settings") {
            SettingsScreen(navController)
        }
    }
}