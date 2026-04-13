import 'package:go_router/go_router.dart';

import 'screens/dashboard_screen.dart';
import 'screens/flags/flag_list_screen.dart';
import 'screens/flags/flag_detail_screen.dart';
import 'screens/flags/flag_create_screen.dart';
import 'screens/deployments_screen.dart';
import 'screens/releases_screen.dart';
import 'screens/analytics_screen.dart';
import 'screens/settings_screen.dart';
import 'screens/login_screen.dart';
import 'screens/profile_screen.dart';
import 'widgets/app_shell.dart';

final GoRouter appRouter = GoRouter(
  initialLocation: '/dashboard',
  routes: [
    // Login route (no shell)
    GoRoute(
      path: '/login',
      name: 'login',
      builder: (context, state) => const LoginScreen(),
    ),

    // Main app routes with shell
    ShellRoute(
      builder: (context, state, child) => AppShell(child: child),
      routes: [
        GoRoute(
          path: '/dashboard',
          name: 'dashboard',
          builder: (context, state) => const DashboardScreen(),
        ),
        GoRoute(
          path: '/flags',
          name: 'flags',
          builder: (context, state) => const FlagListScreen(),
          routes: [
            GoRoute(
              path: '/new',
              name: 'flag-create',
              builder: (context, state) => const FlagCreateScreen(),
            ),
            GoRoute(
              path: '/:flagId',
              name: 'flag-detail',
              builder: (context, state) {
                final flagId = state.pathParameters['flagId']!;
                return FlagDetailScreen(flagId: flagId);
              },
            ),
          ],
        ),
        GoRoute(
          path: '/deployments',
          name: 'deployments',
          builder: (context, state) => const DeploymentsScreen(),
        ),
        GoRoute(
          path: '/releases',
          name: 'releases',
          builder: (context, state) => const ReleasesScreen(),
        ),
        GoRoute(
          path: '/analytics',
          name: 'analytics',
          builder: (context, state) => const AnalyticsScreen(),
        ),
        GoRoute(
          path: '/settings',
          name: 'settings',
          builder: (context, state) => const SettingsScreen(),
        ),
        GoRoute(
          path: '/profile',
          name: 'profile',
          builder: (context, state) => const ProfileScreen(),
        ),
      ],
    ),
  ],
);