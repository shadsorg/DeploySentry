import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';

import '../widgets/app_shell.dart';
import '../services/api_client.dart';
import '../services/realtime_manager.dart';
import '../models/analytics.dart';
import '../models/flag.dart';
import '../models/deployment.dart';

class DashboardScreen extends StatefulWidget {
  const DashboardScreen({super.key});

  @override
  State<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends State<DashboardScreen> with RealtimeDataMixin {
  AnalyticsSummary? _summary;
  List<Flag> _recentFlags = [];
  List<Deployment> _recentDeployments = [];
  SystemHealth? _systemHealth;

  bool _isLoading = true;
  String? _error;
  String _selectedProject = 'Main Project';
  String _selectedEnvironment = 'Production';

  final List<String> _projects = ['Main Project', 'Mobile App', 'API Service'];
  final List<String> _environments = ['Production', 'Staging', 'Development'];

  @override
  void initState() {
    super.initState();
    _loadDashboardData();

    // Initialize real-time updates
    startRealtimeUpdates();
    setupPeriodicRefresh(const Duration(seconds: 30), _loadDashboardData);
  }

  @override
  List<RealtimeEventType> get subscribedEvents => [
    RealtimeEventType.refresh,
    RealtimeEventType.flagUpdated,
    RealtimeEventType.deploymentStatusChanged,
    RealtimeEventType.systemAlert,
  ];

  @override
  void onRealtimeEvent(RealtimeEvent event) {
    if (!mounted) return;

    switch (event.type) {
      case RealtimeEventType.refresh:
      case RealtimeEventType.flagUpdated:
      case RealtimeEventType.deploymentStatusChanged:
        _loadDashboardData();
        break;
      case RealtimeEventType.systemAlert:
        if (event.data != null) {
          _showSystemAlert(event.data!);
        }
        break;
      default:
        break;
    }
  }

  Future<void> _loadDashboardData() async {
    if (!mounted) return;

    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      // Use real API calls with fallback to mock data for demo
      final futures = await Future.wait([
        _loadAnalyticsSummary(),
        _loadRecentFlags(),
        _loadRecentDeployments(),
        _loadSystemHealth(),
      ]);

      if (mounted) {
        setState(() {
          _summary = futures[0] as AnalyticsSummary;
          _recentFlags = futures[1] as List<Flag>;
          _recentDeployments = futures[2] as List<Deployment>;
          _systemHealth = futures[3] as SystemHealth?;
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

  Future<AnalyticsSummary> _loadAnalyticsSummary() async {
    try {
      return await apiClient.getAnalyticsSummary(
        projectId: 'proj_123',
        environmentId: 'env_456',
        timeRange: '24h',
      );
    } catch (e) {
      // Fallback to mock data for demo
      await Future.delayed(const Duration(milliseconds: 300));
      return AnalyticsSummary(
        flags: FlagPerformance(
          totalEvaluations: 15420,
          activeFlags: 12,
          cacheHitRate: 94.7,
          averageLatencyMs: 2.3,
        ),
        deployments: DeploymentPerformance(
          totalDeployments: 45,
          successRate: 97.8,
          averageDurationMinutes: 12.5,
          failedDeployments: 1,
        ),
        api: ApiPerformance(
          totalRequests: 28540,
          averageLatencyMs: 125.6,
          errorRate: 0.12,
          requestsPerSecond: 45.7,
        ),
      );
    }
  }

  Future<List<Flag>> _loadRecentFlags() async {
    try {
      final flags = await apiClient.getFlags(
        projectId: 'proj_123',
        environmentId: 'env_456',
        limit: 5,
      );
      return flags;
    } catch (e) {
      // Mock data fallback
      await Future.delayed(const Duration(milliseconds: 100));
      return [
        Flag(
          id: '1',
          key: 'new_checkout_flow',
          name: 'New Checkout Flow',
          description: 'Enhanced checkout experience with one-click payments',
          enabled: true,
          category: FlagCategory.feature,
          flagType: FlagType.boolean,
          defaultValue: 'false',
          targetingRules: [],
          projectId: 'proj_123',
          environmentId: 'env_456',
          createdAt: DateTime.now().subtract(const Duration(hours: 2)),
          updatedAt: DateTime.now().subtract(const Duration(minutes: 30)),
          createdBy: 'jane@example.com',
          updatedBy: 'jane@example.com',
        ),
        Flag(
          id: '2',
          key: 'mobile_push_notifications',
          name: 'Mobile Push Notifications',
          description: 'Enable push notifications for mobile users',
          enabled: false,
          category: FlagCategory.experiment,
          flagType: FlagType.boolean,
          defaultValue: 'false',
          targetingRules: [],
          projectId: 'proj_123',
          environmentId: 'env_456',
          createdAt: DateTime.now().subtract(const Duration(days: 1)),
          updatedAt: DateTime.now().subtract(const Duration(hours: 4)),
          createdBy: 'bob@example.com',
          updatedBy: 'jane@example.com',
        ),
      ];
    }
  }

  Future<List<Deployment>> _loadRecentDeployments() async {
    try {
      final deployments = await apiClient.getDeployments(
        projectId: 'proj_123',
        limit: 5,
      );
      return deployments;
    } catch (e) {
      // Mock data fallback
      await Future.delayed(const Duration(milliseconds: 100));
      return [
        Deployment(
          id: '1',
          projectId: 'proj_123',
          environmentId: 'env_456',
          releaseId: 'rel_789',
          version: 'v2.1.4',
          strategy: DeployStrategy.rolling,
          status: DeployStatus.completed,
          createdBy: 'github-actions',
          createdAt: DateTime.now().subtract(const Duration(hours: 3)).toIso8601String(),
          updatedAt: DateTime.now().subtract(const Duration(hours: 2, minutes: 45)).toIso8601String(),
          startedAt: DateTime.now().subtract(const Duration(hours: 3)).toIso8601String(),
          completedAt: DateTime.now().subtract(const Duration(hours: 2, minutes: 45)).toIso8601String(),
        ),
        Deployment(
          id: '2',
          projectId: 'proj_123',
          environmentId: 'env_456',
          releaseId: 'rel_790',
          version: 'v2.1.5',
          strategy: DeployStrategy.canary,
          status: DeployStatus.running,
          targetPercentage: 25.0,
          currentPercentage: 15.0,
          createdBy: 'jane@example.com',
          createdAt: DateTime.now().subtract(const Duration(minutes: 15)).toIso8601String(),
          updatedAt: DateTime.now().subtract(const Duration(minutes: 5)).toIso8601String(),
          startedAt: DateTime.now().subtract(const Duration(minutes: 15)).toIso8601String(),
        ),
      ];
    }
  }

  Future<SystemHealth?> _loadSystemHealth() async {
    try {
      return await apiClient.getSystemHealth();
    } catch (e) {
      // Mock health data
      await Future.delayed(const Duration(milliseconds: 50));
      return SystemHealth(
        api: ApiHealthMetrics(
          requestsPerSecond: 45.7,
          avgLatencyMs: 125.6,
          errorRate: 0.12,
          activeConnections: 234,
        ),
        database: DatabaseHealthMetrics(
          connections: 45,
          queryLatencyMs: 12.3,
          cacheHitRate: 94.7,
        ),
        resources: ResourceMetrics(
          cpuUsagePercent: 68.5,
          memoryUsagePercent: 72.1,
          memoryUsageBytes: 2847329280,
          diskUsagePercent: 45.2,
        ),
        timestamp: DateTime.now().toIso8601String(),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Dashboard'),
        actions: [
          PopupMenuButton<String>(
            icon: const Icon(Icons.business),
            tooltip: 'Project & Environment',
            onSelected: (value) {
              // Handle project/environment selection
            },
            itemBuilder: (context) => [
              PopupMenuItem<String>(
                value: 'project',
                child: Row(
                  children: [
                    const Icon(Icons.folder),
                    const SizedBox(width: 8),
                    Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        const Text('Project'),
                        Text(_selectedProject, style: Theme.of(context).textTheme.bodySmall),
                      ],
                    ),
                  ],
                ),
              ),
              PopupMenuItem<String>(
                value: 'environment',
                child: Row(
                  children: [
                    const Icon(Icons.cloud),
                    const SizedBox(width: 8),
                    Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        const Text('Environment'),
                        Text(_selectedEnvironment, style: Theme.of(context).textTheme.bodySmall),
                      ],
                    ),
                  ],
                ),
              ),
            ],
          ),
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: _loadDashboardData,
          ),
          IconButton(
            icon: const Icon(Icons.settings),
            onPressed: () => context.go('/settings'),
          ),
        ],
      ),
      drawer: const AppDrawer(),
      body: RefreshIndicator(
        onRefresh: _loadDashboardData,
        child: _buildBody(),
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
            Text('Error loading dashboard', style: Theme.of(context).textTheme.headlineSmall),
            const SizedBox(height: 8),
            Text(_error!, style: Theme.of(context).textTheme.bodyMedium),
            const SizedBox(height: 16),
            ElevatedButton(
              onPressed: _loadDashboardData,
              child: const Text('Retry'),
            ),
          ],
        ),
      );
    }

    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header with environment info
          _buildEnvironmentHeader(),
          const SizedBox(height: 24),

          // System status alerts
          _buildSystemAlerts(),

          // Quick actions
          _buildQuickActions(),
          const SizedBox(height: 24),

          // Performance overview
          _buildPerformanceOverview(),
          const SizedBox(height: 24),

          // Recent activity
          _buildRecentActivity(),
        ],
      ),
    );
  }

  Widget _buildEnvironmentHeader() {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(Icons.dashboard, color: Theme.of(context).primaryColor),
                const SizedBox(width: 8),
                Text(
                  _selectedProject,
                  style: Theme.of(context).textTheme.titleLarge?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
            Row(
              children: [
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                  decoration: BoxDecoration(
                    color: _getEnvironmentColor(_selectedEnvironment),
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Text(
                    _selectedEnvironment.toUpperCase(),
                    style: const TextStyle(
                      color: Colors.white,
                      fontSize: 12,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ),
                const SizedBox(width: 12),
                Text(
                  'Last updated: ${DateFormat('MMM d, HH:mm').format(DateTime.now())}',
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildSystemAlerts() {
    if (_systemHealth == null) return const SizedBox.shrink();

    final health = _systemHealth!;
    final hasAlerts = health.api.errorRate > 0.05 ||
                     health.resources.cpuUsagePercent > 80 ||
                     health.resources.memoryUsagePercent > 85;

    if (!hasAlerts) return const SizedBox.shrink();

    return Column(
      children: [
        Card(
          color: Colors.orange[50],
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Icon(Icons.warning, color: Colors.orange[700]),
                    const SizedBox(width: 8),
                    Text(
                      'System Alerts',
                      style: Theme.of(context).textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.bold,
                        color: Colors.orange[700],
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 12),
                if (health.api.errorRate > 0.05)
                  _buildAlert('High API error rate: ${(health.api.errorRate * 100).toStringAsFixed(2)}%', Icons.api),
                if (health.resources.cpuUsagePercent > 80)
                  _buildAlert('High CPU usage: ${health.resources.cpuUsagePercent.toStringAsFixed(1)}%', Icons.memory),
                if (health.resources.memoryUsagePercent > 85)
                  _buildAlert('High memory usage: ${health.resources.memoryUsagePercent.toStringAsFixed(1)}%', Icons.storage),
              ],
            ),
          ),
        ),
        const SizedBox(height: 16),
      ],
    );
  }

  Widget _buildAlert(String message, IconData icon) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        children: [
          Icon(icon, size: 16, color: Colors.orange[600]),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              message,
              style: Theme.of(context).textTheme.bodyMedium,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildQuickActions() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Quick Actions',
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
            fontWeight: FontWeight.bold,
          ),
        ),
        const SizedBox(height: 16),
        GridView.count(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          crossAxisCount: 2,
          mainAxisSpacing: 12,
          crossAxisSpacing: 12,
          childAspectRatio: 1.2,
          children: [
            _QuickActionCard(
              icon: Icons.flag,
              title: 'Create Flag',
              subtitle: 'New feature flag',
              color: Colors.blue,
              onTap: () => context.go('/flags/new'),
            ),
            _QuickActionCard(
              icon: Icons.rocket_launch,
              title: 'Deploy',
              subtitle: 'Start deployment',
              color: Colors.green,
              onTap: () => context.go('/deployments'),
            ),
            _QuickActionCard(
              icon: Icons.analytics,
              title: 'Analytics',
              subtitle: 'View metrics',
              color: Colors.purple,
              onTap: () => context.go('/analytics'),
            ),
            _QuickActionCard(
              icon: Icons.publish,
              title: 'Release',
              subtitle: 'Create release',
              color: Colors.orange,
              onTap: () => context.go('/releases'),
            ),
          ],
        ),
      ],
    );
  }

  Widget _buildPerformanceOverview() {
    if (_summary == null) return const SizedBox.shrink();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Performance Overview',
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
            fontWeight: FontWeight.bold,
          ),
        ),
        const SizedBox(height: 16),

        // Flag metrics
        _MetricCard(
          title: 'Feature Flags',
          icon: Icons.flag,
          color: Colors.blue,
          metrics: [
            _Metric('Evaluations', _formatNumber(_summary!.flags.totalEvaluations)),
            _Metric('Active', '${_summary!.flags.activeFlags}'),
            _Metric('Cache Hit', '${_summary!.flags.cacheHitRate.toStringAsFixed(1)}%'),
            _Metric('Latency', '${_summary!.flags.averageLatencyMs.toStringAsFixed(1)}ms'),
          ],
          onTap: () => context.go('/flags'),
        ),
        const SizedBox(height: 12),

        // Deployment metrics
        _MetricCard(
          title: 'Deployments',
          icon: Icons.rocket_launch,
          color: Colors.green,
          metrics: [
            _Metric('Total', '${_summary!.deployments.totalDeployments}'),
            _Metric('Success', '${_summary!.deployments.successRate.toStringAsFixed(1)}%'),
            _Metric('Avg Time', '${_summary!.deployments.averageDurationMinutes.toStringAsFixed(1)}m'),
            _Metric('Failed', '${_summary!.deployments.failedDeployments}'),
          ],
          onTap: () => context.go('/deployments'),
        ),
        const SizedBox(height: 12),

        // API metrics
        _MetricCard(
          title: 'API Performance',
          icon: Icons.api,
          color: Colors.purple,
          metrics: [
            _Metric('Requests', _formatNumber(_summary!.api.totalRequests)),
            _Metric('RPS', '${_summary!.api.requestsPerSecond.toStringAsFixed(1)}'),
            _Metric('Latency', '${_summary!.api.averageLatencyMs.toStringAsFixed(0)}ms'),
            _Metric('Errors', '${(_summary!.api.errorRate * 100).toStringAsFixed(2)}%'),
          ],
          onTap: () => context.go('/analytics'),
        ),
      ],
    );
  }

  Widget _buildRecentActivity() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Recent Activity',
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
            fontWeight: FontWeight.bold,
          ),
        ),
        const SizedBox(height: 16),

        // Recent Flags
        if (_recentFlags.isNotEmpty) ...[
          _ActivitySection(
            title: 'Recent Flags',
            icon: Icons.flag,
            items: _recentFlags.take(3).map((flag) => _ActivityItem(
              title: flag.name,
              subtitle: 'Flag ${flag.enabled ? "enabled" : "disabled"}',
              time: _formatRelativeTime(flag.updatedAt),
              status: flag.enabled ? 'active' : 'inactive',
              onTap: () => context.go('/flags/${flag.id}'),
            )).toList(),
            onViewAll: () => context.go('/flags'),
          ),
          const SizedBox(height: 16),
        ],

        // Recent Deployments
        if (_recentDeployments.isNotEmpty)
          _ActivitySection(
            title: 'Recent Deployments',
            icon: Icons.rocket_launch,
            items: _recentDeployments.take(3).map((deployment) => _ActivityItem(
              title: deployment.version,
              subtitle: '${deployment.strategy.toString().split('.').last} deployment',
              time: _formatRelativeTime(DateTime.parse(deployment.startedAt ?? deployment.createdAt)),
              status: deployment.status.toString().split('.').last,
              onTap: () => context.go('/deployments'),
            )).toList(),
            onViewAll: () => context.go('/deployments'),
          ),
      ],
    );
  }

  Color _getEnvironmentColor(String environment) {
    switch (environment.toLowerCase()) {
      case 'production':
        return Colors.red;
      case 'staging':
        return Colors.orange;
      case 'development':
        return Colors.blue;
      default:
        return Colors.grey;
    }
  }

  String _formatNumber(int number) {
    if (number >= 1000000) {
      return '${(number / 1000000).toStringAsFixed(1)}M';
    } else if (number >= 1000) {
      return '${(number / 1000).toStringAsFixed(1)}K';
    }
    return number.toString();
  }

  String _formatRelativeTime(DateTime dateTime) {
    final now = DateTime.now();
    final difference = now.difference(dateTime);

    if (difference.inMinutes < 60) {
      return '${difference.inMinutes}m ago';
    } else if (difference.inHours < 24) {
      return '${difference.inHours}h ago';
    } else {
      return '${difference.inDays}d ago';
    }
  }

  void _showSystemAlert(Map<String, dynamic> alertData) {
    final severity = alertData['severity'] as String? ?? 'info';
    final message = alertData['message'] as String? ?? 'System alert';

    Color backgroundColor;
    IconData icon;

    switch (severity.toLowerCase()) {
      case 'critical':
      case 'error':
        backgroundColor = Colors.red;
        icon = Icons.error;
        break;
      case 'warning':
        backgroundColor = Colors.orange;
        icon = Icons.warning;
        break;
      default:
        backgroundColor = Colors.blue;
        icon = Icons.info;
        break;
    }

    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Row(
          children: [
            Icon(icon, color: Colors.white),
            const SizedBox(width: 12),
            Expanded(child: Text(message)),
          ],
        ),
        backgroundColor: backgroundColor,
        duration: const Duration(seconds: 5),
        action: SnackBarAction(
          label: 'Dismiss',
          textColor: Colors.white,
          onPressed: () {
            ScaffoldMessenger.of(context).hideCurrentSnackBar();
          },
        ),
      ),
    );
  }
}

class _QuickActionCard extends StatelessWidget {
  final IconData icon;
  final String title;
  final String subtitle;
  final Color color;
  final VoidCallback onTap;

  const _QuickActionCard({
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.color,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(icon, size: 32, color: color),
              const SizedBox(height: 12),
              Text(
                title,
                style: Theme.of(context).textTheme.titleSmall?.copyWith(
                  fontWeight: FontWeight.bold,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 4),
              Text(
                subtitle,
                style: Theme.of(context).textTheme.bodySmall,
                textAlign: TextAlign.center,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _MetricCard extends StatelessWidget {
  final String title;
  final IconData icon;
  final Color color;
  final List<_Metric> metrics;
  final VoidCallback onTap;

  const _MetricCard({
    required this.title,
    required this.icon,
    required this.color,
    required this.metrics,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
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
                  Icon(icon, color: color),
                  const SizedBox(width: 8),
                  Text(
                    title,
                    style: Theme.of(context).textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                  const Spacer(),
                  const Icon(Icons.arrow_forward_ios, size: 16),
                ],
              ),
              const SizedBox(height: 16),
              GridView.count(
                shrinkWrap: true,
                physics: const NeverScrollableScrollPhysics(),
                crossAxisCount: 4,
                childAspectRatio: 1.2,
                children: metrics.map((metric) => Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      metric.value,
                      style: Theme.of(context).textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.bold,
                        color: color,
                      ),
                    ),
                    Text(
                      metric.label,
                      style: Theme.of(context).textTheme.bodySmall,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ],
                )).toList(),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ActivitySection extends StatelessWidget {
  final String title;
  final IconData icon;
  final List<_ActivityItem> items;
  final VoidCallback onViewAll;

  const _ActivitySection({
    required this.title,
    required this.icon,
    required this.items,
    required this.onViewAll,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(icon, color: Theme.of(context).primaryColor),
                const SizedBox(width: 8),
                Text(
                  title,
                  style: Theme.of(context).textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const Spacer(),
                TextButton(
                  onPressed: onViewAll,
                  child: const Text('View All'),
                ),
              ],
            ),
            const SizedBox(height: 12),
            ...items.map((item) => _buildActivityTile(context, item)),
          ],
        ),
      ),
    );
  }

  Widget _buildActivityTile(BuildContext context, _ActivityItem item) {
    return ListTile(
      contentPadding: EdgeInsets.zero,
      title: Text(item.title),
      subtitle: Text(item.subtitle),
      trailing: Column(
        crossAxisAlignment: CrossAxisAlignment.end,
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Text(
            item.time,
            style: Theme.of(context).textTheme.bodySmall,
          ),
          const SizedBox(height: 4),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
            decoration: BoxDecoration(
              color: _getStatusColor(item.status),
              borderRadius: BorderRadius.circular(10),
            ),
            child: Text(
              item.status,
              style: const TextStyle(
                color: Colors.white,
                fontSize: 10,
                fontWeight: FontWeight.bold,
              ),
            ),
          ),
        ],
      ),
      onTap: item.onTap,
    );
  }

  Color _getStatusColor(String status) {
    switch (status.toLowerCase()) {
      case 'active':
      case 'success':
      case 'completed':
        return Colors.green;
      case 'inactive':
      case 'disabled':
        return Colors.grey;
      case 'in_progress':
      case 'running':
        return Colors.blue;
      case 'failed':
      case 'error':
        return Colors.red;
      default:
        return Colors.grey;
    }
  }
}

class _ActivityItem {
  final String title;
  final String subtitle;
  final String time;
  final String status;
  final VoidCallback onTap;

  const _ActivityItem({
    required this.title,
    required this.subtitle,
    required this.time,
    required this.status,
    required this.onTap,
  });
}

class _Metric {
  final String label;
  final String value;

  const _Metric(this.label, this.value);
}