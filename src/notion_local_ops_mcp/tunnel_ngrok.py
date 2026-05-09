from __future__ import annotations

from dataclasses import dataclass
from typing import Mapping


@dataclass(slots=True)
class NgrokHealth:
    process_alive: bool
    public_url: str | None
    error: str | None

    def url_changed_to(self, other: object) -> str | None:
        if not isinstance(other, NgrokHealth):
            return None
        if other.public_url and self.public_url != other.public_url:
            return other.public_url
        return None


def extract_public_url(payload: Mapping[str, object] | object | None) -> str | None:
    if not isinstance(payload, Mapping):
        return None

    tunnels = payload.get("tunnels")
    if not isinstance(tunnels, list) or not tunnels:
        return None

    first_valid: str | None = None
    for entry in tunnels:
        if not isinstance(entry, dict):
            continue

        public_url = entry.get("public_url")
        if not isinstance(public_url, str):
            continue
        public_url = public_url.strip()
        if not public_url or "://" not in public_url:
            continue

        if first_valid is None:
            first_valid = public_url
        if public_url.startswith("https://"):
            return public_url

    return first_valid
