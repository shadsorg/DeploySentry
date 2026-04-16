import 'package:flutter/material.dart';

import 'router.dart';
import 'services/realtime_manager.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Initialize real-time data manager
  await RealtimeManager().initialize(
    baseUrl: 'https://api.dr-sentry.com',
    refreshInterval: const Duration(seconds: 30),
  );

  runApp(const DeploySentryApp());
}

class DeploySentryApp extends StatelessWidget {
  const DeploySentryApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp.router(
      title: 'DeploySentry',
      theme: ThemeData(
        colorSchemeSeed: const Color(0xFF1A73E8),
        useMaterial3: true,
        appBarTheme: const AppBarTheme(
          centerTitle: true,
          elevation: 0,
        ),
        cardTheme: CardThemeData(
          elevation: 2,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(12),
          ),
        ),
        filledButtonTheme: FilledButtonThemeData(
          style: FilledButton.styleFrom(
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(8),
            ),
          ),
        ),
        elevatedButtonTheme: ElevatedButtonThemeData(
          style: ElevatedButton.styleFrom(
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(8),
            ),
          ),
        ),
      ),
      debugShowCheckedModeBanner: false,
      routerConfig: appRouter,
    );
  }
}
