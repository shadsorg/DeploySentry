import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import '../widgets/app_shell.dart';
import '../services/api_client.dart';
import '../models/deployment.dart';

class DeploymentsScreen extends StatefulWidget {
  const DeploymentsScreen({super.key});

  @override
  State<DeploymentsScreen> createState() => _DeploymentsScreenState();
}

class _DeploymentsScreenState extends State<DeploymentsScreen>
    with TickerProviderStateMixin {
  late TabController _tabController;

  List<Deployment> _deployments = [];
  List<Deployment> _activeDeployments = [];
  bool _isLoading = true;
  String? _error;
  String _selectedEnvironment = 'All';
  String _selectedStatus = 'All';

  final List<String> _environments = ['All', 'Production', 'Staging', 'Development'];
  final List<String> _statuses = ['All', 'Running', 'Completed', 'Failed', 'Pending'];

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
    _loadDeployments();
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  Future<void> _loadDeployments() async {
    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      // Try real API call first, fallback to mock data
      List<Deployment> deployments;
      try {
        deployments = await apiClient.getDeployments(
          projectId: 'proj_123',
          limit: 50,
        );
      } catch (e) {
        // Mock data fallback
        await Future.delayed(const Duration(milliseconds: 500));
        deployments = _generateMockDeployments();
      }

      final activeDeployments = deployments
          .where((d) => d.status == DeployStatus.running || d.status == DeployStatus.pending)
          .toList();

      setState(() {
        _deployments = deployments;
        _activeDeployments = activeDeployments;
        _isLoading = false;
      });
    } catch (e) {
      setState(() {
        _error = e.toString();
        _isLoading = false;
      });
    }
  }

  List<Deployment> _generateMockDeployments() {
    final now = DateTime.now();
    return [
      Deployment(
        id: '1',
        projectId: 'proj_123',
        environmentId: 'env_prod',
        releaseId: 'rel_001',
        version: 'v3.2.1',
        strategy: DeployStrategy.rolling,
        status: DeployStatus.running,
        targetPercentage: 100.0,
        currentPercentage: 75.0,
        createdBy: 'github-actions',
        createdAt: now.subtract(const Duration(minutes: 25)).toIso8601String(),
        updatedAt: now.subtract(const Duration(minutes: 2)).toIso8601String(),
        startedAt: now.subtract(const Duration(minutes: 25)).toIso8601String(),
        notes: 'Production deployment with bug fixes',
      ),
      Deployment(
        id: '2',
        projectId: 'proj_123',
        environmentId: 'env_stage',
        releaseId: 'rel_002',
        version: 'v3.2.2',
        strategy: DeployStrategy.canary,
        status: DeployStatus.completed,
        targetPercentage: 20.0,
        currentPercentage: 20.0,
        createdBy: 'jane@example.com',
        createdAt: now.subtract(const Duration(hours: 2)).toIso8601String(),
        updatedAt: now.subtract(const Duration(hours: 1, minutes: 45)).toIso8601String(),
        startedAt: now.subtract(const Duration(hours: 2)).toIso8601String(),
        completedAt: now.subtract(const Duration(hours: 1, minutes: 45)).toIso8601String(),
        notes: 'Canary release for new features',
      ),
      Deployment(
        id: '3',
        projectId: 'proj_123',
        environmentId: 'env_prod',
        releaseId: 'rel_003',
        version: 'v3.2.0',
        strategy: DeployStrategy.blueGreen,
        status: DeployStatus.failed,
        createdBy: 'bob@example.com',
        createdAt: now.subtract(const Duration(hours: 6)).toIso8601String(),
        updatedAt: now.subtract(const Duration(hours: 5, minutes: 30)).toIso8601String(),
        startedAt: now.subtract(const Duration(hours: 6)).toIso8601String(),
        completedAt: now.subtract(const Duration(hours: 5, minutes: 30)).toIso8601String(),
        notes: 'Failed due to health check timeouts',
      ),
      Deployment(
        id: '4',
        projectId: 'proj_123',
        environmentId: 'env_dev',
        releaseId: 'rel_004',
        version: 'v3.1.9',
        strategy: DeployStrategy.rolling,
        status: DeployStatus.completed,
        targetPercentage: 100.0,
        currentPercentage: 100.0,
        createdBy: 'alice@example.com',
        createdAt: now.subtract(const Duration(days: 1)).toIso8601String(),
        updatedAt: now.subtract(const Duration(days: 1)).toIso8601String(),
        startedAt: now.subtract(const Duration(days: 1)).toIso8601String(),
        completedAt: now.subtract(const Duration(days: 1)).toIso8601String(),
        notes: 'Development environment update',
      ),
      Deployment(
        id: '5',
        projectId: 'proj_123',
        environmentId: 'env_stage',
        releaseId: 'rel_005',
        version: 'v3.1.8',
        strategy: DeployStrategy.canary,
        status: DeployStatus.pending,
        targetPercentage: 10.0,
        currentPercentage: 0.0,
        createdBy: 'system',
        createdAt: now.subtract(const Duration(minutes: 5)).toIso8601String(),
        updatedAt: now.subtract(const Duration(minutes: 5)).toIso8601String(),
        notes: 'Scheduled deployment',
      ),
    ];
  }

  List<Deployment> get _filteredDeployments {
    return _deployments.where((deployment) {
      final environmentMatch = _selectedEnvironment == 'All' ||
          _getEnvironmentName(deployment.environmentId) == _selectedEnvironment;

      final statusMatch = _selectedStatus == 'All' ||
          _mapDeployStatus(deployment.status) == _selectedStatus;

      return environmentMatch && statusMatch;
    }).toList();
  }

  String _getEnvironmentName(String environmentId) {
    switch (environmentId) {
      case 'env_prod':
        return 'Production';
      case 'env_stage':
        return 'Staging';
      case 'env_dev':
        return 'Development';
      default:
        return 'Unknown';
    }
  }

  String _mapDeployStatus(DeployStatus status) {
    switch (status) {
      case DeployStatus.pending:
        return 'Pending';
      case DeployStatus.running:
        return 'Running';
      case DeployStatus.completed:
        return 'Completed';
      case DeployStatus.failed:
        return 'Failed';
      case DeployStatus.paused:
        return 'Paused';
      case DeployStatus.rolledBack:
        return 'Rolled Back';
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Deployments'),
        actions: [
          PopupMenuButton<String>(
            icon: const Icon(Icons.filter_list),
            onSelected: (value) {
              if (value == 'environment') {
                _showEnvironmentFilter();
              } else if (value == 'status') {
                _showStatusFilter();
              }
            },
            itemBuilder: (context) => [
              PopupMenuItem<String>(
                value: 'environment',
                child: Row(
                  children: [
                    const Icon(Icons.cloud),
                    const SizedBox(width: 8),
                    Text('Environment: $_selectedEnvironment'),
                  ],
                ),
              ),
              PopupMenuItem<String>(
                value: 'status',
                child: Row(
                  children: [
                    const Icon(Icons.info),
                    const SizedBox(width: 8),
                    Text('Status: $_selectedStatus'),
                  ],
                ),
              ),
            ],
          ),
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: _loadDeployments,
          ),
          IconButton(
            icon: const Icon(Icons.add),
            onPressed: _showCreateDeploymentDialog,
          ),
        ],
        bottom: TabBar(
          controller: _tabController,
          tabs: [
            Tab(
              icon: const Icon(Icons.play_circle),
              text: 'Active (${_activeDeployments.length})',
            ),
            Tab(
              icon: const Icon(Icons.history),
              text: 'History (${_deployments.length})',
            ),
          ],
        ),
      ),
      drawer: const AppDrawer(),
      body: _isLoading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? _buildErrorState()
              : TabBarView(
                  controller: _tabController,
                  children: [
                    _buildActiveDeployments(),
                    _buildDeploymentHistory(),
                  ],
                ),
    );
  }

  Widget _buildErrorState() {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.error_outline, size: 64, color: Colors.red[300]),
          const SizedBox(height: 16),
          Text('Failed to load deployments', style: Theme.of(context).textTheme.headlineSmall),
          const SizedBox(height: 8),
          Text(_error!, style: Theme.of(context).textTheme.bodyMedium),
          const SizedBox(height: 16),
          ElevatedButton(
            onPressed: _loadDeployments,
            child: const Text('Retry'),
          ),
        ],
      ),
    );
  }

  Widget _buildActiveDeployments() {
    if (_activeDeployments.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.check_circle, size: 64, color: Colors.green[300]),
            const SizedBox(height: 16),
            Text(
              'No Active Deployments',
              style: Theme.of(context).textTheme.headlineSmall,
            ),
            const SizedBox(height: 8),
            const Text('All deployments are completed or idle'),
          ],
        ),
      );
    }

    return RefreshIndicator(
      onRefresh: _loadDeployments,
      child: ListView.builder(
        padding: const EdgeInsets.all(16),
        itemCount: _activeDeployments.length,
        itemBuilder: (context, index) {
          final deployment = _activeDeployments[index];
          return _buildActiveDeploymentCard(deployment);
        },
      ),
    );
  }

  Widget _buildActiveDeploymentCard(Deployment deployment) {
    final progress = deployment.currentPercentage ?? 0.0;
    final target = deployment.targetPercentage ?? 100.0;
    final progressRatio = target > 0 ? progress / target : 0.0;

    return Card(
      margin: const EdgeInsets.only(bottom: 16),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                _buildStatusIcon(deployment.status),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        deployment.version,
                        style: Theme.of(context).textTheme.titleLarge?.copyWith(
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                      Text(
                        '${_getEnvironmentName(deployment.environmentId)} • ${deployment.strategy.toString().split('.').last}',
                        style: Theme.of(context).textTheme.bodyMedium,
                      ),
                    ],
                  ),
                ),
                _buildActionButton(deployment),
              ],
            ),
            const SizedBox(height: 16),

            if (deployment.status == DeployStatus.running) ...[
              Text(
                'Progress: ${progress.toStringAsFixed(1)}% of ${target.toStringAsFixed(0)}%',
                style: Theme.of(context).textTheme.bodyMedium,
              ),
              const SizedBox(height: 8),
              ClipRRect(
                borderRadius: BorderRadius.circular(4),
                child: LinearProgressIndicator(
                  value: progressRatio,
                  backgroundColor: Colors.grey[300],
                  valueColor: AlwaysStoppedAnimation<Color>(
                    deployment.status == DeployStatus.failed ? Colors.red :
                    progressRatio >= 1.0 ? Colors.green : Colors.blue,
                  ),
                  minHeight: 8,
                ),
              ),
              const SizedBox(height: 12),
            ],

            Row(
              children: [
                Expanded(
                  child: _buildDeploymentInfo('Started', _formatTime(deployment.startedAt ?? deployment.createdAt)),
                ),
                Expanded(
                  child: _buildDeploymentInfo('Duration', _formatDuration(deployment.startedAt ?? deployment.createdAt)),
                ),
                Expanded(
                  child: _buildDeploymentInfo('Triggered By', deployment.createdBy),
                ),
              ],
            ),

            if (deployment.notes != null && deployment.notes!.isNotEmpty) ...[
              const SizedBox(height: 12),
              Container(
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: Colors.grey[100],
                  borderRadius: BorderRadius.circular(8),
                ),
                width: double.infinity,
                child: Text(
                  deployment.notes!,
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildDeploymentHistory() {
    final filtered = _filteredDeployments;

    if (filtered.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.history, size: 64, color: Colors.grey[400]),
            const SizedBox(height: 16),
            Text(
              'No Deployments Found',
              style: Theme.of(context).textTheme.headlineSmall,
            ),
            const SizedBox(height: 8),
            const Text('Try adjusting your filters'),
          ],
        ),
      );
    }

    return RefreshIndicator(
      onRefresh: _loadDeployments,
      child: ListView.builder(
        padding: const EdgeInsets.all(16),
        itemCount: filtered.length,
        itemBuilder: (context, index) {
          final deployment = filtered[index];
          return _buildDeploymentHistoryCard(deployment);
        },
      ),
    );
  }

  Widget _buildDeploymentHistoryCard(Deployment deployment) {
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: ListTile(
        leading: _buildStatusIcon(deployment.status),
        title: Text(
          deployment.version,
          style: Theme.of(context).textTheme.titleMedium?.copyWith(
            fontWeight: FontWeight.bold,
          ),
        ),
        subtitle: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('${_getEnvironmentName(deployment.environmentId)} • ${deployment.strategy.toString().split('.').last}'),
            Text('${deployment.createdBy} • ${_formatTime(deployment.createdAt)}'),
          ],
        ),
        trailing: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
              decoration: BoxDecoration(
                color: _getStatusColor(deployment.status),
                borderRadius: BorderRadius.circular(12),
              ),
              child: Text(
                _mapDeployStatus(deployment.status),
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 12,
                  fontWeight: FontWeight.bold,
                ),
              ),
            ),
            const SizedBox(height: 4),
            if (deployment.completedAt != null)
              Text(
                _formatDuration(deployment.startedAt ?? deployment.createdAt, deployment.completedAt!),
                style: Theme.of(context).textTheme.bodySmall,
              ),
          ],
        ),
        onTap: () => _showDeploymentDetails(deployment),
      ),
    );
  }

  Widget _buildStatusIcon(DeployStatus status) {
    switch (status) {
      case DeployStatus.pending:
        return const Icon(Icons.schedule, color: Colors.orange);
      case DeployStatus.running:
        return const Icon(Icons.play_circle, color: Colors.blue);
      case DeployStatus.completed:
        return const Icon(Icons.check_circle, color: Colors.green);
      case DeployStatus.failed:
        return const Icon(Icons.error, color: Colors.red);
      case DeployStatus.paused:
        return const Icon(Icons.pause_circle, color: Colors.orange);
      case DeployStatus.rolledBack:
        return const Icon(Icons.undo, color: Colors.purple);
    }
  }

  Color _getStatusColor(DeployStatus status) {
    switch (status) {
      case DeployStatus.pending:
        return Colors.orange;
      case DeployStatus.running:
        return Colors.blue;
      case DeployStatus.completed:
        return Colors.green;
      case DeployStatus.failed:
        return Colors.red;
      case DeployStatus.paused:
        return Colors.orange;
      case DeployStatus.rolledBack:
        return Colors.purple;
    }
  }

  Widget _buildActionButton(Deployment deployment) {
    switch (deployment.status) {
      case DeployStatus.running:
        return IconButton(
          icon: const Icon(Icons.pause),
          onPressed: () => _pauseDeployment(deployment),
        );
      case DeployStatus.paused:
        return IconButton(
          icon: const Icon(Icons.play_arrow),
          onPressed: () => _resumeDeployment(deployment),
        );
      case DeployStatus.failed:
        return IconButton(
          icon: const Icon(Icons.refresh),
          onPressed: () => _retryDeployment(deployment),
        );
      default:
        return const SizedBox.shrink();
    }
  }

  Widget _buildDeploymentInfo(String label, String value) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          value,
          style: Theme.of(context).textTheme.titleSmall?.copyWith(
            fontWeight: FontWeight.bold,
          ),
        ),
        Text(
          label,
          style: Theme.of(context).textTheme.bodySmall?.copyWith(
            color: Colors.grey[600],
          ),
        ),
      ],
    );
  }

  String _formatTime(String dateTimeString) {
    final dateTime = DateTime.parse(dateTimeString);
    final now = DateTime.now();
    final difference = now.difference(dateTime);

    if (difference.inMinutes < 60) {
      return '${difference.inMinutes}m ago';
    } else if (difference.inHours < 24) {
      return '${difference.inHours}h ago';
    } else {
      return DateFormat('MMM d, HH:mm').format(dateTime);
    }
  }

  String _formatDuration(String startTime, [String? endTime]) {
    final start = DateTime.parse(startTime);
    final end = endTime != null ? DateTime.parse(endTime) : DateTime.now();
    final duration = end.difference(start);

    if (duration.inHours > 0) {
      return '${duration.inHours}h ${duration.inMinutes % 60}m';
    } else {
      return '${duration.inMinutes}m';
    }
  }

  void _showEnvironmentFilter() {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Filter by Environment'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: _environments.map((env) => RadioListTile<String>(
            title: Text(env),
            value: env,
            groupValue: _selectedEnvironment,
            onChanged: (value) {
              setState(() => _selectedEnvironment = value!);
              Navigator.of(context).pop();
            },
          )).toList(),
        ),
      ),
    );
  }

  void _showStatusFilter() {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Filter by Status'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: _statuses.map((status) => RadioListTile<String>(
            title: Text(status),
            value: status,
            groupValue: _selectedStatus,
            onChanged: (value) {
              setState(() => _selectedStatus = value!);
              Navigator.of(context).pop();
            },
          )).toList(),
        ),
      ),
    );
  }

  void _showCreateDeploymentDialog() {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Create Deployment'),
        content: const Text('Deployment creation dialog would go here.'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('Cancel'),
          ),
          ElevatedButton(
            onPressed: () {
              Navigator.of(context).pop();
              // TODO: Implement deployment creation
            },
            child: const Text('Create'),
          ),
        ],
      ),
    );
  }

  void _showDeploymentDetails(Deployment deployment) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (context) => DraggableScrollableSheet(
        initialChildSize: 0.6,
        maxChildSize: 0.9,
        minChildSize: 0.3,
        builder: (context, scrollController) => Container(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  _buildStatusIcon(deployment.status),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Text(
                      deployment.version,
                      style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                  ),
                  IconButton(
                    onPressed: () => Navigator.of(context).pop(),
                    icon: const Icon(Icons.close),
                  ),
                ],
              ),
              const SizedBox(height: 16),
              Expanded(
                child: ListView(
                  controller: scrollController,
                  children: [
                    _buildDetailRow('Environment', _getEnvironmentName(deployment.environmentId)),
                    _buildDetailRow('Strategy', deployment.strategy.toString().split('.').last),
                    _buildDetailRow('Status', _mapDeployStatus(deployment.status)),
                    _buildDetailRow('Triggered By', deployment.createdBy),
                    _buildDetailRow('Created', _formatTime(deployment.createdAt)),
                    if (deployment.startedAt != null)
                      _buildDetailRow('Started', _formatTime(deployment.startedAt!)),
                    if (deployment.completedAt != null)
                      _buildDetailRow('Completed', _formatTime(deployment.completedAt!)),
                    if (deployment.targetPercentage != null)
                      _buildDetailRow('Target %', '${deployment.targetPercentage!.toStringAsFixed(1)}%'),
                    if (deployment.currentPercentage != null)
                      _buildDetailRow('Current %', '${deployment.currentPercentage!.toStringAsFixed(1)}%'),
                    if (deployment.notes != null && deployment.notes!.isNotEmpty) ...[
                      const SizedBox(height: 16),
                      Text(
                        'Notes',
                        style: Theme.of(context).textTheme.titleMedium?.copyWith(
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                      const SizedBox(height: 8),
                      Container(
                        padding: const EdgeInsets.all(12),
                        decoration: BoxDecoration(
                          color: Colors.grey[100],
                          borderRadius: BorderRadius.circular(8),
                        ),
                        width: double.infinity,
                        child: Text(deployment.notes!),
                      ),
                    ],
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildDetailRow(String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 100,
            child: Text(
              label,
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                fontWeight: FontWeight.bold,
              ),
            ),
          ),
          Expanded(
            child: Text(value),
          ),
        ],
      ),
    );
  }

  void _pauseDeployment(Deployment deployment) {
    // TODO: Implement pause functionality
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('Pausing deployment ${deployment.version}')),
    );
  }

  void _resumeDeployment(Deployment deployment) {
    // TODO: Implement resume functionality
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('Resuming deployment ${deployment.version}')),
    );
  }

  void _retryDeployment(Deployment deployment) {
    // TODO: Implement retry functionality
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('Retrying deployment ${deployment.version}')),
    );
  }
}