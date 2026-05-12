#!/usr/bin/env python3
"""OpenAPI スキーマ出力スクリプト

FastAPI アプリの OpenAPI スキーマを JSON 形式で出力する。

使い方:
    uv run scripts/export_openapi.py
    uv run scripts/export_openapi.py --output openapi.json
"""

import json
import sys
from pathlib import Path
from typing import Annotated

import typer

# プロジェクトルートを sys.path に追加
sys.path.insert(0, str(Path(__file__).parent.parent))

from main import app  # noqa: E402

cli = typer.Typer()


@cli.command()
def main(
    output: Annotated[
        Path | None,
        typer.Option("--output", "-o", help="出力ファイルパス。省略時は標準出力。"),
    ] = None,
) -> None:
    """OpenAPI スキーマを JSON で出力する。"""
    schema = app.openapi()
    content = json.dumps(schema, ensure_ascii=False, indent=2)

    if output:
        output.write_text(content, encoding="utf-8")
        typer.echo(f"OpenAPI スキーマを {output} に書き出しました。", err=True)
    else:
        typer.echo(content)


if __name__ == "__main__":
    cli()
