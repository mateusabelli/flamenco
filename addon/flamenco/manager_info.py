# SPDX-License-Identifier: GPL-3.0-or-later
# <pep8 compliant>

import dataclasses
import json
import platform
from pathlib import Path
from typing import TYPE_CHECKING, Optional, Union

from urllib3.exceptions import HTTPError, MaxRetryError

import bpy

if TYPE_CHECKING:
    from flamenco.manager import ApiClient as _ApiClient
    from flamenco.manager.models import (
        AvailableJobTypes as _AvailableJobTypes,
        FlamencoVersion as _FlamencoVersion,
        SharedStorageLocation as _SharedStorageLocation,
        WorkerTagList as _WorkerTagList,
    )
else:
    _ApiClient = object
    _AvailableJobTypes = object
    _FlamencoVersion = object
    _SharedStorageLocation = object
    _WorkerTagList = object


@dataclasses.dataclass
class ManagerInfo:
    """Cached information obtained from a Flamenco Manager.

    This is the root object of what is stored on disk, every time someone
    presses a 'refresh' button to update worker tags, job types, etc.
    """

    flamenco_version: _FlamencoVersion
    shared_storage: _SharedStorageLocation
    job_types: _AvailableJobTypes
    worker_tags: _WorkerTagList

    @staticmethod
    def type_info() -> dict[str, type]:
        # Do a late import, so that the API is only imported when actually used.
        from flamenco.manager.models import (
            AvailableJobTypes,
            FlamencoVersion,
            SharedStorageLocation,
            WorkerTagList,
        )

        # These types cannot be obtained by introspecting the ManagerInfo class, as
        # at runtime that doesn't use real type annotations.
        return {
            "flamenco_version": FlamencoVersion,
            "shared_storage": SharedStorageLocation,
            "job_types": AvailableJobTypes,
            "worker_tags": WorkerTagList,
        }


class FetchError(RuntimeError):
    """Raised when the manager info could not be fetched from the Manager."""


class LoadError(RuntimeError):
    """Raised when the manager info could not be loaded from disk cache."""


_cached_manager_info: Optional[ManagerInfo] = None


def fetch(api_client: _ApiClient) -> ManagerInfo:
    global _cached_manager_info

    # Do a late import, so that the API is only imported when actually used.
    from flamenco.manager import ApiException
    from flamenco.manager.apis import MetaApi, JobsApi, WorkerMgtApi
    from flamenco.manager.models import (
        AvailableJobTypes,
        FlamencoVersion,
        SharedStorageLocation,
        WorkerTagList,
    )

    meta_api = MetaApi(api_client)
    jobs_api = JobsApi(api_client)
    worker_mgt_api = WorkerMgtApi(api_client)

    try:
        flamenco_version: FlamencoVersion = meta_api.get_version()
        shared_storage: SharedStorageLocation = meta_api.get_shared_storage(
            "users", platform.system().lower()
        )
        job_types: AvailableJobTypes = jobs_api.get_job_types()
        worker_tags: WorkerTagList = worker_mgt_api.fetch_worker_tags()
    except ApiException as ex:
        raise FetchError("Manager cannot be reached: %s" % ex) from ex
    except MaxRetryError as ex:
        # This is the common error, when for example the port number is
        # incorrect and nothing is listening. The exception text is not included
        # because it's very long and confusing.
        raise FetchError("Manager cannot be reached") from ex
    except HTTPError as ex:
        raise FetchError("Manager cannot be reached: %s" % ex) from ex

    _cached_manager_info = ManagerInfo(
        flamenco_version=flamenco_version,
        shared_storage=shared_storage,
        job_types=job_types,
        worker_tags=worker_tags,
    )
    return _cached_manager_info


class Encoder(json.JSONEncoder):
    def default(self, o):
        from flamenco.manager.model_utils import OpenApiModel

        if isinstance(o, OpenApiModel):
            return o.to_dict()

        if isinstance(o, ManagerInfo):
            # dataclasses.asdict() creates a copy of the OpenAPI models,
            # in a way that just doesn't work, hence this workaround.
            return {f.name: getattr(o, f.name) for f in dataclasses.fields(o)}

        return super().default(o)


def _to_json(info: ManagerInfo) -> str:
    return json.dumps(info, indent="  ", cls=Encoder)


def _from_json(contents: Union[str, bytes]) -> ManagerInfo:
    # Do a late import, so that the API is only imported when actually used.
    from flamenco.manager.configuration import Configuration
    from flamenco.manager.model_utils import validate_and_convert_types

    json_dict = json.loads(contents)
    dummy_cfg = Configuration()
    api_models = {}

    for name, api_type in ManagerInfo.type_info().items():
        api_model = validate_and_convert_types(
            json_dict[name],
            (api_type,),
            [name],
            True,
            True,
            dummy_cfg,
        )
        api_models[name] = api_model

    return ManagerInfo(**api_models)


def _json_filepath() -> Path:
    # This is the '~/.config/blender/{version}' path.
    user_path = Path(bpy.utils.resource_path(type="USER"))
    return user_path / "config" / "flamenco-manager-info.json"


def save(info: ManagerInfo) -> None:
    json_path = _json_filepath()
    json_path.parent.mkdir(parents=True, exist_ok=True)

    as_json = _to_json(info)
    json_path.write_text(as_json, encoding="utf8")


def load() -> ManagerInfo:
    json_path = _json_filepath()
    if not json_path.exists():
        raise FileNotFoundError(f"{json_path.name} not found in {json_path.parent}")

    try:
        as_json = json_path.read_text(encoding="utf8")
    except OSError as ex:
        raise LoadError(f"Could not read {json_path}: {ex}") from ex

    try:
        return _from_json(as_json)
    except json.JSONDecodeError as ex:
        raise LoadError(f"Could not decode JSON in {json_path}") from ex


def load_into_cache() -> Optional[ManagerInfo]:
    global _cached_manager_info

    _cached_manager_info = None
    try:
        _cached_manager_info = load()
    except FileNotFoundError:
        return None
    except LoadError as ex:
        print(f"Could not load Flamenco Manager info from disk: {ex}")
        return None

    return _cached_manager_info


def load_cached() -> Optional[ManagerInfo]:
    global _cached_manager_info

    if _cached_manager_info is not None:
        return _cached_manager_info

    return load_into_cache()
