import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';

import '../../services/api_client.dart';
import '../../models/flag.dart';
import '../../models/analytics.dart';

class FlagDetailScreen extends StatefulWidget {
  final String flagId;

  const FlagDetailScreen({
    super.key,
    required this.flagId,
  });

  @override
  State<FlagDetailScreen> createState() => _FlagDetailScreenState();
}

class _FlagDetailScreenState extends State<FlagDetailScreen>
    with SingleTickerProviderStateMixin {
  Flag? _flag;
  FlagStats? _stats;
  bool _isLoading = true;
  bool _isUpdating = false;
  String? _error;
  late TabController _tabController;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 3, vsync: this);
    _loadFlagDetails();
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  Future<void> _loadFlagDetails() async {
    if (!mounted) return;

    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      // Mock flag data - replace with actual API call
      await Future.delayed(const Duration(milliseconds: 500));

      final flag = Flag(
        id: widget.flagId,
        projectId: 'proj-1',
        environmentId: 'env-1',
        key: 'new-checkout-flow',
        name: 'New Checkout Flow',
        description: 'Enable the redesigned checkout experience with improved UX and conversion tracking. This flag controls the rollout of our new checkout system.',
        flagType: FlagType.boolean,
        category: FlagCategory.feature,
        purpose: 'Improve conversion rates and user experience',
        owners: ['team-frontend', 'alice@example.com'],
        isPermanent: false,
        expiresAt: '2024-12-31T23:59:59Z',
        enabled: true,
        defaultValue: 'false',
        archived: false,
        tags: ['frontend', 'checkout', 'conversion'],
        createdBy: 'alice@example.com',
        createdAt: '2024-03-01T10:00:00Z',
        updatedAt: '2024-03-20T14:30:00Z',
      );

      final stats = FlagStats(
        flagKey: flag.key,
        totalEvaluations: 25420,
        cacheHitRate: 96.8,
        averageLatencyMs: 1.8,
        errorRate: 0.05,
        resultDistribution: {
          'true': 12840,
          'false': 12580,
        },
      );

      if (mounted) {
        setState(() {
          _flag = flag;
          _stats = stats;
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

  Future<void> _toggleFlag() async {
    if (_flag == null || _isUpdating) return;

    setState(() => _isUpdating = true);

    try {
      final newEnabled = !_flag!.enabled;

      // Optimistic update
      setState(() {
        _flag = _flag!.copyWith(enabled: newEnabled);
      });

      // API call would go here
      await Future.delayed(const Duration(milliseconds: 300));
      // await apiClient.toggleFlag(_flag!.id, newEnabled);

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              'Flag ${newEnabled ? 'enabled' : 'disabled'} successfully',
            ),
            backgroundColor: Colors.green,
          ),
        );
      }
    } catch (e) {
      // Revert optimistic update
      setState(() {
        _flag = _flag!.copyWith(enabled: !_flag!.enabled);
      });

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Failed to toggle flag: $e'),
            backgroundColor: Colors.red,
          ),
        );
      }
    } finally {
      if (mounted) {
        setState(() => _isUpdating = false);
      }
    }
  }

  Future<void> _archiveFlag() async {
    if (_flag == null) return;

    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Archive Flag'),
        content: Text('Are you sure you want to archive "${_flag!.name}"?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: const Text('Archive'),
          ),
        ],
      ),
    );

    if (confirmed == true && mounted) {
      try {
        // await apiClient.archiveFlag(_flag!.id);
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Flag archived successfully'),
            backgroundColor: Colors.orange,
          ),
        );
        context.go('/flags');
      } catch (e) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Failed to archive flag: $e'),
            backgroundColor: Colors.red,
          ),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_isLoading) {
      return Scaffold(
        appBar: AppBar(title: const Text('Loading...')),
        body: const Center(child: CircularProgressIndicator()),
      );
    }

    if (_error != null) {
      return Scaffold(
        appBar: AppBar(title: const Text('Error')),
        body: Center(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(Icons.error_outline, size: 64, color: Colors.red[300]),
              const SizedBox(height: 16),
              Text('Error loading flag', style: Theme.of(context).textTheme.headlineSmall),
              const SizedBox(height: 8),
              Text(_error!, style: Theme.of(context).textTheme.bodyMedium),
              const SizedBox(height: 16),
              ElevatedButton(
                onPressed: _loadFlagDetails,
                child: const Text('Retry'),
              ),
            ],
          ),
        ),
      );
    }

    if (_flag == null) {
      return Scaffold(
        appBar: AppBar(title: const Text('Not Found')),
        body: const Center(child: Text('Flag not found')),
      );
    }

    return Scaffold(
      appBar: AppBar(
        title: Text(_flag!.name),
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: _loadFlagDetails,
          ),
          PopupMenuButton(
            itemBuilder: (context) => [
              PopupMenuItem(
                onTap: () {
                  context.go('/flags/${_flag!.id}/edit');
                },
                child: const Row(
                  children: [
                    Icon(Icons.edit),
                    SizedBox(width: 8),
                    Text('Edit'),
                  ],
                ),
              ),
              PopupMenuItem(
                onTap: () {
                  Clipboard.setData(ClipboardData(text: _flag!.key));
                  ScaffoldMessenger.of(context).showSnackBar(
                    const SnackBar(content: Text('Flag key copied to clipboard')),
                  );
                },
                child: const Row(
                  children: [
                    Icon(Icons.copy),
                    SizedBox(width: 8),
                    Text('Copy Key'),
                  ],
                ),
              ),
              PopupMenuItem(
                onTap: _archiveFlag,
                child: const Row(
                  children: [
                    Icon(Icons.archive, color: Colors.orange),
                    SizedBox(width: 8),
                    Text('Archive'),
                  ],
                ),
              ),
            ],
          ),
        ],
        bottom: TabBar(
          controller: _tabController,
          tabs: const [
            Tab(text: 'Details'),
            Tab(text: 'Analytics'),
            Tab(text: 'History'),
          ],
        ),
      ),
      body: TabBarView(
        controller: _tabController,
        children: [
          _buildDetailsTab(),
          _buildAnalyticsTab(),
          _buildHistoryTab(),
        ],
      ),
    );
  }

  Widget _buildDetailsTab() {
    return RefreshIndicator(
      onRefresh: _loadFlagDetails,
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Status and toggle
            _FlagStatusCard(
              flag: _flag!,
              isUpdating: _isUpdating,
              onToggle: _toggleFlag,
            ),
            const SizedBox(height: 16),

            // Basic information
            _InfoCard(
              title: 'Basic Information',
              children: [
                _InfoRow('Key', _flag!.key, monospace: true),
                _InfoRow('Type', _flag!.flagType.name.toUpperCase()),
                _InfoRow('Category', _flag!.category.name.toUpperCase()),
                _InfoRow('Purpose', _flag!.purpose),
                if (_flag!.description.isNotEmpty)
                  _InfoRow('Description', _flag!.description),
              ],
            ),
            const SizedBox(height: 16),

            // Configuration
            _InfoCard(
              title: 'Configuration',
              children: [
                _InfoRow('Default Value', _flag!.defaultValue, monospace: true),
                _InfoRow('Permanent', _flag!.isPermanent ? 'Yes' : 'No'),
                if (!_flag!.isPermanent && _flag!.expiresAt != null)
                  _InfoRow('Expires', _formatDate(_flag!.expiresAt!)),
              ],
            ),
            const SizedBox(height: 16),

            // Ownership & Tags
            _InfoCard(
              title: 'Ownership & Tags',
              children: [
                _InfoRow('Owners', _flag!.owners.join(', ')),
                if (_flag!.tags.isNotEmpty)
                  _TagRow('Tags', _flag!.tags),
              ],
            ),
            const SizedBox(height: 16),

            // Metadata
            _InfoCard(
              title: 'Metadata',
              children: [
                _InfoRow('Created', _formatDate(_flag!.createdAt)),
                _InfoRow('Updated', _formatDate(_flag!.updatedAt)),
                _InfoRow('Created By', _flag!.createdBy),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildAnalyticsTab() {
    if (_stats == null) {
      return const Center(child: CircularProgressIndicator());
    }

    return RefreshIndicator(
      onRefresh: _loadFlagDetails,
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Performance metrics
            _MetricsCard(
              title: 'Performance Metrics',
              metrics: [
                _MetricItem(
                  label: 'Total Evaluations',
                  value: _formatNumber(_stats!.totalEvaluations),
                  icon: Icons.bar_chart,
                ),
                _MetricItem(
                  label: 'Cache Hit Rate',
                  value: '${_stats!.cacheHitRate.toStringAsFixed(1)}%',
                  icon: Icons.speed,
                  color: _stats!.cacheHitRate > 90 ? Colors.green : Colors.orange,
                ),
                _MetricItem(
                  label: 'Avg Latency',
                  value: '${_stats!.averageLatencyMs.toStringAsFixed(1)}ms',
                  icon: Icons.timer,
                  color: _stats!.averageLatencyMs < 5 ? Colors.green : Colors.orange,
                ),
                _MetricItem(
                  label: 'Error Rate',
                  value: '${_stats!.errorRate.toStringAsFixed(2)}%',
                  icon: Icons.error_outline,
                  color: _stats!.errorRate < 1 ? Colors.green : Colors.red,
                ),
              ],
            ),
            const SizedBox(height: 16),

            // Result distribution
            _ResultDistributionCard(
              title: 'Result Distribution',
              distribution: _stats!.resultDistribution,
              totalEvaluations: _stats!.totalEvaluations,
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildHistoryTab() {
    return const Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.history, size: 64, color: Colors.grey),
          SizedBox(height: 16),
          Text('Audit History'),
          SizedBox(height: 8),
          Text('Flag change history coming soon...'),
        ],
      ),
    );
  }

  String _formatDate(String dateString) {
    final date = DateTime.parse(dateString);
    return DateFormat('MMM d, yyyy h:mm a').format(date);
  }

  String _formatNumber(int number) {
    return NumberFormat('#,###').format(number);
  }
}

class _FlagStatusCard extends StatelessWidget {
  final Flag flag;
  final bool isUpdating;
  final VoidCallback onToggle;

  const _FlagStatusCard({
    required this.flag,
    required this.isUpdating,
    required this.onToggle,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          children: [
            Icon(
              Icons.flag,
              size: 48,
              color: flag.enabled ? Colors.green : Colors.grey,
            ),
            const SizedBox(width: 16),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    flag.enabled ? 'ENABLED' : 'DISABLED',
                    style: Theme.of(context).textTheme.titleLarge?.copyWith(
                          fontWeight: FontWeight.bold,
                          color: flag.enabled ? Colors.green : Colors.grey,
                        ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    flag.enabled
                        ? 'Flag is active and evaluating'
                        : 'Flag is disabled',
                    style: Theme.of(context).textTheme.bodyMedium,
                  ),
                ],
              ),
            ),
            Switch(
              value: flag.enabled,
              onChanged: isUpdating ? null : (_) => onToggle(),
            ),
            if (isUpdating)
              const Padding(
                padding: EdgeInsets.only(left: 8),
                child: SizedBox(
                  width: 20,
                  height: 20,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

class _InfoCard extends StatelessWidget {
  final String title;
  final List<Widget> children;

  const _InfoCard({
    required this.title,
    required this.children,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              title,
              style: Theme.of(context).textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
            ),
            const SizedBox(height: 12),
            ...children,
          ],
        ),
      ),
    );
  }
}

class _InfoRow extends StatelessWidget {
  final String label;
  final String value;
  final bool monospace;

  const _InfoRow(
    this.label,
    this.value, {
    this.monospace = false,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 120,
            child: Text(
              label,
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: Colors.grey[600],
                    fontWeight: FontWeight.w500,
                  ),
            ),
          ),
          Expanded(
            child: Text(
              value,
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                    fontFamily: monospace ? 'monospace' : null,
                  ),
            ),
          ),
        ],
      ),
    );
  }
}

class _TagRow extends StatelessWidget {
  final String label;
  final List<String> tags;

  const _TagRow(this.label, this.tags);

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 120,
            child: Text(
              label,
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: Colors.grey[600],
                    fontWeight: FontWeight.w500,
                  ),
            ),
          ),
          Expanded(
            child: Wrap(
              spacing: 8,
              runSpacing: 4,
              children: tags.map((tag) => Chip(
                label: Text(tag),
                backgroundColor: Colors.blue[100],
                labelStyle: const TextStyle(fontSize: 12),
                materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
                visualDensity: VisualDensity.compact,
              )).toList(),
            ),
          ),
        ],
      ),
    );
  }
}

class _MetricsCard extends StatelessWidget {
  final String title;
  final List<_MetricItem> metrics;

  const _MetricsCard({
    required this.title,
    required this.metrics,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              title,
              style: Theme.of(context).textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
            ),
            const SizedBox(height: 16),
            GridView.count(
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              crossAxisCount: 2,
              childAspectRatio: 2,
              mainAxisSpacing: 12,
              crossAxisSpacing: 12,
              children: metrics.map((metric) => Container(
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: Colors.grey[50],
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Icon(
                          metric.icon,
                          size: 16,
                          color: metric.color ?? Theme.of(context).primaryColor,
                        ),
                        const SizedBox(width: 4),
                        Expanded(
                          child: Text(
                            metric.label,
                            style: Theme.of(context).textTheme.bodySmall,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                      ],
                    ),
                    const Spacer(),
                    Text(
                      metric.value,
                      style: Theme.of(context).textTheme.titleMedium?.copyWith(
                            fontWeight: FontWeight.bold,
                            color: metric.color,
                          ),
                    ),
                  ],
                ),
              )).toList(),
            ),
          ],
        ),
      ),
    );
  }
}

class _MetricItem {
  final String label;
  final String value;
  final IconData icon;
  final Color? color;

  const _MetricItem({
    required this.label,
    required this.value,
    required this.icon,
    this.color,
  });
}

class _ResultDistributionCard extends StatelessWidget {
  final String title;
  final Map<String, int> distribution;
  final int totalEvaluations;

  const _ResultDistributionCard({
    required this.title,
    required this.distribution,
    required this.totalEvaluations,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              title,
              style: Theme.of(context).textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
            ),
            const SizedBox(height: 16),
            ...distribution.entries.map((entry) {
              final percentage = (entry.value / totalEvaluations * 100);
              return Padding(
                padding: const EdgeInsets.only(bottom: 12),
                child: Column(
                  children: [
                    Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        Text(
                          entry.key,
                          style: Theme.of(context).textTheme.titleSmall?.copyWith(
                                fontWeight: FontWeight.bold,
                              ),
                        ),
                        Text(
                          '${NumberFormat('#,###').format(entry.value)} (${percentage.toStringAsFixed(1)}%)',
                          style: Theme.of(context).textTheme.bodyMedium,
                        ),
                      ],
                    ),
                    const SizedBox(height: 4),
                    LinearProgressIndicator(
                      value: percentage / 100,
                      backgroundColor: Colors.grey[300],
                      valueColor: AlwaysStoppedAnimation<Color>(
                        entry.key == 'true' ? Colors.green : Colors.blue,
                      ),
                    ),
                  ],
                ),
              );
            }),
          ],
        ),
      ),
    );
  }
}