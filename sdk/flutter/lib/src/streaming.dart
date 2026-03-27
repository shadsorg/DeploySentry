import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:http/http.dart' as http;

import 'models.dart';

/// Server-Sent Events (SSE) client for real-time flag updates.
class FlagStreamClient {
  static const Duration _initialRetryDelay = Duration(seconds: 1);
  static const Duration _maxRetryDelay = Duration(seconds: 30);
  static const double _backoffMultiplier = 2.0;
  static const double _jitterFraction = 0.2;
  static final _random = Random();

  final String url;
  final Map<String, String> headers;

  http.Client? _httpClient;
  StreamController<Flag>? _controller;
  bool _closed = false;
  Timer? _reconnectTimer;
  Duration _currentRetryDelay = _initialRetryDelay;

  FlagStreamClient({
    required this.url,
    required this.headers,
  });

  /// Stream of flag updates received from the server.
  Stream<Flag> get updates {
    _controller ??= StreamController<Flag>.broadcast(
      onListen: _connect,
      onCancel: _disconnect,
    );
    return _controller!.stream;
  }

  /// Start listening for SSE events.
  void _connect() async {
    if (_closed) return;

    _httpClient = http.Client();
    final request = http.Request('GET', Uri.parse(url));
    request.headers.addAll(headers);
    request.headers['Accept'] = 'text/event-stream';
    request.headers['Cache-Control'] = 'no-cache';

    try {
      final response = await _httpClient!.send(request);

      if (response.statusCode != 200) {
        _scheduleReconnect();
        return;
      }
      _currentRetryDelay = _initialRetryDelay; // Reset on successful connect

      final lineBuffer = StringBuffer();

      await for (final chunk in response.stream.transform(utf8.decoder)) {
        if (_closed) break;

        lineBuffer.write(chunk);
        final content = lineBuffer.toString();
        final lines = content.split('\n');

        // Keep the last incomplete line in the buffer.
        lineBuffer.clear();
        if (!content.endsWith('\n')) {
          lineBuffer.write(lines.removeLast());
        } else {
          lines.removeLast(); // Remove trailing empty string from split.
        }

        String? eventData;
        for (final line in lines) {
          if (line.startsWith('data: ')) {
            eventData = line.substring(6);
          } else if (line.isEmpty && eventData != null) {
            _handleEvent(eventData);
            eventData = null;
          }
        }
      }
    } catch (_) {
      // Connection lost or error; attempt reconnect.
    }

    if (!_closed) {
      _scheduleReconnect();
    }
  }

  void _handleEvent(String data) {
    try {
      final json = jsonDecode(data) as Map<String, dynamic>;
      final flag = Flag.fromJson(json);
      _controller?.add(flag);
    } catch (_) {
      // Skip malformed events.
    }
  }

  void _scheduleReconnect() {
    if (_closed) return;
    _reconnectTimer?.cancel();

    final jitter = _currentRetryDelay.inMilliseconds *
        _jitterFraction *
        (2 * _random.nextDouble() - 1);
    final jitteredDelay = Duration(
      milliseconds: _currentRetryDelay.inMilliseconds + jitter.round(),
    );

    _reconnectTimer = Timer(jitteredDelay, _connect);

    _currentRetryDelay = Duration(
      milliseconds: (_currentRetryDelay.inMilliseconds * _backoffMultiplier)
          .round()
          .clamp(0, _maxRetryDelay.inMilliseconds),
    );
  }

  void _disconnect() {
    _httpClient?.close();
    _httpClient = null;
    _reconnectTimer?.cancel();
    _reconnectTimer = null;
  }

  /// Close the streaming connection and release resources.
  void close() {
    _closed = true;
    _disconnect();
    _controller?.close();
    _controller = null;
  }
}
