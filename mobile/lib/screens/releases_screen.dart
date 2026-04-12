import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:intl/intl.dart';
import 'package:url_launcher/url_launcher.dart';
import '../widgets/app_shell.dart';
import '../services/api_client.dart';
import '../models/release.dart';

class ReleasesScreen extends StatefulWidget {
  const ReleasesScreen({super.key});

  @override
  State<ReleasesScreen> createState() => _ReleasesScreenState();
}

class _ReleasesScreenState extends State<ReleasesScreen>
    with TickerProviderStateMixin {
  late TabController _tabController;

  List<Release> _releases = [];
  bool _isLoading = true;
  String? _error;
  String _selectedStatus = 'All';

  final List<String> _statuses = ['All', 'Draft', 'Staging', 'Canary', 'Production', 'Archived'];

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
    _loadReleases();
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  Future<void> _loadReleases() async {
    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      List<Release> releases;
      try {
        releases = await apiClient.getReleases(
          projectId: 'proj_123',
          limit: 50,
        );
      } catch (e) {
        // Mock data fallback
        await Future.delayed(const Duration(milliseconds: 500));
        releases = _generateMockReleases();
      }

      setState(() {
        _releases = releases;
        _isLoading = false;
      });
    } catch (e) {
      setState(() {
        _error = e.toString();
        _isLoading = false;
      });
    }
  }

  List<Release> _generateMockReleases() {
    final now = DateTime.now();
    return [
      Release(
        id: '1',
        projectId: 'proj_123',
        version: 'v3.2.1',
        description: 'Critical bug fixes for payment processing and improved user authentication',
        commitSha: 'abc123def456',
        status: ReleaseStatus.production,
        artifactUrl: 'https://artifacts.deploysentry.com/v3.2.1.tar.gz',
        changelogUrl: 'https://github.com/example/repo/releases/tag/v3.2.1',
        metadata: {
          'build_number': '1024',
          'test_coverage': '94.2%',
          'security_scan': 'passed'
        },
        tags: ['hotfix', 'payment', 'security'],
        createdBy: 'release-bot',
        createdAt: now.subtract(const Duration(days: 2)).toIso8601String(),
        updatedAt: now.subtract(const Duration(hours: 6)).toIso8601String(),
      ),
      Release(
        id: '2',
        projectId: 'proj_123',
        version: 'v3.3.0',
        description: 'New dashboard features, mobile app improvements, and analytics enhancements',
        commitSha: 'def456ghi789',
        status: ReleaseStatus.canary,
        artifactUrl: 'https://artifacts.deploysentry.com/v3.3.0.tar.gz',
        changelogUrl: 'https://github.com/example/repo/releases/tag/v3.3.0',
        metadata: {
          'build_number': '1025',
          'test_coverage': '95.8%',
          'performance_score': '98/100'
        },
        tags: ['feature', 'dashboard', 'mobile', 'analytics'],
        createdBy: 'jane@example.com',
        createdAt: now.subtract(const Duration(hours: 12)).toIso8601String(),
        updatedAt: now.subtract(const Duration(minutes: 30)).toIso8601String(),
      ),
      Release(
        id: '3',
        projectId: 'proj_123',
        version: 'v3.3.1',
        description: 'Performance optimizations and bug fixes based on canary feedback',
        commitSha: 'ghi789jkl012',
        status: ReleaseStatus.staging,
        artifactUrl: 'https://artifacts.deploysentry.com/v3.3.1.tar.gz',
        metadata: {
          'build_number': '1026',
          'test_coverage': '96.1%'
        },
        tags: ['performance', 'bugfix'],
        createdBy: 'bob@example.com',
        createdAt: now.subtract(const Duration(hours: 4)).toIso8601String(),
        updatedAt: now.subtract(const Duration(minutes: 15)).toIso8601String(),
      ),
      Release(
        id: '4',
        projectId: 'proj_123',
        version: 'v3.4.0-beta',
        description: 'Beta release with experimental AI features and new user interface',
        commitSha: 'jkl012mno345',
        status: ReleaseStatus.draft,
        metadata: {
          'build_number': '1027',
          'experimental': true
        },
        tags: ['beta', 'ai', 'ui', 'experimental'],
        createdBy: 'alice@example.com',
        createdAt: now.subtract(const Duration(hours: 1)).toIso8601String(),
        updatedAt: now.subtract(const Duration(minutes: 5)).toIso8601String(),
      ),
      Release(
        id: '5',
        projectId: 'proj_123',
        version: 'v3.1.9',
        description: 'Previous stable release - now archived',
        commitSha: 'mno345pqr678',
        status: ReleaseStatus.archived,
        artifactUrl: 'https://artifacts.deploysentry.com/v3.1.9.tar.gz',
        changelogUrl: 'https://github.com/example/repo/releases/tag/v3.1.9',
        metadata: {
          'build_number': '1020'
        },
        tags: ['stable', 'archived'],
        createdBy: 'system',
        createdAt: now.subtract(const Duration(days: 30)).toIso8601String(),
        updatedAt: now.subtract(const Duration(days: 7)).toIso8601String(),
      ),
    ];
  }

  List<Release> get _filteredReleases {
    if (_selectedStatus == 'All') return _releases;

    final status = _parseReleaseStatus(_selectedStatus);
    return _releases.where((release) => release.status == status).toList();
  }

  ReleaseStatus? _parseReleaseStatus(String status) {
    switch (status.toLowerCase()) {
      case 'draft':
        return ReleaseStatus.draft;
      case 'staging':
        return ReleaseStatus.staging;
      case 'canary':
        return ReleaseStatus.canary;
      case 'production':
        return ReleaseStatus.production;
      case 'archived':
        return ReleaseStatus.archived;
      default:
        return null;
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Releases'),
        actions: [
          PopupMenuButton<String>(
            icon: const Icon(Icons.filter_list),
            onSelected: (value) {
              setState(() => _selectedStatus = value);
            },
            itemBuilder: (context) => _statuses.map((status) => PopupMenuItem<String>(
              value: status,
              child: Row(
                children: [
                  Icon(
                    status == _selectedStatus ? Icons.radio_button_checked : Icons.radio_button_unchecked,
                    size: 16,
                  ),
                  const SizedBox(width: 8),
                  Text(status),
                ],
              ),
            )).toList(),
          ),
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: _loadReleases,
          ),
          IconButton(
            icon: const Icon(Icons.add),
            onPressed: _showCreateReleaseDialog,
          ),
        ],
        bottom: TabBar(
          controller: _tabController,
          tabs: const [
            Tab(icon: Icon(Icons.timeline), text: 'Pipeline'),
            Tab(icon: Icon(Icons.list), text: 'All Releases'),
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
                    _buildPipelineView(),
                    _buildReleasesListView(),
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
          Text('Failed to load releases', style: Theme.of(context).textTheme.headlineSmall),
          const SizedBox(height: 8),
          Text(_error!, style: Theme.of(context).textTheme.bodyMedium),
          const SizedBox(height: 16),
          ElevatedButton(
            onPressed: _loadReleases,
            child: const Text('Retry'),
          ),
        ],
      ),
    );
  }

  Widget _buildPipelineView() {
    // Group releases by status for pipeline visualization
    final pipeline = <ReleaseStatus, List<Release>>{};
    for (final release in _releases) {
      pipeline.putIfAbsent(release.status, () => []).add(release);
    }

    return RefreshIndicator(
      onRefresh: _loadReleases,
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Release Pipeline',
              style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                fontWeight: FontWeight.bold,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              'Track releases through their lifecycle from draft to production',
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                color: Colors.grey[600],
              ),
            ),
            const SizedBox(height: 24),

            _buildPipelineStage('Draft', ReleaseStatus.draft, pipeline[ReleaseStatus.draft] ?? [], Colors.grey),
            _buildPipelineArrow(),
            _buildPipelineStage('Staging', ReleaseStatus.staging, pipeline[ReleaseStatus.staging] ?? [], Colors.blue),
            _buildPipelineArrow(),
            _buildPipelineStage('Canary', ReleaseStatus.canary, pipeline[ReleaseStatus.canary] ?? [], Colors.orange),
            _buildPipelineArrow(),
            _buildPipelineStage('Production', ReleaseStatus.production, pipeline[ReleaseStatus.production] ?? [], Colors.green),

            if (pipeline[ReleaseStatus.archived]?.isNotEmpty == true) ...[
              const SizedBox(height: 32),
              _buildPipelineStage('Archived', ReleaseStatus.archived, pipeline[ReleaseStatus.archived] ?? [], Colors.purple),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildPipelineStage(String title, ReleaseStatus status, List<Release> releases, Color color) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 12,
                  height: 12,
                  decoration: BoxDecoration(
                    color: color,
                    shape: BoxShape.circle,
                  ),
                ),
                const SizedBox(width: 12),
                Text(
                  title,
                  style: Theme.of(context).textTheme.titleLarge?.copyWith(
                    fontWeight: FontWeight.bold,
                    color: color,
                  ),
                ),
                const SizedBox(width: 8),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                  decoration: BoxDecoration(
                    color: color.withOpacity(0.1),
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Text(
                    '${releases.length}',
                    style: TextStyle(
                      color: color,
                      fontWeight: FontWeight.bold,
                      fontSize: 12,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),

            if (releases.isEmpty)
              Container(
                padding: const EdgeInsets.all(24),
                child: Center(
                  child: Text(
                    'No releases in $title',
                    style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                      color: Colors.grey[600],
                    ),
                  ),
                ),
              )
            else
              ...releases.map((release) => _buildPipelineReleaseCard(release, color)),
          ],
        ),
      ),
    );
  }

  Widget _buildPipelineArrow() {
    return Container(
      height: 24,
      child: Center(
        child: Icon(
          Icons.keyboard_arrow_down,
          size: 32,
          color: Colors.grey[400],
        ),
      ),
    );
  }

  Widget _buildPipelineReleaseCard(Release release, Color accentColor) {
    return Container(
      margin: const EdgeInsets.only(bottom: 8),
      child: Card(
        elevation: 1,
        child: ListTile(
          title: Text(
            release.version,
            style: Theme.of(context).textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.bold,
            ),
          ),
          subtitle: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (release.description != null)
                Text(
                  release.description!,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              const SizedBox(height: 4),
              Text(
                'Created by ${release.createdBy} • ${_formatTime(release.createdAt)}',
                style: Theme.of(context).textTheme.bodySmall?.copyWith(
                  color: Colors.grey[600],
                ),
              ),
            ],
          ),
          trailing: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              if (_canPromote(release))
                IconButton(
                  icon: const Icon(Icons.arrow_upward),
                  onPressed: () => _promoteRelease(release),
                  tooltip: 'Promote',
                ),
              PopupMenuButton<String>(
                onSelected: (action) => _handleReleaseAction(release, action),
                itemBuilder: (context) => [
                  PopupMenuItem<String>(
                    value: 'view',
                    child: Row(
                      children: const [
                        Icon(Icons.visibility),
                        SizedBox(width: 8),
                        Text('View Details'),
                      ],
                    ),
                  ),
                  if (release.artifactUrl != null)
                    PopupMenuItem<String>(
                      value: 'download',
                      child: Row(
                        children: const [
                          Icon(Icons.download),
                          SizedBox(width: 8),
                          Text('Download'),
                        ],
                      ),
                    ),
                  if (release.changelogUrl != null)
                    PopupMenuItem<String>(
                      value: 'changelog',
                      child: Row(
                        children: const [
                          Icon(Icons.description),
                          SizedBox(width: 8),
                          Text('Changelog'),
                        ],
                      ),
                    ),
                  if (release.commitSha != null)
                    PopupMenuItem<String>(
                      value: 'commit',
                      child: Row(
                        children: const [
                          Icon(Icons.commit),
                          SizedBox(width: 8),
                          Text('View Commit'),
                        ],
                      ),
                    ),
                ],
              ),
            ],
          ),
          onTap: () => _showReleaseDetails(release),
        ),
      ),
    );
  }

  Widget _buildReleasesListView() {
    final filtered = _filteredReleases;

    if (filtered.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.inbox, size: 64, color: Colors.grey[400]),
            const SizedBox(height: 16),
            Text(
              'No Releases Found',
              style: Theme.of(context).textTheme.headlineSmall,
            ),
            const SizedBox(height: 8),
            const Text('Try adjusting your filters'),
          ],
        ),
      );
    }

    return RefreshIndicator(
      onRefresh: _loadReleases,
      child: ListView.builder(
        padding: const EdgeInsets.all(16),
        itemCount: filtered.length,
        itemBuilder: (context, index) {
          final release = filtered[index];
          return _buildReleaseListCard(release);
        },
      ),
    );
  }

  Widget _buildReleaseListCard(Release release) {
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: ListTile(
        leading: Container(
          width: 12,
          height: 12,
          decoration: BoxDecoration(
            color: _getStatusColor(release.status),
            shape: BoxShape.circle,
          ),
        ),
        title: Text(
          release.version,
          style: Theme.of(context).textTheme.titleMedium?.copyWith(
            fontWeight: FontWeight.bold,
          ),
        ),
        subtitle: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (release.description != null)
              Text(
                release.description!,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            const SizedBox(height: 4),
            Wrap(
              spacing: 4,
              children: release.tags.take(3).map((tag) => Chip(
                label: Text(
                  tag,
                  style: const TextStyle(fontSize: 10),
                ),
                materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
                padding: const EdgeInsets.symmetric(horizontal: 4),
              )).toList(),
            ),
            const SizedBox(height: 4),
            Text(
              '${release.createdBy} • ${_formatTime(release.createdAt)}',
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                color: Colors.grey[600],
              ),
            ),
          ],
        ),
        trailing: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
              decoration: BoxDecoration(
                color: _getStatusColor(release.status),
                borderRadius: BorderRadius.circular(12),
              ),
              child: Text(
                _formatStatusName(release.status),
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 12,
                  fontWeight: FontWeight.bold,
                ),
              ),
            ),
            if (_canPromote(release)) ...[
              const SizedBox(height: 4),
              IconButton(
                icon: const Icon(Icons.arrow_upward, size: 16),
                onPressed: () => _promoteRelease(release),
                constraints: const BoxConstraints(),
                padding: EdgeInsets.zero,
                tooltip: 'Promote',
              ),
            ],
          ],
        ),
        onTap: () => _showReleaseDetails(release),
      ),
    );
  }

  Color _getStatusColor(ReleaseStatus status) {
    switch (status) {
      case ReleaseStatus.draft:
        return Colors.grey;
      case ReleaseStatus.staging:
        return Colors.blue;
      case ReleaseStatus.canary:
        return Colors.orange;
      case ReleaseStatus.production:
        return Colors.green;
      case ReleaseStatus.archived:
        return Colors.purple;
    }
  }

  String _formatStatusName(ReleaseStatus status) {
    return status.toString().split('.').last.toUpperCase();
  }

  bool _canPromote(Release release) {
    return release.status == ReleaseStatus.draft ||
           release.status == ReleaseStatus.staging ||
           release.status == ReleaseStatus.canary;
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

  void _promoteRelease(Release release) {
    ReleaseStatus nextStatus;
    switch (release.status) {
      case ReleaseStatus.draft:
        nextStatus = ReleaseStatus.staging;
        break;
      case ReleaseStatus.staging:
        nextStatus = ReleaseStatus.canary;
        break;
      case ReleaseStatus.canary:
        nextStatus = ReleaseStatus.production;
        break;
      default:
        return;
    }

    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Promote Release'),
        content: Text('Promote ${release.version} to ${_formatStatusName(nextStatus)}?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('Cancel'),
          ),
          ElevatedButton(
            onPressed: () {
              Navigator.of(context).pop();
              _performPromotion(release, nextStatus);
            },
            child: const Text('Promote'),
          ),
        ],
      ),
    );
  }

  void _performPromotion(Release release, ReleaseStatus newStatus) {
    // TODO: Implement actual promotion API call
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text('Promoting ${release.version} to ${_formatStatusName(newStatus)}'),
        duration: const Duration(seconds: 2),
      ),
    );

    // Mock update for demo
    setState(() {
      final index = _releases.indexWhere((r) => r.id == release.id);
      if (index != -1) {
        _releases[index] = release.copyWith(
          status: newStatus,
          updatedAt: DateTime.now().toIso8601String(),
        );
      }
    });
  }

  void _handleReleaseAction(Release release, String action) {
    switch (action) {
      case 'view':
        _showReleaseDetails(release);
        break;
      case 'download':
        _downloadArtifact(release);
        break;
      case 'changelog':
        _openChangelog(release);
        break;
      case 'commit':
        _viewCommit(release);
        break;
    }
  }

  void _showReleaseDetails(Release release) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (context) => DraggableScrollableSheet(
        initialChildSize: 0.7,
        maxChildSize: 0.95,
        minChildSize: 0.4,
        builder: (context, scrollController) => Container(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Container(
                    width: 16,
                    height: 16,
                    decoration: BoxDecoration(
                      color: _getStatusColor(release.status),
                      shape: BoxShape.circle,
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Text(
                      release.version,
                      style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                  ),
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                    decoration: BoxDecoration(
                      color: _getStatusColor(release.status),
                      borderRadius: BorderRadius.circular(16),
                    ),
                    child: Text(
                      _formatStatusName(release.status),
                      style: const TextStyle(
                        color: Colors.white,
                        fontSize: 12,
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
                    if (release.description != null) ...[
                      Text(
                        'Description',
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
                        child: Text(release.description!),
                      ),
                      const SizedBox(height: 20),
                    ],

                    Text(
                      'Details',
                      style: Theme.of(context).textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                    const SizedBox(height: 12),

                    _buildDetailRow('Version', release.version),
                    _buildDetailRow('Status', _formatStatusName(release.status)),
                    _buildDetailRow('Created By', release.createdBy),
                    _buildDetailRow('Created', _formatTime(release.createdAt)),
                    _buildDetailRow('Updated', _formatTime(release.updatedAt)),

                    if (release.commitSha != null)
                      _buildDetailRow('Commit SHA', release.commitSha!, isMonospace: true, onTap: () {
                        Clipboard.setData(ClipboardData(text: release.commitSha!));
                        ScaffoldMessenger.of(context).showSnackBar(
                          const SnackBar(content: Text('Commit SHA copied to clipboard')),
                        );
                      }),

                    if (release.tags.isNotEmpty) ...[
                      const SizedBox(height: 16),
                      Text(
                        'Tags',
                        style: Theme.of(context).textTheme.titleMedium?.copyWith(
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                      const SizedBox(height: 8),
                      Wrap(
                        spacing: 8,
                        runSpacing: 8,
                        children: release.tags.map((tag) => Chip(
                          label: Text(tag),
                          backgroundColor: Colors.blue[50],
                        )).toList(),
                      ),
                    ],

                    if (release.metadata != null && release.metadata!.isNotEmpty) ...[
                      const SizedBox(height: 16),
                      Text(
                        'Metadata',
                        style: Theme.of(context).textTheme.titleMedium?.copyWith(
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                      const SizedBox(height: 8),
                      ...release.metadata!.entries.map((entry) =>
                        _buildDetailRow(entry.key, entry.value.toString())),
                    ],

                    const SizedBox(height: 24),
                    Row(
                      children: [
                        if (release.artifactUrl != null)
                          Expanded(
                            child: ElevatedButton.icon(
                              onPressed: () => _downloadArtifact(release),
                              icon: const Icon(Icons.download),
                              label: const Text('Download'),
                            ),
                          ),
                        if (release.artifactUrl != null && release.changelogUrl != null)
                          const SizedBox(width: 12),
                        if (release.changelogUrl != null)
                          Expanded(
                            child: OutlinedButton.icon(
                              onPressed: () => _openChangelog(release),
                              icon: const Icon(Icons.description),
                              label: const Text('Changelog'),
                            ),
                          ),
                      ],
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildDetailRow(String label, String value, {bool isMonospace = false, VoidCallback? onTap}) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
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
            child: GestureDetector(
              onTap: onTap,
              child: Text(
                value,
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  fontFamily: isMonospace ? 'monospace' : null,
                  color: onTap != null ? Theme.of(context).primaryColor : null,
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  void _downloadArtifact(Release release) {
    // TODO: Implement artifact download
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('Downloading artifact for ${release.version}')),
    );
  }

  void _openChangelog(Release release) async {
    final url = release.changelogUrl;
    if (url != null) {
      final uri = Uri.parse(url);
      if (await canLaunchUrl(uri)) {
        await launchUrl(uri);
      } else {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('Could not launch changelog URL for ${release.version}')),
          );
        }
      }
    } else {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('No changelog URL available for ${release.version}')),
        );
      }
    }
  }

  void _viewCommit(Release release) {
    // TODO: Open commit in browser
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('Viewing commit ${release.commitSha}')),
    );
  }

  void _showCreateReleaseDialog() {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Create Release'),
        content: const Text('Release creation form would go here.'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('Cancel'),
          ),
          ElevatedButton(
            onPressed: () {
              Navigator.of(context).pop();
              // TODO: Implement release creation
            },
            child: const Text('Create'),
          ),
        ],
      ),
    );
  }
}