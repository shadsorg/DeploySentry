import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

class FlagCreateScreen extends StatelessWidget {
  const FlagCreateScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Create Flag'),
        actions: [
          TextButton(
            onPressed: () {
              // TODO: Save flag
              context.go('/flags');
            },
            child: const Text('Save'),
          ),
        ],
      ),
      body: const Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.flag, size: 64, color: Colors.green),
            SizedBox(height: 16),
            Text('Create flag form coming soon...'),
          ],
        ),
      ),
    );
  }
}