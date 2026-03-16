# コミットメッセージ規約

[Conventional Commits](https://www.conventionalcommits.org/) 形式に従い、**1行**で記述する。ボディ・フッターは不要。

## フォーマット

```
<絵文字> <type>(<scope>): <日本語の説明>
```

- `<scope>` は省略可
- 説明は**日本語**、命令形（「〜を追加」「〜を修正」）

## タイプと絵文字

| 絵文字 | type | 用途 |
|--------|------|------|
| ✨ | feat | 新機能の追加 |
| 🐛 | fix | バグ修正 |
| 📝 | docs | ドキュメントのみの変更 |
| 💄 | style | 動作に影響しない変更（フォーマット等） |
| ♻️ | refactor | バグ修正・機能追加を伴わないコード変更 |
| ✅ | test | テストの追加・修正 |
| 🔧 | chore | ビルド・ツール・設定ファイルの変更 |
| ⚡ | perf | パフォーマンス改善 |
| 👷 | ci | CI/CD 設定の変更 |

## 例

```
✨ feat(auth): Keycloak OIDC ログインフローを実装
🐛 fix(proxy): セッションCookieが二重に設定される問題を修正
♻️ refactor(redis): セッション保存ロジックを共通化
🔧 chore: Docker Composeにヘルスチェックを追加
```
