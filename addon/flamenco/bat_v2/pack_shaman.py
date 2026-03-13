# SPDX-FileCopyrightText: 2026 Blender Authors
# SPDX-License-Identifier: GPL-3.0-or-later

"""BAT v2 packing to a Shaman server."""

from __future__ import annotations

__all__ = ("pack_start",)

import dataclasses
import email.header
import logging
import random
from functools import partial
from pathlib import Path, PurePosixPath
from typing import TYPE_CHECKING, Any, TypeAlias

import bpy

if TYPE_CHECKING:
    # _BATPacker: TypeAlias = pack.BATPacker
    from ..manager import ApiClient as _ApiClient
    from ..manager.apis import ShamanApi as _ShamanApi
    from ..manager.models import (
        ShamanCheckoutResult as _ShamanCheckoutResult,
    )
    from ..manager.models import (
        ShamanFileSpec as _ShamanFileSpec,
    )
    from ..manager.models import (
        ShamanRequirementsRequest as _ShamanRequirementsRequest,
    )
    from .submodules.file_usage import FileInfo as _FileInfo
    from .submodules.pack import BATPacker as _BATPacker
    from .submodules.pack import QueueingExecutor as _QueueingExecutor
else:
    _ApiClient = object
    _ShamanApi = object
    _ShamanCheckoutResult = object
    _ShamanRequirementsRequest = object
    _ShamanFileSpec = object
    _BATPacker = object
    _FileInfo = object
    _QueueingExecutor = object

log = logging.getLogger(__name__)

MAX_DEFERRED_PATHS = 8
MAX_FAILED_PATHS = 8
HASH_STORAGE_PATH = Path(bpy.app.cachedir) / "flamenco/shaman"
HASH_METHOD = "sha256"

HashableShamanFileSpec = tuple[str, int, str]
"""Tuple of the 'sha', 'size', and 'path' fields of a ShamanFileSpec."""

# Alias some types from blender_asset_tracer so that we can use type annotations
# without having to import from BATv2.
BATPackReporter: TypeAlias = Any
BATPacker: TypeAlias = Any


def pack_start(
    project_root: Path,
    reporter: BATPackReporter,
    *,
    use_relative_only: bool,
    api_client: _ApiClient,
    checkout_path: PurePosixPath,
) -> BATPacker:
    """Investigate what's needed to create a BAT pack."""
    from ..manager.apis import ShamanApi
    from .submodules import file_usage, pack

    shaman_api = ShamanApi(api_client)
    executor = pack.QueueingExecutor()
    shaman_transferer = ShamanPacker(shaman_api, checkout_path, executor)

    batpacker = pack.BATPacker(
        project_root,
        file_usage.Options(
            use_relative_only=use_relative_only,
        ),
        reporter,
        file_transfer=shaman_transferer,
    )

    return batpacker


@dataclasses.dataclass
class ShamanUploadProgress:
    # When another client is already uploading a file that we also want to
    # upload, we defer the file. That way, when we do get around to uploading
    # it, the other person may already have finished their upload, saving us
    # time.
    #
    # Mapping from 'path in pack' to the BAT FileInfo and Shaman FileSpec.
    deferred: dict[PurePosixPath, tuple[_FileInfo, _ShamanFileSpec]] = (
        dataclasses.field(default_factory=dict)
    )

    # When a file doesn't want to get uploaded, it's stored here to retry. If
    # too many files fail, or the retry counter reaches its max, it'll get
    # reported as an actual error.
    # The string value is the error message.
    failures: dict[PurePosixPath, tuple[_FileInfo, _ShamanFileSpec, str]] = (
        dataclasses.field(default_factory=dict)
    )

    retry_counter: int = 0
    max_retries: int = 50

    def is_deferred(self, relpath_in_pack: PurePosixPath) -> bool:
        return relpath_in_pack in self.deferred

    @property
    def num_deferred(self) -> int:
        return len(self.deferred)

    @property
    def num_failed(self) -> int:
        return len(self.failures)

    def defer(
        self,
        bat_file_info: _FileInfo,
        shaman_file_spec: _ShamanFileSpec,
    ) -> None:
        # A file should only be deferred once. Once an upload has been deferred,
        # the next attempt shouldn't be deferred again.
        assert bat_file_info.relpath_in_pack not in self.deferred

        self.deferred[bat_file_info.relpath_in_pack] = (bat_file_info, shaman_file_spec)

    def failed(
        self,
        bat_file_info: _FileInfo,
        shaman_file_spec: _ShamanFileSpec,
        errormsg: str,
    ) -> None:
        # A file should only be added to the 'failures' dict once. When its
        # upload is retried, it should be removed from the 'failures' dict first.
        assert bat_file_info.relpath_in_pack not in self.failures

        self.failures[bat_file_info.relpath_in_pack] = (
            bat_file_info,
            shaman_file_spec,
            errormsg,
        )


@dataclasses.dataclass
class ShamanPacker:
    shaman_api: _ShamanApi
    checkout_path: PurePosixPath
    executor: _QueueingExecutor

    # The reporter is not passed on construction, but rather it's taken from the
    # batpacker passed to ShamanPacker.start(). That way, it uses the
    # batpacker's reporter, which makes the batpacker know about its calls.
    #
    # See BATPackReporterWrapper and BATPacker.run_for() in BAT's blender_asset_tracer/pack.py.
    reporter: BATPackReporter | None = None

    # Shaman may decide to create the checkout at another path than requested.
    # This will be set to the actually-used path.
    checkout_path_final: PurePosixPath | None = None

    _num_files_to_transfer_total: int = -1
    _num_files_to_transfer_done: int = 0

    @property
    def is_succes(self) -> bool:
        """Return whether the Shaman operation was completed succesfully."""
        return bool(self.checkout_path_final)

    def start(self, batpacker: _BATPacker) -> None:
        self.reporter = batpacker.reporter
        files_to_copy = batpacker.all_files_to_copy()

        # Initial value is the total number of files to copy. Once the Shaman
        # server has told us how many files to submit, this will be adjusted.
        self._num_files_to_transfer_total = len(files_to_copy)
        self._num_files_to_transfer_done = 0

        self.executor.queue(partial(self._step_queue_hashing, files_to_copy))

    def step(self) -> bool:
        """Perform a single step in the Shaman file transfer.

        Returns whether there are more steps to do (True) or the process is done (False).
        """
        if self.executor.is_done:
            return False
        self.executor.run_step()
        return not self.executor.is_done

    def blendfile_location_in_pack(self) -> PurePosixPath:
        assert self.checkout_path_final is not None
        return self.checkout_path_final

    def num_files_to_transfer(self) -> tuple[int, int]:
        """Return the number of files that need to be transferred.

        This is a tuple [total, done] with the total number of files to
        transfer, and the number of transferred files so far.

        The number may change during the packing process, as it takes time
        for the Shaman protocol to get this information. Or some paths may
        turn out to be multiple paths (UDIMs for example).
        """
        return self._num_files_to_transfer_total, self._num_files_to_transfer_done

    def _step_queue_hashing(self, files_to_copy: dict[Path, _FileInfo]) -> None:
        from ..manager.models import ShamanRequirementsRequest

        # Shaman Spec that's shared between the queued function calls. They
        # can all just append to the same list.
        shaman_spec = ShamanRequirementsRequest(files=[])
        assert isinstance(shaman_spec, ShamanRequirementsRequest)

        # Tracks deferred files and failed uploads.
        upload_progress = ShamanUploadProgress()

        # Queue up all the hash computations.
        for file_info in files_to_copy.values():
            self.executor.queue(
                partial(
                    self._step_hash_file,
                    file_info,
                    shaman_spec,
                )
            )

        # After the hashes are gathered in 'filespecs', send the spec to Shaman.
        self.executor.queue(
            partial(
                self._step_queue_uploads_of_files,
                files_to_copy,
                shaman_spec,
                upload_progress,
            )
        )

    def _step_hash_file(
        self,
        file_info: _FileInfo,
        shaman_spec: _ShamanRequirementsRequest,
    ) -> None:
        assert self.reporter is not None
        from _bpy_internal import disk_file_hash_service

        from ..manager.models import ShamanFileSpec

        path_to_pack = file_info.path_to_pack

        if not path_to_pack.exists():
            # If the file is missing, there's little else to do than reporting
            # it as such and continue with the next file.
            if path_to_pack == file_info.source_path:
                log.info("File missing: %s", path_to_pack)
            else:
                log.info(
                    "File missing after rewriting %s to %s",
                    file_info.source_path,
                    path_to_pack,
                )
            self.reporter.on_missing_file(
                file_info.source_path, file_info.relpath_in_pack
            )
            return

        # It might be tempting to use the same Disk File Hash Service as BAT's
        # path rewriting system is using. However, that only hashes the files
        # that need rewriting, and the code below only deals with paths after
        # rewriting (or where rewriting was not necessary). That means that
        # there is no benefit in sharing the same database.
        dfhs = disk_file_hash_service.get_service(HASH_STORAGE_PATH)
        checksum = dfhs.get_hash(path_to_pack, HASH_METHOD)

        filesize = path_to_pack.stat().st_size

        filespec = ShamanFileSpec(
            sha=checksum,
            size=filesize,
            path=str(file_info.relpath_in_pack),
        )
        assert isinstance(filespec, ShamanFileSpec)
        shaman_spec.files.append(filespec)

    def _step_queue_uploads_of_files(
        self,
        files_to_copy: dict[Path, _FileInfo],
        shaman_spec: _ShamanRequirementsRequest,
        upload_progress: ShamanUploadProgress,
    ) -> None:
        """Send the spec file to Shaman, and queue file uploads."""

        # Query Shaman to figure out which files still need uploading.
        to_upload = self._send_spec_to_shaman(shaman_spec)
        if to_upload is None:
            # Errors have been reported already, so just stop.
            return

        log.info(
            "Feeding %d/%d files to the Shaman", len(to_upload), len(shaman_spec.files)
        )
        self._num_files_to_transfer_total = len(to_upload)

        # Create a mapping from the path in the pack (which is used in
        # `filespecs`) to the FileInfo.
        path_in_pack_to_abs: dict[str, _FileInfo] = {
            str(file_info.relpath_in_pack): file_info
            for file_info in files_to_copy.values()
        }

        # Queue the file uploads.
        for index, file_spec in enumerate(to_upload):
            file_info = path_in_pack_to_abs[file_spec.path]
            is_last_file = index == len(to_upload)
            self.executor.queue(
                partial(
                    self._step_upload_file,
                    file_info,
                    file_spec,
                    is_last_file,
                    upload_progress,
                )
            )
        self.executor.queue(
            partial(
                self._step_check_upload_success,
                files_to_copy,
                shaman_spec,
                upload_progress,
            )
        )

    def _step_upload_file(
        self,
        file_info: _FileInfo,
        file_spec: _ShamanFileSpec,
        is_last_file: bool,
        upload_progress: ShamanUploadProgress,
    ) -> None:
        assert self.reporter is not None
        from ..manager.exceptions import ApiException

        # Pre-flight check. The generated API code will load the entire file
        # into memory before sending it to the Shaman. It's faster to do a check
        # at Shaman first, to see if we need uploading at all.
        check_resp = self.shaman_api.shaman_file_store_check(
            checksum=file_spec.sha,
            filesize=file_spec.size,
        )
        if check_resp.status.value == "stored":
            log.info("  %s: skipping, already on server", file_spec.path)
            return

        # See whether we may be able to defer uploading this file or not.
        can_defer = bool(
            not is_last_file
            and upload_progress.num_deferred < MAX_DEFERRED_PATHS
            and not upload_progress.is_deferred(file_info.relpath_in_pack)
        )

        filename_header = _encode_original_filename_header(file_spec.path)
        self.reporter.on_copy_start(file_info.source_path, file_info.relpath_in_pack)
        try:
            with file_info.path_to_pack.open("rb") as file_reader:
                self.shaman_api.shaman_file_store(
                    checksum=file_spec.sha,
                    filesize=file_spec.size,
                    body=file_reader,
                    x_shaman_can_defer_upload=can_defer,
                    x_shaman_original_filename=filename_header,
                )
        except ApiException as ex:
            if ex.status == 425:
                # Too Early, i.e. defer uploading this file.
                log.info(
                    "  %s: someone else is uploading this file, deferring",
                    file_spec.path,
                )
                upload_progress.defer(file_info, file_spec)
                return
            elif ex.status == 417:
                # Expectation Failed; mismatch of checksum or file size.
                msg = "Error from Shaman uploading %s, code %d: %s" % (
                    file_spec.path,
                    ex.status,
                    ex.body,
                )
            else:  # Unknown error
                msg = "API exception\nHeaders: %s\nBody: %s\n" % (
                    ex.headers,
                    ex.body,
                )

            log.error(msg)
            upload_progress.failed(file_info, file_spec, msg)
            return

        self._num_files_to_transfer_done += 1
        self.reporter.on_copy_done(file_info.source_path, file_info.relpath_in_pack)

    def _step_check_upload_success(
        self,
        files_to_copy: dict[Path, _FileInfo],
        shaman_spec: _ShamanRequirementsRequest,
        upload_progress: ShamanUploadProgress,
    ) -> None:
        """See if there were any deferred or failed files.

        If there were, re-queue the uploading of the remaining files.
        Unless the number of retries has been exceeded, in which case the
        failures are final.
        """
        assert self.reporter is not None

        if upload_progress.num_deferred == 0 and upload_progress.num_failed == 0:
            # Nothing left to do, so move on to the next stage.
            self.executor.queue(partial(self._step_request_checkout, shaman_spec))
            return

        upload_progress.retry_counter += 1
        if upload_progress.retry_counter >= upload_progress.max_retries:
            # Failed uploads have really failed now.
            #
            # Deferred uploads shouldn't be mentioned, because they only get
            # deferred on the first upload attempt. After that, if they fail,
            # they get into the failures.
            for fileinfo, _, errormsg in upload_progress.failures.values():
                self.reporter.on_copy_error(
                    fileinfo.source_path, fileinfo.relpath_in_pack, errormsg
                )
            return

        # Retry uploading.
        self.executor.queue(
            partial(
                self._step_queue_uploads_of_files,
                files_to_copy,
                shaman_spec,
                upload_progress,
            )
        )

    def _step_request_checkout(self, shaman_spec: _ShamanRequirementsRequest) -> None:
        """Ask the Shaman to create a checkout of this BAT pack."""
        assert self.checkout_path
        assert self.reporter is not None

        from ..manager.exceptions import ApiException
        from ..manager.models import ShamanCheckout, ShamanCheckoutResult

        log.info(
            "Requesting checkout at Shaman for checkout_path=%s", self.checkout_path
        )

        checkoutRequest = ShamanCheckout(
            files=shaman_spec.files,
            checkout_path=str(self.checkout_path),
        )

        try:
            result: ShamanCheckoutResult = self.shaman_api.shaman_checkout(
                checkoutRequest
            )
        except ApiException as ex:
            if ex.status == 424:  # Files were missing
                msg = "We did not upload some files, checkout aborted"
            elif ex.status == 409:  # Checkout already exists
                msg = "There is already an existing checkout at %s" % self.checkout_path
            else:  # Unknown error
                msg = "API exception\nHeaders: %s\nBody: %s\n" % (
                    ex.headers,
                    ex.body,
                )
            log.error(msg)
            self.reporter.on_error(msg)
            return

        log.info("Shaman created checkout at %s", result.checkout_path)
        self.checkout_path_final = result.checkout_path

    def _send_spec_to_shaman(
        self,
        requirements: _ShamanRequirementsRequest,
    ) -> list[_ShamanFileSpec] | None:
        """Send the checkout definition file to the Shaman.

        :return: A list of file specs that still need to be uploaded, or
            None if there was an error.
        """
        assert self.reporter is not None
        from ..manager.exceptions import ApiException
        from ..manager.models import ShamanRequirementsResponse

        requested_relpaths = {file.path for file in requirements.files}

        try:
            resp = self.shaman_api.shaman_checkout_requirements(requirements)
        except ApiException as ex:
            # TODO: the body should be JSON of a predefined type, parse it to get the actual message.
            msg = "Error from Shaman, code %d: %s" % (ex.status, ex.body)
            log.error(msg)
            self.reporter.on_error(msg)
            return None
        assert isinstance(resp, ShamanRequirementsResponse)

        # Go over the response, and create two queues for uploading. Any file
        # that's already being uploaded by somebody else will be put in the
        # low-priority queue.
        to_upload_normal_prio: list[_ShamanFileSpec] = []
        to_upload_low_prio: list[_ShamanFileSpec] = []
        for file_spec in resp.files:
            if file_spec.path not in requested_relpaths:
                msg = (
                    "Shaman requested path we did not intend to upload: %r" % file_spec
                )
                log.error(msg)
                self.reporter.on_error(msg)
                return None

            log.debug("   %s: %s", file_spec.status, file_spec.path)
            status = file_spec.status.value
            if status == "unknown":
                to_upload_normal_prio.append(file_spec)
            elif status == "uploading":
                to_upload_low_prio.append(file_spec)
            else:
                msg = "Unknown status in response from Shaman: %r" % file_spec
                log.error(msg)
                self.reporter.on_error(msg)
                return None

        # Randomize the two lists, so that when two clients upload similar sets
        # of files, collissions are minimized.
        random.shuffle(to_upload_normal_prio)
        random.shuffle(to_upload_low_prio)

        return to_upload_normal_prio + to_upload_low_prio


def _encode_original_filename_header(filename: str) -> str:
    """Encode the 'original filename' as valid HTTP Header.

    See the specs for the X-Shaman-Original-Filename header in the OpenAPI
    operation `shamanFileStore`, defined in flamenco-openapi.yaml.
    """

    # This is a no-op when the filename is already in ASCII.
    fake_header = email.header.Header()
    fake_header.append(filename, charset="utf-8")
    return fake_header.encode()
