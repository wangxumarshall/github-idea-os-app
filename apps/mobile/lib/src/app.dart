import 'package:flutter/material.dart';

import 'screens/idea_editor_screen.dart';
import 'screens/ideas_home_screen.dart';
import 'screens/new_idea_screen.dart';

class IdeaOSApp extends StatelessWidget {
  const IdeaOSApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'GitHub IdeaOS',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF1C7C54)),
        useMaterial3: true,
      ),
      routes: {
        '/': (_) => const IdeasHomeScreen(),
        '/ideas/new': (_) => const NewIdeaScreen(),
      },
      onGenerateRoute: (settings) {
        if (settings.name?.startsWith('/ideas/') == true) {
          final slug = settings.name!.substring('/ideas/'.length);
          return MaterialPageRoute<void>(
            builder: (_) => IdeaEditorScreen(slug: slug),
            settings: settings,
          );
        }
        return null;
      },
    );
  }
}
