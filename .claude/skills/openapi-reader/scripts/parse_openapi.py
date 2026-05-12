#!/usr/bin/env python3
# /// script
# requires-python = ">=3.11"
# dependencies = [
#   "rich>=13.0",
#   "typer>=0.9",
# ]
# ///
"""OpenAPI JSON仕様を読み取り、構造化して表示するスクリプト。

使い方:
    uv run parse_openapi.py openapi.json
    uv run parse_openapi.py openapi.json --summary
    uv run parse_openapi.py openapi.json --tag users
    uv run parse_openapi.py openapi.json --schemas-only
"""

import json
import sys
from pathlib import Path
from typing import Any

import typer
from rich.console import Console
from rich.panel import Panel
from rich.table import Table
from rich.text import Text
from rich import print as rprint

console = Console()
app = typer.Typer(add_completion=False)

# HTTPメソッドの色
METHOD_COLORS = {
    "get": "green",
    "post": "blue",
    "put": "yellow",
    "patch": "magenta",
    "delete": "red",
    "options": "cyan",
    "head": "white",
}


def resolve_ref(schema: dict[str, Any], components: dict[str, Any]) -> dict[str, Any]:
    """$ref を再帰的に解決する。"""
    if "$ref" not in schema:
        return schema

    ref_path = schema["$ref"]  # 例: "#/components/schemas/User"
    parts = ref_path.lstrip("#/").split("/")

    resolved = components
    for part in parts[1:]:  # "components" をスキップ
        resolved = resolved.get(part, {})

    return resolve_ref(resolved, components)


def schema_to_str(schema: dict[str, Any], components: dict[str, Any], depth: int = 0) -> str:
    """スキーマを人間が読みやすい文字列に変換する。"""
    if not schema:
        return "any"

    schema = resolve_ref(schema, components)
    indent = "  " * depth

    schema_type = schema.get("type", "")
    schema_format = schema.get("format", "")

    if "$ref" in schema:
        ref_name = schema["$ref"].split("/")[-1]
        return ref_name

    if schema_type == "array":
        items = schema.get("items", {})
        item_str = schema_to_str(items, components, depth)
        return f"array[{item_str}]"

    if schema_type == "object" or "properties" in schema:
        props = schema.get("properties", {})
        required_fields = set(schema.get("required", []))
        if not props:
            additional = schema.get("additionalProperties")
            if additional:
                val_type = schema_to_str(additional, components, depth)
                return f"object<string, {val_type}>"
            return "object"

        if depth > 2:
            return f"object{{{', '.join(props.keys())}}}"

        lines = ["object {"]
        for prop_name, prop_schema in props.items():
            marker = "" if prop_name in required_fields else "?"
            prop_type = schema_to_str(prop_schema, components, depth + 1)
            description = prop_schema.get("description", "")
            desc_str = f"  # {description}" if description else ""
            lines.append(f"{indent}  {prop_name}{marker}: {prop_type}{desc_str}")
        lines.append(f"{indent}}}")
        return "\n".join(lines)

    if "enum" in schema:
        enum_vals = " | ".join(repr(v) for v in schema["enum"])
        return f"enum({enum_vals})"

    if "oneOf" in schema:
        parts = [schema_to_str(s, components, depth) for s in schema["oneOf"]]
        return " | ".join(parts)

    if "anyOf" in schema:
        parts = [schema_to_str(s, components, depth) for s in schema["anyOf"]]
        return " | ".join(parts)

    if "allOf" in schema:
        parts = [schema_to_str(s, components, depth) for s in schema["allOf"]]
        return " & ".join(parts)

    type_str = schema_type or "any"
    if schema_format:
        type_str = f"{type_str}({schema_format})"

    return type_str


def display_info(spec: dict[str, Any]) -> None:
    """API基本情報を表示する。"""
    info = spec.get("info", {})
    title = info.get("title", "Untitled API")
    version = info.get("version", "unknown")
    description = info.get("description", "")

    panel_content = f"[bold]{title}[/bold]  [dim]v{version}[/dim]"
    if description:
        panel_content += f"\n\n{description}"

    servers = spec.get("servers", [])
    if servers:
        panel_content += "\n\n[dim]Servers:[/dim]"
        for server in servers:
            url = server.get("url", "")
            desc = server.get("description", "")
            panel_content += f"\n  {url}"
            if desc:
                panel_content += f"  [dim]({desc})[/dim]"

    console.print(Panel(panel_content, title="[bold cyan]API情報[/bold cyan]", expand=False))


def display_endpoints_table(spec: dict[str, Any], tag_filter: str | None = None) -> None:
    """エンドポイント一覧をテーブルで表示する。"""
    paths = spec.get("paths", {})
    components = spec.get("components", {})

    # タグ別にグループ化
    by_tag: dict[str, list[tuple[str, str, dict]]] = {}
    for path, path_item in paths.items():
        for method, operation in path_item.items():
            if method not in METHOD_COLORS:
                continue
            tags = operation.get("tags", ["default"])
            for tag in tags:
                by_tag.setdefault(tag, []).append((method, path, operation))

    if not by_tag:
        console.print("[dim]エンドポイントが見つかりませんでした。[/dim]")
        return

    for tag, endpoints in sorted(by_tag.items()):
        if tag_filter and tag.lower() != tag_filter.lower():
            continue

        table = Table(
            show_header=True,
            header_style="bold",
            expand=True,
            title=f"[bold]{tag}[/bold]",
            title_justify="left",
        )
        table.add_column("Method", style="bold", width=8, no_wrap=True)
        table.add_column("Path", no_wrap=True)
        table.add_column("Summary")
        table.add_column("OperationId", style="dim", no_wrap=True)

        for method, path, operation in sorted(endpoints, key=lambda x: x[1]):
            color = METHOD_COLORS.get(method, "white")
            method_text = Text(method.upper(), style=f"bold {color}")
            summary = operation.get("summary", operation.get("description", ""))
            op_id = operation.get("operationId", "")
            table.add_row(method_text, path, summary, op_id)

        console.print(table)
        console.print()


def display_endpoint_detail(
    path: str,
    method: str,
    operation: dict[str, Any],
    components: dict[str, Any],
) -> None:
    """エンドポイントの詳細を表示する。"""
    color = METHOD_COLORS.get(method, "white")
    title = f"[bold {color}]{method.upper()}[/bold {color}] [bold]{path}[/bold]"

    lines = []

    summary = operation.get("summary", "")
    description = operation.get("description", "")
    if summary:
        lines.append(f"[bold]{summary}[/bold]")
    if description and description != summary:
        lines.append(description)

    # パラメータ
    params = operation.get("parameters", [])
    if params:
        lines.append("\n[underline]Parameters[/underline]")
        for param in params:
            param = resolve_ref(param, components)
            name = param.get("name", "")
            location = param.get("in", "")
            required = param.get("required", False)
            schema = param.get("schema", {})
            type_str = schema_to_str(schema, components)
            description = param.get("description", "")
            req_marker = "[red]*[/red]" if required else "[dim]?[/dim]"
            desc_str = f" — {description}" if description else ""
            lines.append(f"  {req_marker} [cyan]{name}[/cyan] ([dim]{location}[/dim]): {type_str}{desc_str}")

    # リクエストボディ
    request_body = operation.get("requestBody", {})
    if request_body:
        lines.append("\n[underline]Request Body[/underline]")
        required = request_body.get("required", False)
        req_marker = "[red]required[/red]" if required else "[dim]optional[/dim]"
        content = request_body.get("content", {})
        for content_type, media in content.items():
            schema = media.get("schema", {})
            type_str = schema_to_str(schema, components)
            lines.append(f"  {req_marker} [{content_type}]\n{type_str}")

    # レスポンス
    responses = operation.get("responses", {})
    if responses:
        lines.append("\n[underline]Responses[/underline]")
        for status_code, response in sorted(responses.items()):
            response = resolve_ref(response, components)
            resp_desc = response.get("description", "")
            content = response.get("content", {})

            # ステータスコードの色
            if str(status_code).startswith("2"):
                code_style = "green"
            elif str(status_code).startswith("4"):
                code_style = "yellow"
            elif str(status_code).startswith("5"):
                code_style = "red"
            else:
                code_style = "white"

            lines.append(f"  [{code_style}]{status_code}[/{code_style}] {resp_desc}")
            for content_type, media in content.items():
                schema = media.get("schema", {})
                type_str = schema_to_str(schema, components)
                lines.append(f"    [{content_type}]: {type_str}")

    console.print(Panel("\n".join(lines), title=title, expand=False))


def display_all_details(spec: dict[str, Any], tag_filter: str | None = None) -> None:
    """全エンドポイントの詳細を表示する。"""
    paths = spec.get("paths", {})
    components = spec.get("components", {})

    for path, path_item in sorted(paths.items()):
        for method, operation in path_item.items():
            if method not in METHOD_COLORS:
                continue
            tags = operation.get("tags", ["default"])
            if tag_filter and not any(t.lower() == tag_filter.lower() for t in tags):
                continue
            display_endpoint_detail(path, method, operation, components)


def display_schemas(spec: dict[str, Any]) -> None:
    """コンポーネントスキーマを表示する。"""
    components = spec.get("components", {})
    schemas = components.get("schemas", {})

    if not schemas:
        console.print("[dim]スキーマ定義が見つかりませんでした。[/dim]")
        return

    console.print(Panel("[bold]Component Schemas[/bold]", expand=False))
    for name, schema in sorted(schemas.items()):
        type_str = schema_to_str(schema, components)
        description = schema.get("description", "")
        title_line = f"[bold cyan]{name}[/bold cyan]"
        if description:
            title_line += f"\n[dim]{description}[/dim]"
        console.print(Panel(f"{title_line}\n\n{type_str}", expand=False))


@app.command()
def main(
    openapi_path: Path = typer.Argument(..., help="openapi.jsonのパス"),
    summary: bool = typer.Option(False, "--summary", "-s", help="エンドポイント一覧のみ表示（詳細省略）"),
    tag: str | None = typer.Option(None, "--tag", "-t", help="表示するタグでフィルタ"),
    schemas_only: bool = typer.Option(False, "--schemas-only", help="スキーマ定義のみ表示"),
) -> None:
    """OpenAPI JSON仕様を読み取り、構造化して表示する。"""
    if not openapi_path.exists():
        console.print(f"[red]エラー:[/red] ファイルが見つかりません: {openapi_path}")
        raise typer.Exit(1)

    try:
        spec = json.loads(openapi_path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as e:
        console.print(f"[red]エラー:[/red] JSONのパースに失敗しました: {e}")
        raise typer.Exit(1)

    openapi_version = spec.get("openapi", "")
    if not openapi_version.startswith("3."):
        console.print(f"[yellow]警告:[/yellow] OpenAPI 3.x 以外の仕様です ({openapi_version})。表示が不完全な可能性があります。")

    if schemas_only:
        display_schemas(spec)
        return

    display_info(spec)

    if summary:
        display_endpoints_table(spec, tag)
    else:
        display_endpoints_table(spec, tag)
        console.print()
        display_all_details(spec, tag)
        console.print()
        display_schemas(spec)


if __name__ == "__main__":
    app()
