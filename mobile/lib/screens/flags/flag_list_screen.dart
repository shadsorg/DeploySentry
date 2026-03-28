import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../widgets/app_shell.dart';
import '../../services/api_client.dart';
import '../../models/flag.dart';

class FlagListScreen extends StatefulWidget {
  const FlagListScreen({super.key});

  @override
  State<FlagListScreen> createState() => _FlagListScreenState();
}

class _FlagListScreenState extends State<FlagListScreen>
    with SingleTickerProviderStateMixin {
  List<Flag> _flags = [];
  bool _isLoading = true;
  String? _error;
  String? _selectedCategory;
  bool? _showArchived;
  late TabController _tabController;

  final List<String> _categories = [
    'All',
    'release',
    'feature',
    'experiment',
    'ops',
    'permission'
  ];

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
    _loadFlags();
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  Future<void> _loadFlags() async {
    if (!mounted) return;

    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      // Mock data for now - replace with actual API call
      await Future.delayed(const Duration(milliseconds: 500));

      final mockFlags = [
        Flag(
          id: '1',
          projectId: 'proj-1',
          environmentId: 'env-1',
          key: 'new-checkout',
          name: 'New Checkout Flow',
          description: 'Enable the redesigned checkout experience',
          flagType: FlagType.boolean,
          category: FlagCategory.feature,
          purpose: 'Improve conversion rates',
          owners: ['team-frontend'],
          isPermanent: false,
          expiresAt: '2024-12-31T23:59:59Z',
          enabled: true,
          defaultValue: 'false',
          archived: false,
          tags: ['frontend', 'checkout'],
          createdBy: 'user-1',
          createdAt: '2024-03-01T10:00:00Z',
          updatedAt: '2024-03-15T14:30:00Z',
        ),
        Flag(
          id: '2',
          projectId: 'proj-1',
          environmentId: 'env-1',
          key: 'api-rate-limit',
          name: 'API Rate Limit',
          description: 'Dynamic API rate limiting',
          flagType: FlagType.integer,
          category: FlagCategory.ops,
          purpose: 'Prevent API abuse',
          owners: ['team-backend'],
          isPermanent: true,
          expiresAt: null,
          enabled: true,
          defaultValue: '100',
          archived: false,
          tags: ['api', 'security'],
          createdBy: 'user-2',
          createdAt: '2024-02-15T09:00:00Z',
          updatedAt: '2024-03-20T11:45:00Z',
        ),
        Flag(
          id: '3',
          projectId: 'proj-1',
          environmentId: 'env-1',
          key: 'legacy-feature',
          name: 'Legacy Feature',
          description: 'Old feature being phased out',
          flagType: FlagType.boolean,
          category: FlagCategory.release,
          purpose: 'Safe rollback capability',
          owners: ['team-platform'],
          isPermanent: false,
          expiresAt: '2024-06-30T23:59:59Z',
          enabled: false,
          defaultValue: 'false',
          archived: true,
          tags: ['legacy'],
          createdBy: 'user-3',
          createdAt: '2024-01-10T08:00:00Z',
          updatedAt: '2024-03-10T16:20:00Z',
        ),
      ];

      if (mounted) {
        setState(() {
          _flags = mockFlags;
          _isLoading = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _isLoading = false;
        });
      }
    }
  }

  List<Flag> get _filteredFlags {
    return _flags.where((flag) {
      // Filter by tab (active/archived)
      if (_tabController.index == 0 && flag.archived) return false;
      if (_tabController.index == 1 && !flag.archived) return false;

      // Filter by category
      if (_selectedCategory != null &&
          _selectedCategory != 'All' &&
          flag.category.name != _selectedCategory) {
        return false;
      }

      return true;
    }).toList();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Feature Flags'),
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: _loadFlags,
          ),
          PopupMenuButton<String>(
            icon: const Icon(Icons.filter_list),
            tooltip: 'Filter by category',
            onSelected: (String category) {
              setState(() {
                _selectedCategory = category == 'All' ? null : category;
              });
            },
            itemBuilder: (BuildContext context) {
              return _categories.map((String category) {
                return PopupMenuItem<String>(
                  value: category,
                  child: Row(
                    children: [
                      if (_selectedCategory == category ||
                          (category == 'All' && _selectedCategory == null))
                        const Icon(Icons.check, size: 16),
                      if (_selectedCategory != category &&
                          !(category == 'All' && _selectedCategory == null))
                        const SizedBox(width: 16),
                      const SizedBox(width: 8),
                      Text(category == 'All' ? 'All Categories' : category.toUpperCase()),
                    ],
                  ),
                );
              }).toList();
            },
          ),
        ],
        bottom: TabBar(
          controller: _tabController,
          onTap: (index) => setState(() {}),
          tabs: [
            Tab(
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Icon(Icons.flag),
                  const SizedBox(width: 8),
                  Text('Active (${_flags.where((f) => !f.archived).length})'),
                ],
              ),
            ),
            Tab(
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Icon(Icons.archive),
                  const SizedBox(width: 8),
                  Text('Archived (${_flags.where((f) => f.archived).length})'),
                ],
              ),
            ),
          ],
        ),
      ),
      drawer: const AppDrawer(),
      body: _buildBody(),
      floatingActionButton: FloatingActionButton(
        onPressed: () => context.go('/flags/new'),
        tooltip: 'Create Flag',
        child: const Icon(Icons.add),
      ),
    );
  }

  Widget _buildBody() {
    if (_isLoading) {
      return const Center(child: CircularProgressIndicator());
    }

    if (_error != null) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.error_outline, size: 64, color: Colors.red[300]),
            const SizedBox(height: 16),
            Text('Error loading flags', style: Theme.of(context).textTheme.headlineSmall),
            const SizedBox(height: 8),
            Text(_error!, style: Theme.of(context).textTheme.bodyMedium),
            const SizedBox(height: 16),
            ElevatedButton(
              onPressed: _loadFlags,
              child: const Text('Retry'),
            ),
          ],
        ),
      );
    }

    final filteredFlags = _filteredFlags;

    if (filteredFlags.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              _tabController.index == 0 ? Icons.flag : Icons.archive,
              size: 64,
              color: Colors.grey[400],
            ),
            const SizedBox(height: 16),
            Text(
              _tabController.index == 0 ? 'No active flags' : 'No archived flags',
              style: Theme.of(context).textTheme.headlineSmall,
            ),
            const SizedBox(height: 8),
            if (_tabController.index == 0) ...[
              const Text('Create your first feature flag'),
              const SizedBox(height: 16),
              ElevatedButton.icon(
                onPressed: () => context.go('/flags/new'),
                icon: const Icon(Icons.add),
                label: const Text('Create Flag'),
              ),
            ] else
              const Text('No flags have been archived yet'),
          ],
        ),
      );
    }

    return RefreshIndicator(
      onRefresh: _loadFlags,
      child: ListView.builder(
        padding: const EdgeInsets.all(16),
        itemCount: filteredFlags.length,
        itemBuilder: (context, index) {
          final flag = filteredFlags[index];
          return _FlagCard(
            flag: flag,
            onTap: () => context.go('/flags/${flag.id}'),
            onToggle: (enabled) => _toggleFlag(flag, enabled),
          );
        },
      ),
    );
  }

  Future<void> _toggleFlag(Flag flag, bool enabled) async {
    try {
      // Update UI optimistically
      setState(() {
        final index = _flags.indexWhere((f) => f.id == flag.id);
        if (index != -1) {
          _flags[index] = flag.copyWith(enabled: enabled);
        }
      });

      // Make API call
      await apiClient.toggleFlag(flag.id, enabled);

      // Show success message
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              'Flag "${flag.name}" ${enabled ? 'enabled' : 'disabled'}',
            ),
          ),
        );
      }
    } catch (e) {
      // Revert optimistic update on error
      setState(() {
        final index = _flags.indexWhere((f) => f.id == flag.id);
        if (index != -1) {
          _flags[index] = flag;
        }
      });

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Failed to toggle flag: $e'),
            backgroundColor: Colors.red,
          ),
        );
      }
    }
  }
}

class _FlagCard extends StatelessWidget {
  final Flag flag;
  final VoidCallback onTap;
  final Function(bool) onToggle;

  const _FlagCard({
    required this.flag,
    required this.onTap,
    required this.onToggle,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          flag.name,
                          style: Theme.of(context).textTheme.titleMedium?.copyWith(
                                fontWeight: FontWeight.bold,
                              ),
                        ),
                        const SizedBox(height: 4),
                        Text(
                          flag.key,
                          style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                                color: Colors.grey[600],
                                fontFamily: 'monospace',
                              ),
                        ),
                      ],
                    ),
                  ),
                  if (!flag.archived)
                    Switch(
                      value: flag.enabled,
                      onChanged: onToggle,
                    ),
                  if (flag.archived)
                    const Icon(Icons.archive, color: Colors.grey),
                ],
              ),
              if (flag.description.isNotEmpty) ...[
                const SizedBox(height: 8),
                Text(
                  flag.description,
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ],
              const SizedBox(height: 12),
              Row(
                children: [
                  _CategoryChip(category: flag.category),
                  const SizedBox(width: 8),
                  _TypeChip(type: flag.flagType),
                  if (!flag.isPermanent) ...[
                    const SizedBox(width: 8),
                    Icon(
                      Icons.schedule,
                      size: 16,
                      color: Colors.orange[700],
                    ),
                    const SizedBox(width: 4),
                    Text(
                      'Expires',
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.orange[700],
                      ),
                    ),
                  ],
                  const Spacer(),
                  const Icon(Icons.arrow_forward_ios, size: 16),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _CategoryChip extends StatelessWidget {
  final FlagCategory category;

  const _CategoryChip({required this.category});

  @override
  Widget build(BuildContext context) {
    final colors = {
      FlagCategory.release: Colors.blue,
      FlagCategory.feature: Colors.green,
      FlagCategory.experiment: Colors.purple,
      FlagCategory.ops: Colors.orange,
      FlagCategory.permission: Colors.red,
    };

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: colors[category]?.withOpacity(0.1),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors[category]?.withOpacity(0.3) ?? Colors.grey),
      ),
      child: Text(
        category.name.toUpperCase(),
        style: TextStyle(
          fontSize: 10,
          fontWeight: FontWeight.bold,
          color: colors[category],
        ),
      ),
    );
  }
}

class _TypeChip extends StatelessWidget {
  final FlagType type;

  const _TypeChip({required this.type});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: Colors.grey[200],
        borderRadius: BorderRadius.circular(12),
      ),
      child: Text(
        type.name.toUpperCase(),
        style: const TextStyle(
          fontSize: 10,
          fontWeight: FontWeight.bold,
          color: Colors.black54,
        ),
      ),
    );
  }
}