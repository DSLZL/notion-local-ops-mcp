from __future__ import annotations

import pytest

from notion_local_ops_mcp.tunnel_ngrok import NgrokHealth, extract_public_url


def test_extract_public_url_prefers_https_endpoint_when_http_and_https_exist() -> None:
    payload = {
        "tunnels": [
            {"public_url": "http://demo.ngrok-free.app"},
            {"public_url": "https://demo.ngrok-free.app"},
        ]
    }

    assert extract_public_url(payload) == "https://demo.ngrok-free.app"


@pytest.mark.parametrize(
    "payload",
    [
        None,
        {},
        {"tunnels": []},
        {"tunnels": None},
        {"tunnels": "not-a-list"},
        {"tunnels": [{}, {"public_url": None}]},
    ],
)
def test_extract_public_url_returns_none_when_tunnels_missing_or_empty(payload: object) -> None:
    assert extract_public_url(payload) is None


def test_ngrok_health_url_changed_to_detects_url_change() -> None:
    previous = NgrokHealth(
        process_alive=True,
        public_url="https://old.ngrok-free.app",
        error=None,
    )
    current = NgrokHealth(
        process_alive=True,
        public_url="https://new.ngrok-free.app",
        error=None,
    )
    unchanged = NgrokHealth(
        process_alive=True,
        public_url="https://old.ngrok-free.app",
        error="irrelevant",
    )
    missing_new_url = NgrokHealth(process_alive=True, public_url=None, error=None)

    assert previous.url_changed_to(current) == "https://new.ngrok-free.app"
    assert previous.url_changed_to(unchanged) is None
    assert previous.url_changed_to(missing_new_url) is None
