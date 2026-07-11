# E-C01-PATH-CONTRACT

Verified on Windows on 2026-07-11.

- Configuration test now uses `t.TempDir()` and host-native `filepath.Clean` / `filepath.Join` expectations. Production `config.Load` path behavior was not changed.
- Icon validation now accepts only one portable filename and rejects empty, `.`, `..`, POSIX and Windows parent traversal, POSIX root, Windows drive roots with either separator, UNC roots, device roots, mixed separators, and nested relative paths.
- Focused configuration test passed twice (`artifact://17`).
- Focused icon path table passed twice (`artifact://15`).
- Affected backend package regression passed (`artifact://52`).
- No test skip, build tag, platform allow-list, renamed test, removed assertion, route change, or relaxed security expectation was introduced.
