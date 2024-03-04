# SPDX-License-Identifier: GPL-3.0-or-later

from typing import Union

import bpy

from . import manager_info


_enum_items: list[Union[tuple[str, str, str], tuple[str, str, str, int, int]]] = []


def _get_enum_items(self, context):
    global _enum_items

    manager = manager_info.load_cached()
    if manager is None:
        _enum_items = [
            (
                "-",
                "-tags unknown-",
                "Refresh to load the available Worker tags from the Manager",
            ),
        ]
        return _enum_items

    _enum_items = [
        ("-", "All", "No specific tag assigned, any worker can handle this job"),
    ]
    for tag in manager.worker_tags.tags:
        _enum_items.append((tag.id, tag.name, getattr(tag, "description", "")))

    return _enum_items


def register() -> None:
    bpy.types.Scene.flamenco_worker_tag = bpy.props.EnumProperty(
        name="Worker Tag",
        items=_get_enum_items,
        description="The set of Workers that can handle tasks of this job",
    )


def unregister() -> None:
    to_del = ((bpy.types.Scene, "flamenco_worker_tag"),)
    for ob, attr in to_del:
        try:
            delattr(ob, attr)
        except AttributeError:
            pass
