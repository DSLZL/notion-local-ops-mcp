from notion_local_ops_mcp.runtime_manager import next_backoff_seconds


def test_next_backoff_seconds_caps_at_max() -> None:
    assert (
        next_backoff_seconds(
            base=1.0,
            max_seconds=10.0,
            failures=0,
            jitter=0.0,
        )
        == 1.0
    )
    assert (
        next_backoff_seconds(
            base=1.0,
            max_seconds=10.0,
            failures=1,
            jitter=0.0,
        )
        == 2.0
    )
    assert (
        next_backoff_seconds(
            base=1.0,
            max_seconds=10.0,
            failures=4,
            jitter=0.0,
        )
        == 10.0
    )


def test_next_backoff_seconds_clamps_negative_inputs() -> None:
    assert (
        next_backoff_seconds(
            base=-1.0,
            max_seconds=10.0,
            failures=2,
            jitter=0.0,
        )
        == 0.0
    )
    assert (
        next_backoff_seconds(
            base=1.0,
            max_seconds=-5.0,
            failures=2,
            jitter=0.0,
        )
        == 0.0
    )
    assert (
        next_backoff_seconds(
            base=1.0,
            max_seconds=10.0,
            failures=-2,
            jitter=-3.0,
        )
        == 1.0
    )
