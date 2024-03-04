# SPDX-License-Identifier: GPL-3.0-or-later

# <pep8 compliant>

import logging
from typing import TYPE_CHECKING

import bpy

from flamenco import manager_info, job_types

_flamenco_client = None
_log = logging.getLogger(__name__)

if TYPE_CHECKING:
    from flamenco.manager import ApiClient as _ApiClient
    from flamenco.manager.models import (
        FlamencoVersion as _FlamencoVersion,
        SharedStorageLocation as _SharedStorageLocation,
    )
    from .preferences import FlamencoPreferences as _FlamencoPreferences
else:
    _ApiClient = object
    _FlamencoPreferences = object
    _FlamencoVersion = object
    _SharedStorageLocation = object


def flamenco_api_client(manager_url: str) -> _ApiClient:
    """Returns an API client for communicating with a Manager."""
    global _flamenco_client

    if _flamenco_client is not None:
        return _flamenco_client

    from . import dependencies

    dependencies.preload_modules()

    from . import manager

    configuration = manager.Configuration(host=manager_url.rstrip("/"))
    _flamenco_client = manager.ApiClient(configuration)
    _log.info("created API client for Manager at %s", manager_url)

    return _flamenco_client


def flamenco_client_version() -> str:
    """Return the version of the Flamenco OpenAPI client."""

    from . import dependencies

    dependencies.preload_modules()

    from . import manager

    return manager.__version__


def discard_flamenco_data():
    global _flamenco_client

    if _flamenco_client is None:
        return

    _log.info("closing Flamenco client")
    _flamenco_client.close()
    _flamenco_client = None


def ping_manager(
    window_manager: bpy.types.WindowManager,
    scene: bpy.types.Scene,
    api_client: _ApiClient,
) -> tuple[str, str]:
    """Fetch Manager info, and update the scene for it.

    :returns: tuple (report, level). The report will be something like "<name>
        version <version> found", or an error message. The level will be
        'ERROR', 'WARNING', or 'INFO', suitable for reporting via
        `Operator.report()`.
    """

    window_manager.flamenco_status_ping = "..."

    # Remember the old values, as they may have disappeared from the Manager.
    old_job_type_name = getattr(scene, "flamenco_job_type", "")
    old_tag_name = getattr(scene, "flamenco_worker_tag", "")

    try:
        info = manager_info.fetch(api_client)
    except manager_info.FetchError as ex:
        report = str(ex)
        window_manager.flamenco_status_ping = report
        return report, "ERROR"

    manager_info.save(info)

    report = "%s version %s found" % (
        info.flamenco_version.name,
        info.flamenco_version.version,
    )
    report_level = "INFO"

    job_types.refresh_scene_properties(scene, info.job_types)

    # Try to restore the old values.
    #
    # Since you cannot un-set an enum property, and 'empty string' is not a
    # valid value either, when the old choice is no longer available we remove
    # the underlying ID property.
    if old_job_type_name:
        try:
            scene.flamenco_job_type = old_job_type_name
        except TypeError:  # Thrown when the old enum value no longer exists.
            del scene["flamenco_job_type"]
            report = f"Job type {old_job_type_name!r} no longer available, choose another one"
            report_level = "WARNING"

    if old_tag_name:
        try:
            scene.flamenco_worker_tag = old_tag_name
        except TypeError:  # Thrown when the old enum value no longer exists.
            del scene["flamenco_worker_tag"]
            report = f"Tag {old_tag_name!r} no longer available, choose another one"
            report_level = "WARNING"

    window_manager.flamenco_status_ping = report
    return report, report_level
