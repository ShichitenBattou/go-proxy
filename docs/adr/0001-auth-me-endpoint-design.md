# ADR-0001: GET /auth/me エンドポイントの設計

## Status
提案中（Proposed）

## Context

現在、BFFには認証済みユーザーの情報を取得するエンドポイントが存在しません。フロントエンドが以下を実現するために `/auth/me` エンドポイントが必要です：

- ログイン状態の確認
- ユーザー名/メールアドレスの表示
- SPA初期化時の認証チェック

### 現在の実装状況

**Redisセッション構造:**
```go
// redis/redis.go:34-46
func SetSession(sessionId string, userId string) error
```
- キー: `session:<sha256(sessionId)>`
- 値: **文字列（userId のみ）**
- TTL: 30日（`SESSION_TTL`）

**問題点:**
現在のセッションには `userId` しか保存されていないため、`/auth/me` でユーザー情報を返すには以下のいずれかが必要：
1. セッションデータ構造を拡張してユーザー情報を保存
2. Keycloak UserInfo エンドポイントを都度呼び出し

## Decision

以下の設計判断について相談したいです：

### 1. セッションデータ構造

**選択肢A: JSON構造体に拡張（推奨）**
```go
type SessionData struct {
    UserID   string `json:"user_id"`    // OIDC sub claim
    Email    string `json:"email"`
    Name     string `json:"name"`
    // AccessToken  string `json:"access_token,omitempty"`  // 任意
    // RefreshToken string `json:"refresh_token,omitempty"` // 任意
}
```

**メリット:**
- Redis読み取り1回で完結（パフォーマンス）
- Keycloakへの依存を減らせる
- オフライン動作可能

**デメリット:**
- セッションサイズ増加
- ユーザー情報の更新がリアルタイムでない

---

**選択肢B: userId のみ保存 + UserInfo API呼び出し**
```go
// セッション構造は現状維持
// /auth/me のたびに Keycloak UserInfo エンドポイントを呼ぶ
```

**メリット:**
- セッションサイズ最小
- ユーザー情報が常に最新

**デメリット:**
- 毎回Keycloakへ通信（レイテンシ増加）
- Keycloakダウン時にユーザー情報取得不可
- Access Token の管理が必要

### 2. トークンの保存

**Access Token / Refresh Token をセッションに保存するか？**

**保存する場合:**
- `/auth/me` でUserInfo APIを呼べる
- API呼び出し時にバックエンドへトークンを転送可能（現在は未実装）
- トークンリフレッシュが可能

**保存しない場合:**
- セキュリティリスク低減（Redisから漏洩してもトークンなし）
- セッションサイズ削減
- ただし、UserInfo API呼び出し不可 → 選択肢Aが必須

### 3. レスポンス形式

**最小限（推奨）:**
```json
{
  "email": "user@example.com",
  "name": "John Doe",
  "sub": "keycloak-user-id"
}
```

**詳細:**
```json
{
  "sub": "keycloak-user-id",
  "email": "user@example.com",
  "email_verified": true,
  "name": "John Doe",
  "preferred_username": "john",
  "given_name": "John",
  "family_name": "Doe"
}
```

## 推奨案

**私の推奨: 選択肢A（JSON構造体）+ トークン保存なし + 最小限レスポンス**

**理由:**
1. **パフォーマンス**: Redis 1回読み取りで完結
2. **シンプル**: Keycloakへの依存最小
3. **セキュリティ**: トークンをRedisに保存しないことでリスク低減
4. **十分性**: BFFパターンでは、バックエンドAPIがユーザー識別に必要なのは `sub`（userID）のみ

**実装方針:**
- `/auth/callback` でIDトークンからクレームを抽出し、SessionDataとして保存
- `/auth/me` は Redis から SessionData を読み取り、そのまま返す
- トークンは保存せず、認証状態の維持のみセッションで管理

## Consequences

**選択肢Aを採用した場合:**

**ポジティブ:**
- 高速なレスポンス（Redisのみ）
- Keycloakダウンタイムの影響を受けない
- 実装がシンプル

**ネガティブ:**
- ユーザー情報の更新が即座に反映されない（例: メールアドレス変更）
  - 対策: 次回ログイン時に更新、または定期的にセッションを無効化
- 将来的にAccess Tokenが必要になった場合、構造変更が必要
  - 対策: その時点で段階的にトークン保存機能を追加

**ネガティブへの対応策:**
- ユーザー情報の鮮度が重要な場合は、セッションTTLを短くする（例: 1日）
- または、選択肢Bに切り替え（後方互換性は保てない）

---

## 質問

この設計で進めてよろしいでしょうか？または、以下の点について異なる方針がありますか？

1. **トークンの保存**: 将来的にバックエンドAPIへAccess Tokenを転送する予定はありますか？
2. **ユーザー情報の鮮度**: リアルタイムなユーザー情報取得が必要なユースケースはありますか？
3. **レスポンス項目**: email/name/sub 以外に必要なクレームはありますか？（例: roles, groups）
