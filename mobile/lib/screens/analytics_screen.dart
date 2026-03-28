import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import '../widgets/app_shell.dart';
import '../services/api_client.dart';
import '../models/analytics.dart';

class AnalyticsScreen extends StatefulWidget {
  const AnalyticsScreen({super.key});

  @override
  State<AnalyticsScreen> createState() => _AnalyticsScreenState();
}

class _AnalyticsScreenState extends State<AnalyticsScreen>
    with TickerProviderStateMixin {
  late TabController _tabController;

  AnalyticsSummary? _summary;
  List<FlagStats> _flagStats = [];
  SystemHealth? _systemHealth;
  bool _isLoading = true;
  String _selectedTimeRange = '24h';
  String _selectedProject = 'proj_123';
  String _selectedEnvironment = 'env_456';

  final List<String> _timeRanges = ['1h', '6h', '24h', '7d', '30d'];
  final Map<String, String> _timeRangeLabels = {
    '1h': 'Last Hour',
    '6h': 'Last 6 Hours',
    '24h': 'Last 24 Hours',
    '7d': 'Last 7 Days',
    '30d': 'Last 30 Days',
  };

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 3, vsync: this);
    _loadAnalyticsData();
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  Future<void> _loadAnalyticsData() async {
    setState(() => _isLoading = true);

    try {
      final summaryFuture = apiClient.getAnalyticsSummary(
        projectId: _selectedProject,
        environmentId: _selectedEnvironment,
        timeRange: _selectedTimeRange,
      );

      final flagStatsFuture = apiClient.getFlagStats(
        projectId: _selectedProject,
        environmentId: _selectedEnvironment,
        timeRange: _selectedTimeRange,
        limit: 20,
      );

      final healthFuture = apiClient.getSystemHealth();

      final results = await Future.wait([
        summaryFuture,
        flagStatsFuture,
        healthFuture,
      ]);

      setState(() {
        _summary = results[0] as AnalyticsSummary;
        _flagStats = results[1] as List<FlagStats>;
        _systemHealth = results[2] as SystemHealth;
        _isLoading = false;
      });
    } catch (e) {
      setState(() => _isLoading = false);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Failed to load analytics: $e'),
            backgroundColor: Colors.red,
          ),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Analytics'),
        actions: [
          PopupMenuButton<String>(
            icon: const Icon(Icons.access_time),
            tooltip: 'Time Range',
            onSelected: (value) {
              setState(() => _selectedTimeRange = value);
              _loadAnalyticsData();
            },
            itemBuilder: (context) => _timeRanges.map((range) {
              return PopupMenuItem<String>(
                value: range,
                child: Row(
                  children: [
                    Icon(
                      range == _selectedTimeRange ? Icons.radio_button_checked : Icons.radio_button_unchecked,
                      size: 16,
                    ),
                    const SizedBox(width: 8),
                    Text(_timeRangeLabels[range]!),
                  ],
                ),
              );
            }).toList(),
          ),
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: _loadAnalyticsData,
          ),
        ],
        bottom: TabBar(
          controller: _tabController,
          tabs: const [
            Tab(icon: Icon(Icons.dashboard), text: 'Overview'),
            Tab(icon: Icon(Icons.flag), text: 'Flags'),
            Tab(icon: Icon(Icons.monitor_heart), text: 'Health'),
          ],
        ),
      ),
      drawer: const AppDrawer(),
      body: _isLoading
          ? const Center(child: CircularProgressIndicator())
          : TabBarView(
              controller: _tabController,
              children: [
                _buildOverviewTab(),
                _buildFlagsTab(),
                _buildHealthTab(),
              ],
            ),
    );
  }

  Widget _buildOverviewTab() {
    if (_summary == null) return const Center(child: Text('No data available'));

    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'System Overview',
            style: Theme.of(context).textTheme.headlineSmall,
          ),
          const SizedBox(height: 16),

          // Key Metrics Grid
          _buildMetricsGrid(),
          const SizedBox(height: 24),

          Text(
            'Performance Charts',
            style: Theme.of(context).textTheme.titleLarge,
          ),
          const SizedBox(height: 16),

          // Performance Charts
          _buildPerformanceCharts(),
        ],
      ),
    );
  }

  Widget _buildMetricsGrid() {
    final summary = _summary!;

    return GridView.count(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      crossAxisCount: 2,
      mainAxisSpacing: 16,
      crossAxisSpacing: 16,
      childAspectRatio: 1.2,
      children: [
        _buildMetricCard(
          'Flag Evaluations',
          _formatNumber(summary.flags.totalEvaluations),
          Icons.flag,
          Colors.blue,
          subtitle: 'Total evaluations',
        ),
        _buildMetricCard(
          'Active Flags',
          summary.flags.activeFlags.toString(),
          Icons.toggle_on,
          Colors.green,
          subtitle: 'Currently enabled',
        ),
        _buildMetricCard(
          'Deployments',
          _formatNumber(summary.deployments.totalDeployments),
          Icons.rocket_launch,
          Colors.purple,
          subtitle: '${summary.deployments.successRate.toStringAsFixed(1)}% success',
        ),
        _buildMetricCard(
          'API Requests',
          _formatNumber(summary.api.totalRequests),
          Icons.api,
          Colors.orange,
          subtitle: '${summary.api.requestsPerSecond.toStringAsFixed(1)} req/s',
        ),
      ],
    );
  }

  Widget _buildMetricCard(String title, String value, IconData icon, Color color, {String? subtitle}) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(icon, color: color, size: 24),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    title,
                    style: Theme.of(context).textTheme.labelMedium,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
            Text(
              value,
              style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                fontWeight: FontWeight.bold,
                color: color,
              ),
            ),
            if (subtitle != null)
              Text(
                subtitle,
                style: Theme.of(context).textTheme.bodySmall?.copyWith(
                  color: Colors.grey[600],
                ),
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildPerformanceCharts() {
    final summary = _summary!;

    return Column(
      children: [
        // Latency Chart
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Latency Performance',
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 16),
                Row(
                  children: [
                    Expanded(
                      child: _buildLatencyBar(
                        'Flag Evaluation',
                        summary.flags.averageLatencyMs,
                        Colors.blue,
                        100, // max expected latency
                      ),
                    ),
                    const SizedBox(width: 16),
                    Expanded(
                      child: _buildLatencyBar(
                        'API Response',
                        summary.api.averageLatencyMs,
                        Colors.orange,
                        500, // max expected latency
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 16),

        // Cache Performance Chart
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Cache Performance',
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 16),
                _buildCacheChart(summary.flags.cacheHitRate),
              ],
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildLatencyBar(String label, double latencyMs, Color color, double maxLatency) {
    final percentage = (latencyMs / maxLatency).clamp(0.0, 1.0);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(
              label,
              style: Theme.of(context).textTheme.bodyMedium,
            ),
            Text(
              '${latencyMs.toStringAsFixed(1)}ms',
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                fontWeight: FontWeight.bold,
                color: color,
              ),
            ),
          ],
        ),
        const SizedBox(height: 8),
        ClipRRect(
          borderRadius: BorderRadius.circular(4),
          child: LinearProgressIndicator(
            value: percentage,
            backgroundColor: Colors.grey[300],
            valueColor: AlwaysStoppedAnimation<Color>(color),
            minHeight: 8,
          ),
        ),
      ],
    );
  }

  Widget _buildCacheChart(double cacheHitRate) {
    final percentage = cacheHitRate / 100.0;

    return Row(
      children: [
        Expanded(
          flex: 3,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                'Cache Hit Rate',
                style: Theme.of(context).textTheme.bodyLarge,
              ),
              const SizedBox(height: 8),
              Text(
                '${cacheHitRate.toStringAsFixed(1)}%',
                style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                  fontWeight: FontWeight.bold,
                  color: percentage > 0.8 ? Colors.green :
                         percentage > 0.6 ? Colors.orange : Colors.red,
                ),
              ),
            ],
          ),
        ),
        Expanded(
          flex: 2,
          child: SizedBox(
            height: 80,
            width: 80,
            child: Stack(
              children: [
                CircularProgressIndicator(
                  value: percentage,
                  strokeWidth: 8,
                  backgroundColor: Colors.grey[300],
                  valueColor: AlwaysStoppedAnimation<Color>(
                    percentage > 0.8 ? Colors.green :
                    percentage > 0.6 ? Colors.orange : Colors.red,
                  ),
                ),
                Center(
                  child: Text(
                    '${(percentage * 100).toInt()}%',
                    style: Theme.of(context).textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildFlagsTab() {
    if (_flagStats.isEmpty) {
      return const Center(child: Text('No flag statistics available'));
    }

    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        Text(
          'Flag Performance',
          style: Theme.of(context).textTheme.headlineSmall,
        ),
        const SizedBox(height: 16),

        ..._flagStats.map((stat) => _buildFlagStatCard(stat)),
      ],
    );
  }

  Widget _buildFlagStatCard(FlagStats stat) {
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(Icons.flag, color: Colors.blue, size: 20),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    stat.flagKey,
                    style: Theme.of(context).textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                  decoration: BoxDecoration(
                    color: stat.errorRate > 0.05 ? Colors.red[100] : Colors.green[100],
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Text(
                    stat.errorRate > 0.05 ? 'HIGH ERROR' : 'HEALTHY',
                    style: TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.bold,
                      color: stat.errorRate > 0.05 ? Colors.red[700] : Colors.green[700],
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),

            // Metrics Grid
            Row(
              children: [
                Expanded(
                  child: _buildStatMetric('Evaluations', _formatNumber(stat.totalEvaluations)),
                ),
                Expanded(
                  child: _buildStatMetric('Latency', '${stat.averageLatencyMs.toStringAsFixed(1)}ms'),
                ),
                Expanded(
                  child: _buildStatMetric('Cache Hit', '${stat.cacheHitRate.toStringAsFixed(1)}%'),
                ),
                Expanded(
                  child: _buildStatMetric('Error Rate', '${(stat.errorRate * 100).toStringAsFixed(2)}%'),
                ),
              ],
            ),

            if (stat.resultDistribution.isNotEmpty) ...[
              const SizedBox(height: 16),
              Text(
                'Result Distribution',
                style: Theme.of(context).textTheme.titleSmall,
              ),
              const SizedBox(height: 8),
              _buildResultDistribution(stat.resultDistribution),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildStatMetric(String label, String value) {
    return Column(
      children: [
        Text(
          value,
          style: Theme.of(context).textTheme.titleMedium?.copyWith(
            fontWeight: FontWeight.bold,
            color: Colors.blue,
          ),
        ),
        Text(
          label,
          style: Theme.of(context).textTheme.bodySmall?.copyWith(
            color: Colors.grey[600],
          ),
          textAlign: TextAlign.center,
        ),
      ],
    );
  }

  Widget _buildResultDistribution(Map<String, int> distribution) {
    final total = distribution.values.fold<int>(0, (sum, count) => sum + count);
    if (total == 0) return const Text('No data');

    return Column(
      children: distribution.entries.map((entry) {
        final percentage = (entry.value / total);
        return Padding(
          padding: const EdgeInsets.only(bottom: 4),
          child: Row(
            children: [
              SizedBox(
                width: 80,
                child: Text(
                  entry.key,
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ),
              Expanded(
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(2),
                  child: LinearProgressIndicator(
                    value: percentage,
                    backgroundColor: Colors.grey[300],
                    valueColor: AlwaysStoppedAnimation<Color>(
                      entry.key.toLowerCase() == 'true' ? Colors.green : Colors.blue,
                    ),
                    minHeight: 6,
                  ),
                ),
              ),
              SizedBox(
                width: 50,
                child: Text(
                  '${(percentage * 100).toInt()}%',
                  style: Theme.of(context).textTheme.bodySmall,
                  textAlign: TextAlign.right,
                ),
              ),
            ],
          ),
        );
      }).toList(),
    );
  }

  Widget _buildHealthTab() {
    if (_systemHealth == null) {
      return const Center(child: Text('No health data available'));
    }

    final health = _systemHealth!;

    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        Text(
          'System Health',
          style: Theme.of(context).textTheme.headlineSmall,
        ),
        const SizedBox(height: 8),
        Text(
          'Last updated: ${DateFormat('MMM d, y HH:mm').format(DateTime.parse(health.timestamp))}',
          style: Theme.of(context).textTheme.bodySmall?.copyWith(
            color: Colors.grey[600],
          ),
        ),
        const SizedBox(height: 24),

        // API Health
        _buildHealthCard(
          'API Performance',
          Icons.api,
          Colors.blue,
          [
            _buildHealthMetric('Requests/sec', health.api.requestsPerSecond.toStringAsFixed(1)),
            _buildHealthMetric('Avg Latency', '${health.api.avgLatencyMs.toStringAsFixed(1)}ms'),
            _buildHealthMetric('Error Rate', '${(health.api.errorRate * 100).toStringAsFixed(2)}%'),
            _buildHealthMetric('Connections', health.api.activeConnections.toString()),
          ],
        ),

        // Database Health
        _buildHealthCard(
          'Database Performance',
          Icons.storage,
          Colors.green,
          [
            _buildHealthMetric('Connections', health.database.connections.toString()),
            _buildHealthMetric('Query Latency', '${health.database.queryLatencyMs.toStringAsFixed(1)}ms'),
            _buildHealthMetric('Cache Hit Rate', '${health.database.cacheHitRate.toStringAsFixed(1)}%'),
          ],
        ),

        // Resource Usage
        _buildResourceHealthCard(health.resources),
      ],
    );
  }

  Widget _buildHealthCard(String title, IconData icon, Color color, List<Widget> metrics) {
    return Card(
      margin: const EdgeInsets.only(bottom: 16),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(icon, color: color, size: 24),
                const SizedBox(width: 8),
                Text(
                  title,
                  style: Theme.of(context).textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 16),
            GridView.count(
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              crossAxisCount: 2,
              childAspectRatio: 3,
              mainAxisSpacing: 8,
              crossAxisSpacing: 16,
              children: metrics,
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildHealthMetric(String label, String value) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          value,
          style: Theme.of(context).textTheme.titleMedium?.copyWith(
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

  Widget _buildResourceHealthCard(ResourceMetrics resources) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(Icons.monitor, color: Colors.orange, size: 24),
                const SizedBox(width: 8),
                Text(
                  'Resource Usage',
                  style: Theme.of(context).textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 16),

            _buildResourceBar('CPU', resources.cpuUsagePercent, Colors.red),
            const SizedBox(height: 12),
            _buildResourceBar('Memory', resources.memoryUsagePercent, Colors.orange),
            const SizedBox(height: 12),
            _buildResourceBar('Disk', resources.diskUsagePercent, Colors.yellow[700]!),
            const SizedBox(height: 8),

            Text(
              'Memory: ${_formatBytes(resources.memoryUsageBytes)}',
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                color: Colors.grey[600],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildResourceBar(String label, double percentage, Color color) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(label),
            Text(
              '${percentage.toStringAsFixed(1)}%',
              style: TextStyle(
                fontWeight: FontWeight.bold,
                color: percentage > 80 ? Colors.red :
                       percentage > 60 ? Colors.orange : Colors.green,
              ),
            ),
          ],
        ),
        const SizedBox(height: 4),
        ClipRRect(
          borderRadius: BorderRadius.circular(4),
          child: LinearProgressIndicator(
            value: percentage / 100,
            backgroundColor: Colors.grey[300],
            valueColor: AlwaysStoppedAnimation<Color>(
              percentage > 80 ? Colors.red :
              percentage > 60 ? Colors.orange : color,
            ),
            minHeight: 6,
          ),
        ),
      ],
    );
  }

  String _formatNumber(int number) {
    if (number >= 1000000) {
      return '${(number / 1000000).toStringAsFixed(1)}M';
    } else if (number >= 1000) {
      return '${(number / 1000).toStringAsFixed(1)}K';
    }
    return number.toString();
  }

  String _formatBytes(int bytes) {
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    double value = bytes.toDouble();
    int unitIndex = 0;

    while (value >= 1024 && unitIndex < units.length - 1) {
      value /= 1024;
      unitIndex++;
    }

    return '${value.toStringAsFixed(1)} ${units[unitIndex]}';
  }
}