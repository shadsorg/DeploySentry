import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

class AppShell extends StatelessWidget {
  final Widget child;

  const AppShell({
    super.key,
    required this.child,
  });

  @override
  Widget build(BuildContext context) {
    final currentLocation = GoRouterState.of(context).uri.path;

    return Scaffold(
      body: child,
      bottomNavigationBar: BottomNavigationBar(
        type: BottomNavigationBarType.fixed,
        currentIndex: _calculateSelectedIndex(currentLocation),
        onTap: (index) => _onItemTapped(context, index),
        items: const [
          BottomNavigationBarItem(
            icon: Icon(Icons.dashboard),
            label: 'Dashboard',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.flag),
            label: 'Flags',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.rocket_launch),
            label: 'Deployments',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.publish),
            label: 'Releases',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.analytics),
            label: 'Analytics',
          ),
        ],
      ),
    );
  }

  int _calculateSelectedIndex(String location) {
    if (location.startsWith('/dashboard')) return 0;
    if (location.startsWith('/flags')) return 1;
    if (location.startsWith('/deployments')) return 2;
    if (location.startsWith('/releases')) return 3;
    if (location.startsWith('/analytics')) return 4;
    return 0;
  }

  void _onItemTapped(BuildContext context, int index) {
    switch (index) {
      case 0:
        context.go('/dashboard');
        break;
      case 1:
        context.go('/flags');
        break;
      case 2:
        context.go('/deployments');
        break;
      case 3:
        context.go('/releases');
        break;
      case 4:
        context.go('/analytics');
        break;
    }
  }
}

class AppDrawer extends StatelessWidget {
  const AppDrawer({super.key});

  @override
  Widget build(BuildContext context) {
    final currentLocation = GoRouterState.of(context).uri.path;

    return Drawer(
      child: ListView(
        padding: EdgeInsets.zero,
        children: [
          const DrawerHeader(
            decoration: BoxDecoration(
              color: Colors.blue,
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'DeploySentry',
                  style: TextStyle(
                    color: Colors.white,
                    fontSize: 24,
                    fontWeight: FontWeight.bold,
                  ),
                ),
                SizedBox(height: 8),
                Text(
                  'Deploy, release, and feature flag management',
                  style: TextStyle(
                    color: Colors.white70,
                    fontSize: 14,
                  ),
                ),
              ],
            ),
          ),
          _DrawerItem(
            icon: Icons.dashboard,
            title: 'Dashboard',
            isSelected: currentLocation.startsWith('/dashboard'),
            onTap: () {
              context.go('/dashboard');
              Navigator.pop(context);
            },
          ),
          _DrawerItem(
            icon: Icons.flag,
            title: 'Feature Flags',
            isSelected: currentLocation.startsWith('/flags'),
            onTap: () {
              context.go('/flags');
              Navigator.pop(context);
            },
          ),
          _DrawerItem(
            icon: Icons.rocket_launch,
            title: 'Deployments',
            isSelected: currentLocation.startsWith('/deployments'),
            onTap: () {
              context.go('/deployments');
              Navigator.pop(context);
            },
          ),
          _DrawerItem(
            icon: Icons.publish,
            title: 'Releases',
            isSelected: currentLocation.startsWith('/releases'),
            onTap: () {
              context.go('/releases');
              Navigator.pop(context);
            },
          ),
          _DrawerItem(
            icon: Icons.analytics,
            title: 'Analytics',
            isSelected: currentLocation.startsWith('/analytics'),
            onTap: () {
              context.go('/analytics');
              Navigator.pop(context);
            },
          ),
          const Divider(),
          _DrawerItem(
            icon: Icons.settings,
            title: 'Settings',
            isSelected: currentLocation.startsWith('/settings'),
            onTap: () {
              context.go('/settings');
              Navigator.pop(context);
            },
          ),
        ],
      ),
    );
  }
}

class _DrawerItem extends StatelessWidget {
  final IconData icon;
  final String title;
  final bool isSelected;
  final VoidCallback onTap;

  const _DrawerItem({
    required this.icon,
    required this.title,
    required this.isSelected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return ListTile(
      leading: Icon(
        icon,
        color: isSelected ? Theme.of(context).primaryColor : null,
      ),
      title: Text(
        title,
        style: TextStyle(
          color: isSelected ? Theme.of(context).primaryColor : null,
          fontWeight: isSelected ? FontWeight.bold : null,
        ),
      ),
      selected: isSelected,
      onTap: onTap,
    );
  }
}