# ADR-0016: Keycloak 未起動時の NGINX エラーページ表示

## Status
承認済み（Accepted） - 2026-05-07

## Context

### 問題

現在の NGINX 設定では、`/idp/` ロケーションのリバースプロキシ先である Keycloak (`keycloak:8080`) が起動していない場合、NGINX はデフォルトのエラーレスポンス（502 Bad Gateway など）をそのまま返す。ユーザーにとってこのレスポンスは意味が不明確であり、「何が起きているか」「どうすればよいか」が伝わらない。

### 発生するエラーコード

Keycloak が未起動 or 到達不能の場合、以下の HTTP エラーが発生しうる:

| コード | 意味 |
|--------|------|
| 502    | Bad Gateway（Keycloak に接続できない）|
| 503    | Service Unavailable（Keycloak が起動中 or 過負荷）|
| 504    | Gateway Timeout（Keycloak が応答しない）|

### 選択肢

**案A: `/idp/` ロケーション専用のカスタムエラーページ**
`/idp/` プロキシブロックに `error_page` ディレクティブを追加し、502/503/504 発生時に専用のエラー HTML を返す。

- Keycloak 固有のメッセージを伝えられる
- 他のロケーション（`/api/` など）のエラーには影響しない

**案B: サーバーレベルでグローバルなエラーページ**
`server` ブロック全体に `error_page` を設定し、すべての 5xx を同一ページにルーティングする。

- 設定がシンプル
- エラー原因が Keycloak か BFF か区別できない

## Decision

**案A: `/idp/` ロケーション専用のカスタムエラーページ** を採用する。

Keycloak 未起動は通常の運用停止とは異なるため、「認証サービスが利用できません」という明確なメッセージをユーザーに伝えるべきである。`/api/` の BFF エラーとは別管理とする。

### 変更概要

#### `front/pages/error/keycloak_unavailable.html`（新規作成）

- 既存の `index.html` / `index.css` のデザインに合わせたシンプルなエラーページ
- メッセージ: 「認証サービスが起動していません。しばらくしてから再度お試しください。」
- ホームページへのリンクを設置

#### `front/default.conf.template`（変更）

`/idp/` ロケーションに以下を追加:

```nginx
proxy_intercept_errors on;
error_page 502 503 504 /error/keycloak_unavailable.html;
```

エラーページの配信用ロケーションを追加:

```nginx
location = /error/keycloak_unavailable.html {
    root /var/www/pages;
    internal;
}
```

`internal` ディレクティブにより、エラーページへの直接アクセスを防ぐ。

## Consequences

**ポジティブ:**
- Keycloak 未起動時にユーザーフレンドリーなエラーページを表示できる
- エラー原因（認証サービス停止）を明示できる
- 他ロケーションへの影響なし

**ネガティブ:**
- エラーページ HTML ファイルの追加・管理が必要
- `proxy_intercept_errors on` は `/idp/` からの 4xx レスポンスも NGINX が横取りする可能性があるため、4xx は対象外に設定する

## Implementation Notes

- `proxy_intercept_errors on` を有効にした場合、Keycloak が返す 4xx（例: 401 Unauthorized）も NGINX が処理しようとするため、`error_page` は 5xx のみに限定する
- エラーページのスタイルはインライン CSS で完結させ、外部 CSS ファイルへの依存を減らす（エラー発生時に CSS が取得できない場合があるため）
- NGINX の `internal` ディレクティブにより `/error/keycloak_unavailable.html` への直接リクエストは 404 になる
