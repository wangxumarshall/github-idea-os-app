import 'package:flutter/material.dart';

class NewIdeaScreen extends StatelessWidget {
  const NewIdeaScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('New Idea')),
      body: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Describe the idea',
              style: Theme.of(context).textTheme.titleSmall,
            ),
            const SizedBox(height: 8),
            TextField(
              minLines: 8,
              maxLines: 12,
              decoration: InputDecoration(
                hintText: 'Describe the idea and let IdeaOS recommend names...',
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(16),
                ),
              ),
            ),
            const SizedBox(height: 20),
            Text(
              'Suggested names',
              style: Theme.of(context).textTheme.titleSmall,
            ),
            const SizedBox(height: 12),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: const [
                Chip(label: Text('idea0004-ai-pr-review')),
                Chip(label: Text('idea0004-pr-risk-lens')),
                Chip(label: Text('idea0004-review-architect')),
              ],
            ),
            const Spacer(),
            SizedBox(
              width: double.infinity,
              child: FilledButton(
                onPressed: () {},
                child: const Text('Create'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
