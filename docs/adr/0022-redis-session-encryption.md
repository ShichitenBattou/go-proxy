# ADR-0022: Redis セッションデータの暗号化

## Status

提案中（Proposed） - 2026-05-12

## Context

### 現状と問題

`bff/redis/redis.go` の `SessionData` 構造体には、OAuth2/OIDC トークンが JSON 平文で Redis に保存されている。

```go
type SessionData struct {
    UserID       string `json:"user_id"`
    Email        string `json:"email"`
    Name         string `json:"name"`
    IDToken      string `json:"id_token,omitempty"`      // 平文
    AccessToken  string `json:"access_token,omitempty"`  // 平文
    RefreshToken string `json:"refresh_token,omitempty"` // 平文
    Provisioned  bool   `json:"provisioned"`
}
```

Redis が侵害された場合（メモリダンプ、RDB スナップショット、`KEYS`/`GET` コマンド等）、すべてのアクティブユーザーのトークンが即座に漏洩し、なりすましが可能になる。

この問題は CLAUDE.md の「既知の技術的負債（中優先度）」に記録されており、本番投入前に解消が必要とされている。

### 制約・前提

- Redis の TLS/ACL はインフラ制約（コンテナオーケストレーション環境）から単独では保証できない
- 呼び出し元（`proxy/handler.go`, `auth/` 各ハンドラー）の変更は最小化したい
- Go 標準ライブラリのみで実装（外部暗号ライブラリは追加しない）

## Decision

**`SetSession` / `GetSessionValue` 内で AES-256-GCM によりセッション JSON 全体を暗号化する。**

暗号化キーを環境変数 `REDIS_ENCRYPTION_KEY`（32 バイトの hex エンコード文字列）で管理する。

**暗号化対象の範囲**: `SessionData`（アクセストークン・リフレッシュトークン・ID トークンを含む）のみ。`StateData` は TTL 5 分の短命データで `redirect_url` と `created_at` しか持たず機密トークンを含まないため、暗号化対象外とする。

### 方式の選択肢比較

| 案 | 概要 | 採否 |
|---|---|---|
| A. **AES-256-GCM（本案）** | セッション JSON を丸ごと暗号化して Redis に保存 | ✅ 採用 |
| B. フィールドレベル暗号化 | `IDToken` / `AccessToken` / `RefreshToken` だけ暗号化 | ❌ 後から追加フィールドが漏れるリスク |
| C. Redis TLS + ACL のみ | インフラ設定で対処 | ❌ RDB スナップショット等のオフライン脅威に無力 |
| D. トークン参照方式 | 別ストアに保存し Redis には opaque ID だけ格納 | ❌ アーキテクチャ変更が大きい |

#### A を採用する理由

- **全体暗号化**により、将来的に追加されるフィールドが暗号化されない事故を防ぐ。
- AES-256-GCM は認証付き暗号（AEAD）であり、改ざん検知も兼ねる。
- Go 標準の `crypto/aes` + `crypto/cipher` のみで実装可能。
- `SetSession` / `GetSessionValue` 内に閉じるため、呼び出し側を無変更のまま透過的に適用できる。

### 実装概要

#### 1. 環境変数の追加（`bff/setup/config.go`）

```go
type Config struct {
    // ... 既存フィールド
    RedisEncryptionKey []byte // REDIS_ENCRYPTION_KEY (32バイト, hex decode済み)
}
```

`REDIS_ENCRYPTION_KEY` が未設定または長さが不正な場合は起動時に `log.Fatal` で停止する。

#### 2. 暗号化ヘルパー（`bff/redis/crypto.go`）

```go
// encrypt は AES-256-GCM でプレーンテキストを暗号化し、
// nonce(12B) + ciphertext + tag を結合したバイト列を返す。
func encrypt(plaintext, key []byte) ([]byte, error)

// decrypt は encrypt の逆操作を行う。
func decrypt(ciphertext, key []byte) ([]byte, error)
```

#### 3. `SetSession` / `GetSessionValue` の変更（`bff/redis/redis.go`）

```
SetSession:
  JSON Marshal → encrypt → base64 encode → Redis SET

GetSessionValue:
  Redis GET → base64 decode → decrypt → JSON Unmarshal
```

ストレージフォーマットは `base64(nonce + ciphertext + tag)` とする。

### 採用しなかった詳細設計

**移行期フォールバック（平文との共存）**: `GetSessionValue` で平文エントリを読んだ場合もエラーにせず正常動作させる案（`enc:` プレフィックスの有無で分岐）。今回は開発環境を対象としており既存データの後方互換は不要なため採用しない。暗号化導入時は Redis の既存セッションを全削除（または TTL 切れを待つ）で対応する。

**キーローテーション機構**: 現時点ではキーローテーションは実装しない。キーを変更する際はサービス再起動（＝全セッション無効化）で対応する。将来的に必要になった場合は別 ADR で検討する。

**nonce 生成**: `crypto/rand` でリクエストごとに 12 バイトのランダム nonce を生成する（GCM の標準推奨）。

## Consequences

**ポジティブ:**
- Redis が侵害されても、トークンは暗号化されているため即座になりすましが不可能になる
- 改ざん検知（AEAD）により、Redis データの完全性も保証される
- 呼び出し元（`proxy/`, `auth/`）のコード変更ゼロ

**ネガティブ:**
- `REDIS_ENCRYPTION_KEY` の管理責任が生じる（紛失 → 全セッション復号不能）
- 暗号化によるわずかな CPU オーバーヘッド（AES-NI 対応 CPU では無視できるレベル）
- 暗号化キー変更時は既存セッションがすべて無効になる（再ログイン必要）

## Implementation Notes

変更対象ファイル：

| ファイル | 変更内容 |
|---|---|
| `bff/setup/config.go` | `RedisEncryptionKey []byte` フィールド追加、起動時バリデーション |
| `bff/redis/crypto.go` | `encrypt` / `decrypt` 関数（新規作成） |
| `bff/redis/redis.go` | `SetSession` / `GetSessionValue` に暗号化/復号を組み込む |
| `bff/.env.example` | `REDIS_ENCRYPTION_KEY` の説明コメント追加 |
| `bff/redis/crypto_test.go` | `encrypt` / `decrypt` のユニットテスト（新規作成） |
| `bff/redis/redis_test.go` | `SetSession` → `GetSessionValue` の暗号化ラウンドトリップテスト |

テスト用キー生成方法：
```bash
openssl rand -hex 32
```
