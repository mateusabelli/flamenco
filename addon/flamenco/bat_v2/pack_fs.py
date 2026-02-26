# SPDX-FileCopyrightText: 2026 Blender Authors
# SPDX-License-Identifier: GPL-3.0-or-later

"""BAT v2 packing to the filesystem."""

from __future__ import annotations

__all__ = ("pack_start",)

from pathlib import Path
from typing import Any, TypeAlias

# Alias some types from blender_asset_tracer so that we can use type annotations
# without having to import from BATv2.
BATPackReporter: TypeAlias = Any
BATPacker: TypeAlias = Any


def pack_start(
    project_root: Path,
    reporter: BATPackReporter,
    *,
    use_relative_only: bool,
    pack_target_dir: Path,
) -> BATPacker:
    """Investigate what's needed to create a BAT pack."""
    from .submodules import file_usage, pack

    batpacker = pack.BATPacker(
        project_root,
        file_usage.Options(
            use_relative_only=use_relative_only,
        ),
        reporter,
        pack_target_dir=pack_target_dir,
    )
    return batpacker
