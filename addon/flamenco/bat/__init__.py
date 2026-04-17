# SPDX-FileCopyrightText: 2026 Blender Authors
# SPDX-License-Identifier: GPL-3.0-or-later

# This is the interface to BAT v1.x


def bat_version() -> str:
    from .submodules import bat_toplevel

    bat_version: str = bat_toplevel.__version__
    return bat_version
