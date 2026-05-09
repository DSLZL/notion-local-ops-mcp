def next_backoff_seconds(
    *,
    base: float,
    max_seconds: float,
    failures: int,
    jitter: float,
) -> float:
    raw = base * (2 ** max(0, failures)) + max(0.0, jitter)
    return min(max_seconds, raw)
