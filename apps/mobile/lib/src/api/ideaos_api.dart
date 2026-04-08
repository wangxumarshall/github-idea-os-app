import 'dart:convert';

import 'package:http/http.dart' as http;

class IdeaSummary {
  const IdeaSummary({
    required this.code,
    required this.slug,
    required this.title,
    required this.summary,
    required this.updatedAt,
  });

  final String code;
  final String slug;
  final String title;
  final String summary;
  final String updatedAt;

  factory IdeaSummary.fromJson(Map<String, dynamic> json) {
    return IdeaSummary(
      code: json['code'] as String? ?? '',
      slug: json['slug'] as String? ?? '',
      title: json['title'] as String? ?? '',
      summary: json['summary'] as String? ?? '',
      updatedAt: json['updated_at'] as String? ?? '',
    );
  }
}

class IdeaDocument extends IdeaSummary {
  const IdeaDocument({
    required super.code,
    required super.slug,
    required super.title,
    required super.summary,
    required super.updatedAt,
    required this.content,
    required this.sha,
  });

  final String content;
  final String sha;

  factory IdeaDocument.fromJson(Map<String, dynamic> json) {
    return IdeaDocument(
      code: json['code'] as String? ?? '',
      slug: json['slug'] as String? ?? '',
      title: json['title'] as String? ?? '',
      summary: json['summary'] as String? ?? '',
      updatedAt: json['updated_at'] as String? ?? '',
      content: json['content'] as String? ?? '',
      sha: json['sha'] as String? ?? '',
    );
  }
}

class IdeaOSApi {
  IdeaOSApi({
    required this.baseUrl,
    required this.bearerToken,
    required this.workspaceId,
    http.Client? client,
  }) : _client = client ?? http.Client();

  final String baseUrl;
  final String bearerToken;
  final String workspaceId;
  final http.Client _client;

  Map<String, String> get _headers => {
        'Authorization': 'Bearer $bearerToken',
        'X-Workspace-ID': workspaceId,
        'Content-Type': 'application/json',
      };

  Future<List<IdeaSummary>> listIdeas() async {
    final response = await _client.get(Uri.parse('$baseUrl/api/ideas'), headers: _headers);
    final payload = _decodeList(response);
    return payload.map(IdeaSummary.fromJson).toList();
  }

  Future<IdeaDocument> getIdea(String slug) async {
    final response = await _client.get(Uri.parse('$baseUrl/api/ideas/$slug'), headers: _headers);
    return IdeaDocument.fromJson(_decodeMap(response));
  }

  Future<IdeaDocument> createIdea({
    required String rawInput,
    required String selectedName,
  }) async {
    final response = await _client.post(
      Uri.parse('$baseUrl/api/ideas'),
      headers: _headers,
      body: jsonEncode({
        'raw_input': rawInput,
        'selected_name': selectedName,
      }),
    );
    return IdeaDocument.fromJson(_decodeMap(response));
  }

  Future<IdeaDocument> updateIdea({
    required String slug,
    required String title,
    required String content,
    required String createdAt,
    required String sha,
    List<String> tags = const [],
  }) async {
    final response = await _client.put(
      Uri.parse('$baseUrl/api/ideas/$slug'),
      headers: _headers,
      body: jsonEncode({
        'title': title,
        'content': content,
        'created_at': createdAt,
        'sha': sha,
        'tags': tags,
      }),
    );
    return IdeaDocument.fromJson(_decodeMap(response));
  }

  Map<String, dynamic> _decodeMap(http.Response response) {
    if (response.statusCode < 200 || response.statusCode >= 300) {
      throw Exception('API error: ${response.statusCode} ${response.body}');
    }
    return jsonDecode(response.body) as Map<String, dynamic>;
  }

  List<Map<String, dynamic>> _decodeList(http.Response response) {
    if (response.statusCode < 200 || response.statusCode >= 300) {
      throw Exception('API error: ${response.statusCode} ${response.body}');
    }
    final payload = jsonDecode(response.body) as List<dynamic>;
    return payload.cast<Map<String, dynamic>>();
  }
}
