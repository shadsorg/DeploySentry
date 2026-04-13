import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../widgets/app_shell.dart';
import '../services/api_client.dart';

class SettingsScreen extends StatelessWidget {
  const SettingsScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Settings'),
      ),
      drawer: const AppDrawer(),
      body: ListView(
        children: [
          const _SectionHeader('Account'),
          ListTile(
            leading: const Icon(Icons.person),
            title: const Text('Profile'),
            subtitle: const Text('Manage your profile and preferences'),
            trailing: const Icon(Icons.chevron_right),
            onTap: () {
              context.push('/profile');
            },
          ),
          ListTile(
            leading: const Icon(Icons.notifications),
            title: const Text('Notifications'),
            subtitle: const Text('Configure notification preferences'),
            trailing: const Icon(Icons.chevron_right),
            onTap: () {
              // TODO: Navigate to notifications
            },
          ),
          const Divider(),
          const _SectionHeader('Application'),
          ListTile(
            leading: const Icon(Icons.palette),
            title: const Text('Theme'),
            subtitle: const Text('Choose your preferred theme'),
            trailing: const Icon(Icons.chevron_right),
            onTap: () {
              // TODO: Theme selection
            },
          ),
          ListTile(
            leading: const Icon(Icons.security),
            title: const Text('Security'),
            subtitle: const Text('API keys and security settings'),
            trailing: const Icon(Icons.chevron_right),
            onTap: () {
              // TODO: Security settings
            },
          ),
          const Divider(),
          const _SectionHeader('Support'),
          ListTile(
            leading: const Icon(Icons.help),
            title: const Text('Help & Documentation'),
            trailing: const Icon(Icons.chevron_right),
            onTap: () {
              // TODO: Help
            },
          ),
          ListTile(
            leading: const Icon(Icons.bug_report),
            title: const Text('Report Issue'),
            trailing: const Icon(Icons.chevron_right),
            onTap: () {
              // TODO: Bug report
            },
          ),
          ListTile(
            leading: const Icon(Icons.info),
            title: const Text('About'),
            trailing: const Icon(Icons.chevron_right),
            onTap: () {
              _showAboutDialog(context);
            },
          ),
          const Divider(),
          ListTile(
            leading: const Icon(Icons.logout, color: Colors.red),
            title: const Text('Sign Out'),
            textColor: Colors.red,
            onTap: () => _signOut(context),
          ),
        ],
      ),
    );
  }

  void _showAboutDialog(BuildContext context) {
    showAboutDialog(
      context: context,
      applicationName: 'DeploySentry',
      applicationVersion: '1.0.0',
      applicationIcon: const Icon(Icons.rocket_launch, size: 48),
      children: [
        const Text('Deploy, release, and feature flag management.'),
      ],
    );
  }

  void _signOut(BuildContext context) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Sign Out'),
        content: const Text('Are you sure you want to sign out?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () async {
              await apiClient.clearToken();
              if (context.mounted) {
                Navigator.pop(context);
                context.go('/login');
              }
            },
            child: const Text('Sign Out'),
          ),
        ],
      ),
    );
  }
}

class _SectionHeader extends StatelessWidget {
  final String title;

  const _SectionHeader(this.title);

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 24, 16, 8),
      child: Text(
        title.toUpperCase(),
        style: Theme.of(context).textTheme.labelMedium?.copyWith(
              color: Theme.of(context).colorScheme.primary,
              fontWeight: FontWeight.bold,
            ),
      ),
    );
  }
}