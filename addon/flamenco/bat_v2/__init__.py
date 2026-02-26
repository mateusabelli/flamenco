# SPDX-FileCopyrightText: 2026 Blender Authors
# SPDX-License-Identifier: GPL-3.0-or-later

"""BAT v2 support for Flamenco.

NOTE: This module uses late imports to avoid importing BAT v2 until it's really
necessary. Functions should _only_ be called from Blender 5.1+ (Python 3.13+),
as BAT uses language features that were not available in 5.0 (Python 3.11).
"""
