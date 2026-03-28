import 'dart:async';
import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;

/// Manages real-time data updates and Server-Sent Events (SSE) connections
class RealtimeManager extends ChangeNotifier {
  static final RealtimeManager _instance = RealtimeManager._internal();
  factory RealtimeManager() => _instance;
  RealtimeManager._internal();

  StreamController<RealtimeEvent>? _eventController;
  Timer? _refreshTimer;
  bool _isConnected = false;
  String? _sseUrl;
  http.Client? _sseClient;

  /// Stream of real-time events
  Stream<RealtimeEvent> get eventStream => _eventController?.stream ?? Stream.empty();

  /// Whether the real-time connection is active
  bool get isConnected => _isConnected;

  /// Initialize real-time updates
  Future<void> initialize({
    String? baseUrl,
    String? apiKey,
    Duration? refreshInterval,
  }) async {
    _eventController ??= StreamController<RealtimeEvent>.broadcast();

    // Set up periodic refresh timer
    _refreshTimer?.cancel();
    if (refreshInterval != null) {
      _refreshTimer = Timer.periodic(refreshInterval, (_) {
        _triggerRefresh();
      });
    }

    // Connect to SSE if URL provided
    if (baseUrl != null) {
      await _connectSSE(baseUrl, apiKey);
    }
  }

  /// Connect to Server-Sent Events stream
  Future<void> _connectSSE(String baseUrl, String? apiKey) async {
    try {
      _sseUrl = '$baseUrl/events/stream';
      _sseClient = http.Client();

      final headers = <String, String>{
        'Accept': 'text/event-stream',
        'Cache-Control': 'no-cache',
      };

      if (apiKey != null) {
        headers['Authorization'] = 'Bearer $apiKey';
      }

      final request = http.Request('GET', Uri.parse(_sseUrl!));
      request.headers.addAll(headers);

      final response = await _sseClient!.send(request);

      if (response.statusCode == 200) {
        _isConnected = true;
        notifyListeners();

        response.stream
            .transform(utf8.decoder)
            .transform(const LineSplitter())
            .listen(
              _handleSSEData,
              onError: _handleSSEError,
              onDone: _handleSSEDone,
            );
      }
    } catch (e) {
      debugPrint('SSE connection failed: $e');
      _handleSSEError(e);
    }
  }

  /// Handle incoming SSE data
  void _handleSSEData(String line) {
    if (line.startsWith('data: ')) {
      final data = line.substring(6);
      try {
        final json = jsonDecode(data) as Map<String, dynamic>;
        final event = RealtimeEvent.fromJson(json);
        _eventController?.add(event);
      } catch (e) {
        debugPrint('Failed to parse SSE data: $e');
      }
    }
  }

  /// Handle SSE errors
  void _handleSSEError(Object error) {
    debugPrint('SSE error: $error');
    _isConnected = false;
    notifyListeners();

    // Attempt to reconnect after delay
    Timer(const Duration(seconds: 5), () {
      if (_sseUrl != null) {
        _connectSSE(_sseUrl!.split('/events/stream')[0], null);
      }
    });
  }

  /// Handle SSE connection closed
  void _handleSSEDone() {
    debugPrint('SSE connection closed');
    _isConnected = false;
    notifyListeners();
  }

  /// Trigger a refresh event for all subscribers
  void _triggerRefresh() {
    _eventController?.add(RealtimeEvent(
      type: RealtimeEventType.refresh,
      timestamp: DateTime.now(),
    ));
  }

  /// Manually trigger refresh
  void refresh() {
    _triggerRefresh();
  }

  /// Subscribe to specific event types
  Stream<RealtimeEvent> subscribe(List<RealtimeEventType> eventTypes) {
    return eventStream.where((event) => eventTypes.contains(event.type));
  }

  /// Dispose of resources
  @override
  void dispose() {
    _refreshTimer?.cancel();
    _sseClient?.close();
    _eventController?.close();
    super.dispose();
  }
}

/// Types of real-time events
enum RealtimeEventType {
  refresh,
  flagUpdated,
  deploymentStatusChanged,
  releasePromoted,
  systemAlert,
}

/// Real-time event data structure
class RealtimeEvent {
  final RealtimeEventType type;
  final DateTime timestamp;
  final Map<String, dynamic>? data;

  RealtimeEvent({
    required this.type,
    required this.timestamp,
    this.data,
  });

  factory RealtimeEvent.fromJson(Map<String, dynamic> json) {
    return RealtimeEvent(
      type: _parseEventType(json['type'] as String?),
      timestamp: DateTime.tryParse(json['timestamp'] as String? ?? '') ?? DateTime.now(),
      data: json['data'] as Map<String, dynamic>?,
    );
  }

  static RealtimeEventType _parseEventType(String? type) {
    switch (type) {
      case 'flag_updated':
        return RealtimeEventType.flagUpdated;
      case 'deployment_status_changed':
        return RealtimeEventType.deploymentStatusChanged;
      case 'release_promoted':
        return RealtimeEventType.releasePromoted;
      case 'system_alert':
        return RealtimeEventType.systemAlert;
      default:
        return RealtimeEventType.refresh;
    }
  }
}

/// Mixin for screens that need real-time updates
mixin RealtimeDataMixin<T extends StatefulWidget> on State<T> {
  StreamSubscription<RealtimeEvent>? _realtimeSubscription;
  Timer? _refreshTimer;

  /// Override this method to handle real-time events
  void onRealtimeEvent(RealtimeEvent event) {}

  /// Override this method to specify which events to listen for
  List<RealtimeEventType> get subscribedEvents => [
    RealtimeEventType.refresh,
  ];

  /// Start listening to real-time events
  void startRealtimeUpdates() {
    _realtimeSubscription = RealtimeManager()
        .subscribe(subscribedEvents)
        .listen(onRealtimeEvent);
  }

  /// Stop listening to real-time events
  void stopRealtimeUpdates() {
    _realtimeSubscription?.cancel();
    _realtimeSubscription = null;
    _refreshTimer?.cancel();
    _refreshTimer = null;
  }

  /// Set up periodic refresh timer
  void setupPeriodicRefresh(Duration interval, VoidCallback callback) {
    _refreshTimer?.cancel();
    _refreshTimer = Timer.periodic(interval, (_) => callback());
  }

  @override
  void dispose() {
    stopRealtimeUpdates();
    super.dispose();
  }
}

/// Auto-refresh configuration
class AutoRefreshConfig {
  final Duration interval;
  final bool enabled;
  final List<RealtimeEventType> triggerEvents;

  const AutoRefreshConfig({
    this.interval = const Duration(seconds: 30),
    this.enabled = true,
    this.triggerEvents = const [
      RealtimeEventType.refresh,
      RealtimeEventType.flagUpdated,
      RealtimeEventType.deploymentStatusChanged,
      RealtimeEventType.releasePromoted,
    ],
  });
}