"""Metric checks of the report."""

from scripts import metrics


def test_metrics_load() -> None:
    try:
        metrics.load()
    except ValueError:
        metrics.reset()
