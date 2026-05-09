# ADR-0014: フロントエンドへの API Explorer 追加

## Status
承認済み（Accepted） - 2026-04-30

## Context

現在の `front/pages/index.html` は投稿一覧を表示するだけの最小限の画面であり、API の各エンドポイントを手軽に呼び出す手段がない。

OpenAPI 仕様（`api/tmp/openapi.json`）には以下のエンドポイントが定義されている（`task export-openapi` で 2026-04-30 に再生成済み）。

| Method | Path | タグ | ログイン要否 |
|--------|------|------|----------|
| GET | /health | default | 不要 |
| POST | /posts | posts | **必要**（BFF セッション） |
| GET | /posts | posts | **必要**（BFF セッション） |
| GET | /posts/{post_id} | posts | **必要**（BFF セッション） |
| GET | /public/posts | **public** | **不要**（BFF がセッションなしで通過させる） |
| POST | /users | users | **必要**（BFF セッション） |
| GET | /users/{user_id} | users | **必要**（BFF セッション） |
| DELETE | /users/{user_id} | users | **必要**（BFF セッション） |

`public` タグのエンドポイント（`GET /public/posts`）は BFF のパブリックプロキシハンドラーを通じてセッションなしでもアクセス可能。それ以外は BFF がセッション検証を行い、未認証の場合は Keycloak へリダイレクトする。

BFF を経由するすべてのリクエストは `/api/` プレフィックスを付けて送信される。BFF は `/api` プレフィックスを除去してバックエンド API へ転送する。

POC フェーズのため、動作検証を素早く行える UI が必要とされている。

## Decision

`front/pages/index.html`・`index.js`・`index.css` を以下の方針で拡張し、API Explorer 機能を追加する。

### 1. 画面構成

- ヘッダー部分（ログイン前後の表示切り替え）は既存の実装をそのまま維持する
- メインコンテンツを API Explorer に置き換える
- エンドポイントを OpenAPI タグ（Health / Posts / Users）でグループ化してセクション表示する

### 2. ログイン前後の表示

POC のため、ログイン状態に関わらず**全エンドポイントを同一画面で表示する**。

- `public` タグのエンドポイント（`GET /public/posts`）には「Public」バッジを付ける
- BFF セッションが必要なエンドポイントには「要ログイン」バッジを付け、視覚的に区別する
- どちらのバッジも表示のみで、フォーム・送信ボタンは全エンドポイントで共通して表示する

### 3. API 呼び出し

- すべての fetch は `/api/<path>` 形式（`https://auth.local/api/...`）で送信する（BFF 経由）
- `credentials: 'include'` を付与してセッション Cookie を送信する
- パスパラメータ（`{user_id}` など）・クエリパラメータ・リクエストボディは入力フォームで受け取る
- レスポンス（ステータスコード + JSON ボディ）はエンドポイントカードの下部に表示する
- エラー（ネットワークエラー・4xx/5xx）も同領域に表示する

### 4. ファイル変更範囲

- `front/pages/index.html` — メインコンテンツを API Explorer に置き換える
- `front/pages/index.js` — 各エンドポイントの呼び出しロジックと `apiCall()` ディスパッチ関数を追加する
- `front/pages/index.css` — エンドポイントカード・メソッドバッジ・レスポンス表示領域のスタイルを追加する

新規ファイルは作成しない（既存ファイルの拡張のみ）。

## Consequences

**ポジティブ:**
- ブラウザだけで全 API エンドポイントを手軽に動作確認できる
- ログイン前後で同一 UI を提供するため、POC 期間中の検証コストが下がる
- 既存のヘッダー・認証状態チェック処理を変更しないため、回帰リスクが低い

**ネガティブ:**
- ログイン前に「要ログイン」エンドポイントが見えているが、実際に呼び出すと BFF または API から 401/403 が返る（POC として許容）
- UI はフォーム入力ベースの簡易実装であり、プロダクション品質ではない

## Implementation Notes

- `index.js` の `apiCall(endpointId)` が各エンドポイントのロジックをディスパッチする
- レスポンス表示は `<pre id="res-{endpoint-id}">` に JSON.stringify で整形出力する
- `DELETE /users/{user_id}` は 204 No Content のためボディなしで「204 No Content」を表示する
- `GET /posts` / `GET /public/posts` のクエリパラメータ（author_id / limit / offset）は値が空の場合は URL に含めない
- `GET /public/posts` は `/api/public/posts` へ送信する（BFF パブリックハンドラー経由）
- `GET /health` は `/api/health` へ送信する
