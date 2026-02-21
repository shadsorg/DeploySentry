import 'package:flutter/material.dart';

void main() {
  runApp(const DeploySentryApp());
}

class DeploySentryApp extends StatelessWidget {
  const DeploySentryApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'DeploySentry',
      theme: ThemeData(
        colorSchemeSeed: const Color(0xFF1A73E8),
        useMaterial3: true,
      ),
      home: const HomePage(),
    );
  }
}

class HomePage extends StatelessWidget {
  const HomePage({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('DeploySentry'),
      ),
      body: const Center(
        child: Text('Deploy, release, and feature flag management.'),
      ),
    );
  }
}
