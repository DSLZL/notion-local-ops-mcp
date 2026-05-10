def next_backoff_seconds(
    *,
    base: float,
    max_seconds: float,
    failures: int,
    jitter: float,
) -> float:
    safe_base = max(0.0, base)
    safe_max = max(0.0, max_seconds)
    raw = safe_base * (2 ** max(0, failures)) + max(0.0, jitter)
    return max(0.0, min(safe_max, raw))
