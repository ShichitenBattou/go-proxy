---
name: openapi-reader
description: >
  OpenAPI JSON仕様ファイルを読み取り、APIのエンドポイント・リクエスト/レスポンススキーマを
  構造化して把握するためのスキル。openapi.json や openapi.yaml を扱う作業、
  APIのエンドポイント一覧を確認したい場面、リクエスト/レスポンスの型を理解したい場面、
  コード生成やAPIクライアント実装の前にAPIの全体像を把握したい場合に必ず使用すること。
  「openapi」「API仕様」「エンドポイント一覧」「スキーマ確認」「APIドキュメント」といった
  キーワードが出てきたら積極的に使用すること。
---

## 概要

`uv run` + PEP 723形式のスクリプトを使って openapi.json を解析し、
Claude が API 全体を把握できる形式で出力する。

## 使い方

### 1. openapi.jsonのパスを特定する

ユーザーから明示されていない場合は、以下の順で探す:

```bash
# プロジェクトルートを探す
find . -name "openapi.json" -not -path "*/node_modules/*" 2>/dev/null

# FastAPIプロジェクトなら実行中のサーバーから取得するか、エクスポートスクリプトを使う
# このプロジェクトの場合:
cd api && uv run scripts/export_openapi.py --output /tmp/openapi.json
```

### 2. スクリプトを実行する

```bash
uv run /path/to/skills/openapi-reader/scripts/parse_openapi.py <openapi.jsonのパス>
```

このプロジェクトでの具体例:

```bash
# まずエクスポート
cd /home/pocko/Projects/go-proxy/api && uv run scripts/export_openapi.py --output /tmp/openapi.json

# 次に解析
uv run /home/pocko/Projects/go-proxy/.claude/skills/openapi-reader/scripts/parse_openapi.py /tmp/openapi.json
```

### 3. 出力を読み取る

スクリプトは以下を出力する:

- **API情報**: タイトル・バージョン・説明
- **エンドポイント一覧**: タグでグループ化、HTTPメソッド・パス・概要
- **各エンドポイントの詳細**: パスパラメータ・クエリパラメータ・リクエストボディ・レスポンス
- **コンポーネントスキーマ**: 再利用されるデータモデルの定義

### オプション

```bash
# エンドポイント一覧のみ（簡易表示）
uv run .../parse_openapi.py openapi.json --summary

# 特定タグのみ
uv run .../parse_openapi.py openapi.json --tag users

# スキーマのみ
uv run .../parse_openapi.py openapi.json --schemas-only
```

## スクリプトの場所

`scripts/parse_openapi.py` — PEP 723インライン依存関係で動作する。
`uv` がインストールされていれば追加セットアップ不要。

## 注意事項

- OpenAPI 3.x 形式に対応（2.x/Swagger は非対応）
- `$ref` 参照は再帰的に解決して表示する
- 出力が大きい場合は `--summary` フラグで簡略化できる
