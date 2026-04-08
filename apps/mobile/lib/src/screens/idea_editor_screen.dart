import 'package:flutter/material.dart';

class IdeaEditorScreen extends StatelessWidget {
  const IdeaEditorScreen({
    super.key,
    required this.slug,
  });

  final String slug;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text(slug),
      ),
      body: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Text(
                  'Last saved: 2s ago',
                  style: Theme.of(context).textTheme.bodySmall,
                ),
                const Spacer(),
                const Chip(label: Text('creating repo')),
              ],
            ),
            const SizedBox(height: 12),
            Expanded(
              child: TextField(
                expands: true,
                minLines: null,
                maxLines: null,
                decoration: InputDecoration(
                  hintText: 'Write the idea here...',
                  alignLabelWithHint: true,
                  border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(20),
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
