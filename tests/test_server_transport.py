from notion_local_ops_mcp.server import build_http_app


def test_http_app_uses_sse_transport() -> None:
    app = build_http_app()

    assert app.state.transport_type == "sse"
    assert app.state.path == "/mcp"
