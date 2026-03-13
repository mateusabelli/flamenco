# SPDX-License-Identifier: GPL-3.0-or-later
# <pep8 compliant>

import datetime
import logging
import time
from pathlib import Path, PurePath, PurePosixPath
from types import ModuleType
from typing import TYPE_CHECKING, Optional

import bpy
from urllib3.exceptions import HTTPError, MaxRetryError

from . import job_submission, job_types, manager_info, preferences
from .bat.submodules import bpathlib
from .job_types_propgroup import JobTypePropertyGroup

if TYPE_CHECKING:
    from .bat.interface import (
        Message as _Message,
    )
    from .bat.interface import (
        PackThread as _PackThread,
    )
    from .manager.api_client import ApiClient as _ApiClient
    from .manager.exceptions import ApiException as _ApiException
    from .manager.models import (
        Error as _Error,
    )
    from .manager.models import (
        SubmittedJob as _SubmittedJob,
    )
else:
    _PackThread = object
    _Message = object
    _SubmittedJob = object
    _ApiClient = object
    _ApiException = object
    _Error = object

# Conditionally import BAT v2, as that version requires Blender 5.1+.
bat_v2: ModuleType | None
if bpy.app.version >= (5, 1, 0) or TYPE_CHECKING:
    from . import bat_v2
    from .bat_v2.pack_fs import BATPacker

    _BATPacker = BATPacker
else:
    bat_v2 = None
    _BATPacker = object

_log = logging.getLogger(__name__)


class FlamencoOpMixin:
    @staticmethod
    def get_api_client(context):
        """Get a Flamenco API client to talk to the Manager.

        Getting the client also loads the dependencies, so only import things
        from `flamenco.manager` after calling this function.
        """
        from . import comms, preferences

        manager_url = preferences.manager_url(context)
        api_client = comms.flamenco_api_client(manager_url)
        return api_client


class FLAMENCO_OT_ping_manager(FlamencoOpMixin, bpy.types.Operator):
    bl_idname = "flamenco.ping_manager"
    bl_label = "Flamenco: Ping Manager"
    bl_description = "Attempt to connect to the Manager"
    bl_options = {"REGISTER"}  # No UNDO.

    def execute(self, context: bpy.types.Context) -> set[str]:
        from . import comms

        api_client = self.get_api_client(context)
        report, level = comms.ping_manager(
            context.window_manager,
            context.scene,
            api_client,
        )
        self.report({level}, report)

        return {"FINISHED"}


class FLAMENCO_OT_eval_setting(FlamencoOpMixin, bpy.types.Operator):
    bl_idname = "flamenco.eval_setting"
    bl_label = "Flamenco: Evaluate Setting Value"
    bl_description = "Automatically determine a suitable value"
    bl_options = {"REGISTER", "INTERNAL", "UNDO"}

    setting_key: bpy.props.StringProperty(name="Setting Key")  # type: ignore
    setting_eval: bpy.props.StringProperty(name="Python Expression")  # type: ignore

    eval_description: bpy.props.StringProperty(name="Description", options={"HIDDEN"})  # type: ignore

    @classmethod
    def description(cls, context, properties):
        if not properties.eval_description:
            return ""  # Causes bl_description to be shown.
        return f"Set value to: {properties.eval_description}"

    def execute(self, context: bpy.types.Context) -> set[str]:
        job = job_submission.job_for_scene(context.scene)
        if job is None:
            self.report({"ERROR"}, "This Scene has no Flamenco job")
            return {"CANCELLED"}

        propgroup: JobTypePropertyGroup = context.scene.flamenco_job_settings
        propgroup.eval_and_assign(context, self.setting_key, self.setting_eval)
        return {"FINISHED"}


class FLAMENCO_OT_submit_job(FlamencoOpMixin, bpy.types.Operator):
    bl_idname = "flamenco.submit_job"
    bl_label = "Flamenco: Submit Job"
    bl_description = "Pack the current blend file and send it to Flamenco"
    bl_options = {"REGISTER"}  # No UNDO.

    blendfile_on_farm: Optional[PurePosixPath] = None
    actual_shaman_checkout_path: Optional[PurePosixPath] = None

    job_name: bpy.props.StringProperty(name="Job Name")  # type: ignore
    job: Optional[_SubmittedJob] = None
    temp_blendfile: Optional[Path] = None
    ignore_version_mismatch: bpy.props.BoolProperty(  # type: ignore
        name="Ignore Version Mismatch",
        default=False,
    )

    TIMER_PERIOD = 0.25  # seconds
    TIMER_PERIOD_BAT_V2 = 0.01  # seconds

    timer: Optional[bpy.types.Timer] = None
    # For BAT v1:
    packthread: Optional[_PackThread] = None
    # For BAT v2:
    bat_v2_packer: _BATPacker | None = None
    bat_v2_packer_reported_error: bool = False
    bat_v2_packer_missing_files: list[Path]

    log = _log.getChild(bl_idname)

    @classmethod
    def poll(cls, context: bpy.types.Context) -> bool:
        # Only allow submission when there is a job type selected.
        job_type = job_types.active_job_type(context.scene)
        cls.poll_message_set("No job type selected")
        return job_type is not None

    def execute(self, context: bpy.types.Context) -> set[str]:
        """Submit the job files in a blocking way.

        This allows scripted submission, which blocks the main thread until
        the process is done.
        """

        filepath, ok = self._presubmit_check(context)
        if not ok:
            return {"CANCELLED"}

        # Use BAT v2 if available.
        if bat_v2 is not None:
            raise NotImplementedError(
                "Blocking submission is not yet implemented for BATv2"
            )

        is_running = self._submit_files_bat_v1(context, filepath)
        if not is_running:
            return {"CANCELLED"}

        # Keep handling messages from the background thread. That's only necessary if there is a
        # background thread.
        if self.packthread:
            while True:
                # Block for 5 seconds at a time. The exact duration doesn't matter,
                # as this while-loop is blocking the main thread anyway.
                msg = self.packthread.poll(timeout=5)
                if not msg:
                    # No message received, is fine, just wait for another one.
                    continue

                result = self._on_bat_pack_msg(context, msg)
                if "RUNNING_MODAL" not in result:
                    break
            self.packthread.join(timeout=5)

        self._quit(context)
        return {"FINISHED"}

    def invoke(self, context: bpy.types.Context, event: bpy.types.Event) -> set[str]:
        filepath, ok = self._presubmit_check(context)
        if not ok:
            return {"CANCELLED"}

        if bat_v2 is None:
            is_running = self._submit_files_bat_v1(context, filepath)
        else:
            is_running = self._submit_files_bat_v2(context, filepath)
        if not is_running:
            print("CANCELLING at invoke")
            return {"CANCELLED"}

        if bat_v2:
            # Only BATv2 supports aborting the packing.
            self.report({"INFO"}, "Submitting files, press ESC to abort")

        context.window_manager.modal_handler_add(self)
        return {"RUNNING_MODAL"}

    def modal(self, context: bpy.types.Context, event: bpy.types.Event) -> set[str]:
        wm = bpy.context.window_manager

        # Only BAT v2 has support for aborting the packing operation.
        should_abort = event.type == "ESC" or wm.flamenco_bat_status == "ABORTING"
        if self.bat_v2_packer is not None and should_abort:
            self.report({"WARNING"}, "Flamenco job submission aborted")
            wm.flamenco_bat_status = "ABORTED"
            wm.flamenco_bat_status_txt = ""
            return self._quit(context)

        # This function is called for TIMER events to poll the BAT pack thread.
        if event.type != "TIMER":
            return {"PASS_THROUGH"}

        if self.bat_v2_packer is not None:
            # BAT v2 pack is underway.
            keep_going = self.bat_v2_packer.step()
            if keep_going:
                return {"RUNNING_MODAL"}

            if self.bat_v2_packer_reported_error:
                # The errors themselves should have been reported already.
                wm.flamenco_bat_status = "ABORTED"
                return self._quit(context)

            # BAT v2 pack is done.
            self.blendfile_on_farm = self.bat_v2_packer.blendfile_location_in_pack()
            self._submit_job(context)
            return self._quit(context)

        if self.packthread is None:
            # If there is no pack thread running, there isn't much we can do.
            return self._quit(context)

        # Limit the time for which messages are processed. If there are no
        # queued messages, this code stops immediately, but otherwise it will
        # continue to process until the deadline.
        deadline = time.monotonic() + 0.9 * self.TIMER_PERIOD
        num_messages = 0
        msg = None
        while time.monotonic() < deadline:
            msg = self.packthread.poll()
            if not msg:
                break
            num_messages += 1
            result = self._on_bat_pack_msg(context, msg)
            if "RUNNING_MODAL" not in result:
                return result

        return {"RUNNING_MODAL"}

    def _check_manager(self, context: bpy.types.Context) -> str:
        """Check the Manager version & fetch the job storage directory.

        :return: an error string when something went wrong.
        """
        from . import comms

        # Get the manager's info. This is cached to disk, so regardless of
        # whether this function actually responds to version mismatches, it has
        # to be called to also refresh the shared storage location.
        api_client = self.get_api_client(context)

        report, report_level = comms.ping_manager(
            context.window_manager,
            context.scene,
            api_client,
        )
        if report_level != "INFO":
            return report

        # Check the Manager's version.
        if not self.ignore_version_mismatch:
            mgrinfo = manager_info.load_cached()

            # Safe to assume, as otherwise the ping_manager() call would not have succeeded.
            assert mgrinfo is not None

            my_version = comms.flamenco_client_version()
            mgrversion = mgrinfo.flamenco_version.shortversion

            if mgrversion != my_version:
                context.window_manager.flamenco_version_mismatch = True
                return (
                    f"Manager ({mgrversion}) and this add-on ({my_version}) version "
                    + "mismatch, either update the add-on or force the submission"
                )

        # Un-set the 'flamenco_version_mismatch' when the versions match or when
        # one forced submission is done. Each submission has to go through the
        # same cycle of submitting, seeing the warning, then explicitly ignoring
        # the mismatch, to make it a conscious decision to keep going with
        # potentially incompatible versions.
        context.window_manager.flamenco_version_mismatch = False

        # Empty error message indicates 'ok'.
        return ""

    def _manager_info(
        self, context: bpy.types.Context
    ) -> Optional[manager_info.ManagerInfo]:
        """Load the manager info.

        If it cannot be loaded, returns None after emitting an error message and
        calling self._quit(context).
        """
        manager = manager_info.load_cached()
        if not manager:
            self.report(
                {"ERROR"}, "No information known about Flamenco Manager, refresh first."
            )
            self._quit(context)
            return None
        return manager

    def _presubmit_check(self, context: bpy.types.Context) -> tuple[Path, bool]:
        """Do a pre-submission check, returning whether submission can continue.

        Reports warnings when returning False, so the caller can just abort.

        Returns a tuple (can_submit, filepath_to_submit)
        """

        # Before doing anything, make sure the info we cached about the Manager
        # is up to date. A change in job storage directory on the Manager can
        # cause nasty error messages when we submit, and it's better to just be
        # ahead of the curve and refresh first. This also allows for checking
        # the actual Manager version before submitting.
        err = self._check_manager(context)
        if err:
            self.report({"WARNING"}, err)
            return Path(), False

        if not context.blend_data.filepath:
            # The file path needs to be known before the file can be submitted.
            self.report(
                {"ERROR"}, "Please save your .blend file before submitting to Flamenco"
            )
            return Path(), False

        filepath = self._save_blendfile(context)

        # Check the job with the Manager, to see if it would be accepted.
        if not self._check_job(context):
            return Path(), False

        return filepath, True

    def _save_blendfile(self, context):
        """Save to a different file, specifically for Flamenco.

        We shouldn't overwrite the artist's file.
        We can compress, since this file won't be managed by SVN and doesn't need diffability.
        """
        render = context.scene.render
        prefs = context.preferences

        # Remember settings we need to restore after saving.
        old_use_file_extension = render.use_file_extension
        old_use_overwrite = render.use_overwrite
        old_use_placeholder = render.use_placeholder
        old_use_all_linked_data_direct = getattr(
            prefs.experimental, "use_all_linked_data_direct", None
        )

        # TODO: see about disabling the denoiser (like the old Blender Cloud addon did).

        try:
            # The file extension should be determined by the render settings, not necessarily
            # by the settings in the output panel.
            render.use_file_extension = True

            # Rescheduling should not overwrite existing frames.
            render.use_overwrite = False
            render.use_placeholder = False

            # To work around a shortcoming of BAT, ensure that all
            # indirectly-linked data is still saved as directly-linked.
            #
            # See `133dde41bb5b: Improve handling of (in)directly linked status
            # for linked IDs` in Blender's Git repository.
            if old_use_all_linked_data_direct is not None:
                self.log.info(
                    "Overriding prefs.experimental.use_all_linked_data_direct = True"
                )
                prefs.experimental.use_all_linked_data_direct = True

            filepath = Path(context.blend_data.filepath)
            if job_submission.is_file_inside_job_storage(context, filepath):
                self.log.info(
                    "Saving blendfile, already in shared storage: %s", filepath
                )
                bpy.ops.wm.save_as_mainfile()
            else:
                filepath = filepath.with_suffix(".flamenco.blend")
                self.log.info("Saving copy to temporary file %s", filepath)
                bpy.ops.wm.save_as_mainfile(
                    filepath=str(filepath), compress=True, copy=True
                )
            self.temp_blendfile = filepath
        finally:
            # Restore the settings we changed, even after an exception.
            render.use_file_extension = old_use_file_extension
            render.use_overwrite = old_use_overwrite
            render.use_placeholder = old_use_placeholder

            # Only restore if the property exists to begin with:
            if old_use_all_linked_data_direct is not None:
                prefs.experimental.use_all_linked_data_direct = (
                    old_use_all_linked_data_direct
                )

        return filepath

    def _submit_files_bat_v2(self, context: bpy.types.Context, blendfile: Path) -> bool:
        """Ensure that the files are somewhere in the shared storage.

        Returns True if a packing thread has been started, and False otherwise.
        """

        from .bat_v2 import pack_fs, pack_shaman

        # Reset state from any previous run.
        self.bat_v2_packer = None
        self.bat_v2_packer_reported_error = False
        self.bat_v2_packer_missing_files = []

        manager = self._manager_info(context)
        if not manager:
            return False

        # Get the project root and double-check its existence.
        prefs = preferences.get(context)
        project_path: Path = prefs.project_root()
        assert project_path.is_absolute(), (
            "Expecting project path {!s} to be an absolute path".format(project_path)
        )
        if not project_path.exists():
            self.report(
                {"ERROR"}, "Project path {!s} does not exist".format(project_path)
            )
            raise FileNotFoundError()

        if job_submission.is_file_inside_job_storage(context, blendfile):
            self.log.info(
                "File is already in job storage location, submitting it as-is"
            )
            self._use_blendfile_directly(context, blendfile)
            return True

        if manager.shared_storage.shaman_enabled:
            # Pack to the Shaman server.
            self.log.info("Copying BAT pack to Shaman storage")
            batpacker = pack_shaman.pack_start(
                project_root=project_path,
                reporter=self,
                use_relative_only=True,  # TODO: get from GUI.
                api_client=self.get_api_client(context),
                checkout_path=PurePosixPath(self.job_name),
            )
            batpacker.start()

            # When packing via Shaman, the Shaman server determines the final
            # location of the blend file, and so it's not known yet.
            self.blendfile_on_farm = None
        else:
            # Pack to the filesystem.
            unique_dir = "%s-%s" % (
                datetime.datetime.now().isoformat("-").replace(":", ""),
                self.job_name,
            )
            pack_target_dir = Path(manager.shared_storage.location) / unique_dir
            self.log.info("Copying BAT pack to shared storage: %s", pack_target_dir)
            batpacker = pack_fs.pack_start(
                project_root=project_path,
                reporter=self,
                use_relative_only=True,  # TODO: get from GUI.
                pack_target_dir=pack_target_dir,
            )
            batpacker.start()

            # When packing to the filesystem, the final path of the file on the
            # farm is known immediately.
            source_file_info = batpacker.source_file_info()
            abspath_on_farm = pack_target_dir / source_file_info.relpath_in_pack
            self.blendfile_on_farm = PurePosixPath(abspath_on_farm.as_posix())
            self.log.info("    %s", abspath_on_farm)

        self.bat_v2_packer = batpacker

        # Start the timer for periodic updates of the packing process. This
        # needs a relatively fast update cycle, as each file to be copied needs
        # its own update.
        #
        # TODO: if blocking the UI for each file copy gets too annoying, move
        # the process to a separate thread.
        wm = context.window_manager
        wm.flamenco_bat_status = "INVESTIGATING"
        wm.flamenco_can_abort = True  # Only BAT v2 can abort.
        self.timer = wm.event_timer_add(self.TIMER_PERIOD_BAT_V2, window=context.window)
        return True

    # Reporter Protocol for our BAT v2 interface.
    # See `BATPackReporter` in BAT's `blender_asset_tracer/pack.py`.
    def on_error_on_error(self, errormsg: str, ex: Exception) -> None:
        import traceback

        self.bat_v2_packer_reported_error = True

        # This callback is only called on serious errors that likely indicate
        # bugs, namely when either `on_copy_error()` or `on_rewrite_error()`
        # caused an exception themselves.
        print(60 * "-")
        print("Flamenco ran into an error while sending files to the farm:")
        print()
        print(errormsg)
        print()
        traceback.print_exception(ex)
        bug_report_url = "https://flamenco.blender.org/get-involved"
        print("Please copy-paste the above into a bug report at", bug_report_url)
        print()
        print(60 * "-")
        self.report({"ERROR"}, "Error sending files, check the terminal")

    def on_copy_start(self, src: Path, dest: PurePath) -> None:
        bpy.context.window_manager.flamenco_bat_status = "TRANSFERRING"
        self.log.info("Uploading %s", dest)
        bpy.context.window_manager.flamenco_bat_status_txt = "Uploading {!s}".format(
            dest.name
        )

    def on_copy_done(self, src: Path, dest: PurePath) -> None:
        assert self.bat_v2_packer is not None
        num_total, num_done = self.bat_v2_packer.num_files_to_transfer()
        if num_total < 0:
            progress = 0
        else:
            progress = int(100 * num_done / num_total)

        bpy.context.window_manager.flamenco_bat_progress = progress

    def on_copy_error(self, src: Path, dest: PurePath, errormsg: str) -> None:
        self.bat_v2_packer_reported_error = True
        self.report({"ERROR"}, "Copying {!s} to {!s}: {!s}".format(src, dest, errormsg))

    def on_rewrite_error(
        self, blendfile: Path, relpath_in_pack: PurePath, errormsg: str
    ) -> None:
        self.bat_v2_packer_reported_error = True
        self.report({"ERROR"}, "Rewriting {!s}: {!s}".format(blendfile, errormsg))

    def on_rewrite_start(self, blendfile: Path, relpath_in_pack: PurePath) -> None:
        self.report({"INFO"}, "Rewriting {!s}".format(blendfile.name))
        wm = bpy.context.window_manager
        wm.flamenco_bat_status = "REWRITING"
        wm.flamenco_bat_status_txt = blendfile.name

    def on_rewrite_done(self, blendfile: Path, relpath_in_pack: PurePath) -> None:
        pass

    def on_missing_file(self, blendfile: Path, relpath_in_pack: PurePath) -> None:
        self.bat_v2_packer_missing_files.append(blendfile)
        self.report({"WARNING"}, "Missing file: {!s}".format(blendfile))

    # End of Reporter Protocol.

    def _submit_files_bat_v1(self, context: bpy.types.Context, blendfile: Path) -> bool:
        """Ensure that the files are somewhere in the shared storage.

        Returns True if a packing thread has been started, and False otherwise.
        """

        from .bat import interface as bat_interface

        if bat_interface.is_packing():
            self.report({"ERROR"}, "Another packing operation is running")
            self._quit(context)
            return False

        manager = self._manager_info(context)
        if not manager:
            return False

        if manager.shared_storage.shaman_enabled:
            # self.blendfile_on_farm will be set when BAT created the checkout,
            # see _on_bat_pack_msg() below.
            self.blendfile_on_farm = None
            self._bat_pack_shaman(context, blendfile)
        elif job_submission.is_file_inside_job_storage(context, blendfile):
            self.log.info(
                "File is already in job storage location, submitting it as-is"
            )
            self._use_blendfile_directly(context, blendfile)
        else:
            self.log.info(
                "File is not already in job storage location, copying it there"
            )
            try:
                self.blendfile_on_farm = self._bat_v1_pack_filesystem(
                    context, blendfile
                )
            except FileNotFoundError:
                self._quit(context)
                return False

        wm = context.window_manager
        wm.flamenco_can_abort = False  # Only BAT v2 can abort.
        self.timer = wm.event_timer_add(self.TIMER_PERIOD, window=context.window)

        return True

    def _bat_v1_pack_filesystem(
        self, context: bpy.types.Context, blendfile: Path
    ) -> PurePosixPath:
        """Use BAT to store the pack on the filesystem.

        :return: the path of the blend file, for use in the job definition.
        """
        from .bat import interface as bat_interface

        # Get project path from addon preferences.
        prefs = preferences.get(context)
        project_path: Path = prefs.project_root()
        project_path = bpathlib.make_absolute(Path(bpy.path.abspath(str(project_path))))

        if not project_path.exists():
            self.report({"ERROR"}, "Project path %s does not exist" % project_path)
            raise FileNotFoundError()

        # Determine where the blend file will be stored.
        manager = self._manager_info(context)
        if not manager:
            raise FileNotFoundError("Manager info not known")
        unique_dir = "%s-%s" % (
            datetime.datetime.now().isoformat("-").replace(":", ""),
            self.job_name,
        )
        pack_target_dir = Path(manager.shared_storage.location) / unique_dir

        # TODO: this should take the blendfile location relative to the project path into account.
        pack_target_file = pack_target_dir / blendfile.name
        self.log.info("Will store blend file at %s", pack_target_file)

        self.packthread = bat_interface.copy(
            base_blendfile=blendfile,
            project=project_path,
            target=str(pack_target_dir),
            exclusion_filter="",  # TODO: get from GUI.
            relative_only=True,  # TODO: get from GUI.
        )

        return PurePosixPath(pack_target_file.as_posix())

    def _shaman_checkout_path(self) -> PurePosixPath:
        """Construct the Shaman checkout path, aka Shaman Checkout ID.

        Note that this may not be the actually used checkout ID, as that will be
        made unique to this job by Flamenco Manager. That will be stored in
        self.actual_shaman_checkout_path after the Shaman checkout is actually
        done.
        """
        assert self.job is not None

        # TODO: get project name from preferences/GUI and insert that here too.
        return PurePosixPath(f"{self.job.name}")

    def _bat_pack_shaman(self, context: bpy.types.Context, blendfile: Path) -> None:
        """Use the Manager's Shaman API to submit the BAT pack.

        :return: the filesystem path of the blend file, for in the render job definition.
        """
        from .bat import (
            interface as bat_interface,
        )
        from .bat import (
            shaman as bat_shaman,
        )

        assert self.job is not None
        self.log.info("Sending BAT pack to Shaman")

        prefs = preferences.get(context)
        project_path: Path = prefs.project_root()

        self.packthread = bat_interface.copy(
            base_blendfile=blendfile,
            project=project_path,
            target="/",  # Target directory irrelevant for Shaman transfers.
            exclusion_filter="",  # TODO: get from GUI.
            relative_only=True,  # TODO: get from GUI.
            packer_class=bat_shaman.Packer,
            packer_kwargs=dict(
                api_client=self.get_api_client(context),
                checkout_path=self._shaman_checkout_path(),
            ),
        )

        # We cannot assume the blendfile location is known until the Shaman
        # checkout has actually been created.

    def _on_bat_pack_msg(self, context: bpy.types.Context, msg: _Message) -> set[str]:
        from .bat import interface as bat_interface

        if isinstance(msg, bat_interface.MsgDone):
            if self.blendfile_on_farm is None:
                # Adjust the blendfile to match the Shaman checkout path. Shaman
                # may have checked out at a different location than we
                # requested.
                #
                # Manager automatically creates a variable "jobs" that will
                # resolve to the job storage directory.
                self.blendfile_on_farm = PurePosixPath("{jobs}") / msg.output_path

            self.actual_shaman_checkout_path = msg.actual_checkout_path
            self._submit_job(context)
            return self._quit(context)

        if isinstance(msg, bat_interface.MsgException):
            self.log.error("Error performing BAT pack: %s", msg.ex)
            self.report({"ERROR"}, "Error performing BAT pack: %s" % msg.ex)

            # This was an exception caught at the top level of the thread, so
            # the packing thread itself has stopped.
            return self._quit(context)

        if isinstance(msg, bat_interface.MsgSetWMAttribute):
            wm = context.window_manager
            setattr(wm, msg.attribute_name, msg.value)

        return {"RUNNING_MODAL"}

    def _use_blendfile_directly(
        self, context: bpy.types.Context, blendfile: Path
    ) -> None:
        # The temporary '.flamenco.blend' file should not be deleted, as it
        # will be used directly by the render job.
        self.temp_blendfile = None

        # The blend file is contained in the job storage path, no need to
        # copy anything.
        self.blendfile_on_farm = bpathlib.make_absolute(blendfile)

        # No Shaman is involved when using the file directly.
        self.actual_shaman_checkout_path = None

        self._submit_job(context)

    def _prepare_job_for_submission(self, context: bpy.types.Context) -> bool:
        """Prepare self.job for sending to Flamenco."""

        self.job = job_submission.job_for_scene(context.scene)
        if self.job is None:
            self.report({"ERROR"}, "Unable to create job")
            return False

        propgroup = getattr(context.scene, "flamenco_job_settings", None)
        assert isinstance(propgroup, JobTypePropertyGroup), "did not expect %s" % (
            type(propgroup)
        )
        propgroup.eval_hidden_settings_of_job(context, self.job)

        job_submission.set_blend_file(
            propgroup.job_type,
            self.job,
            # self.blendfile_on_farm is None when we're just checking the job.
            self.blendfile_on_farm or "dummy-for-job-check.blend",
        )

        if self.actual_shaman_checkout_path:
            job_submission.set_shaman_checkout_id(
                self.job, self.actual_shaman_checkout_path
            )

        return True

    def _submit_job(self, context: bpy.types.Context) -> None:
        """Use the Flamenco API to submit the new Job."""
        assert self.job is not None
        assert self.blendfile_on_farm is not None

        from flamenco.manager import ApiException

        if not self._prepare_job_for_submission(context):
            return

        context.window_manager.flamenco_bat_status = "COMMUNICATING"
        context.window_manager.flamenco_bat_status_txt = ""

        api_client = self.get_api_client(context)
        try:
            submitted_job = job_submission.submit_job(self.job, api_client)
        except MaxRetryError:
            self.report({"ERROR"}, "Unable to reach Flamenco Manager")
            return
        except HTTPError as ex:
            self.report({"ERROR"}, "Error communicating with Flamenco Manager: %s" % ex)
            return
        except ApiException as ex:
            if ex.status == 412:
                self.report(
                    {"ERROR"},
                    "Cached job type is old. Refresh the job types and submit again, please",
                )
                return
            if ex.status == 400:
                error = parse_api_error(api_client, ex)
                self.report({"ERROR"}, error.message)
                return
            self.report({"ERROR"}, f"Could not submit job: {ex.reason}")
            return

        # Show a final report.
        num_missing_files = len(self.bat_v2_packer_missing_files)
        if num_missing_files:
            self.report(
                {"WARNING"},
                "Job {!s} submitted (with {:d} missing files)".format(
                    submitted_job.name, num_missing_files
                ),
            )
            context.window_manager.flamenco_bat_status_txt = (
                "Submitted with {:d} missing files".format(num_missing_files)
            )
        else:
            self.report({"INFO"}, "Job {!s} submitted".format(submitted_job.name))
            context.window_manager.flamenco_bat_status_txt = ""

        context.window_manager.flamenco_bat_status = "DONE"

    def _check_job(self, context: bpy.types.Context) -> bool:
        """Use the Flamenco API to check the Job before submitting files.

        :return: "OK" flag, so True = ok, False = not ok.
        """
        from flamenco.manager import ApiException

        if not self._prepare_job_for_submission(context):
            return False
        assert self.job is not None

        api_client = self.get_api_client(context)
        try:
            job_submission.submit_job_check(self.job, api_client)
        except MaxRetryError:
            self.report({"ERROR"}, "Unable to reach Flamenco Manager")
            return False
        except HTTPError as ex:
            self.report({"ERROR"}, "Error communicating with Flamenco Manager: %s" % ex)
            return False
        except ApiException as ex:
            if ex.status == 412:
                self.report(
                    {"ERROR"},
                    "Cached job type is old. Refresh the job types and submit again, please",
                )
                return False
            if ex.status == 400:
                error = parse_api_error(api_client, ex)
                self.report({"ERROR"}, error.message)
                return False
            self.report({"ERROR"}, f"Could not check job: {ex.reason}")
            return False
        return True

    def _quit(self, context: bpy.types.Context) -> set[str]:
        """Stop any timer and return a 'FINISHED' status.

        Does neither check nor abort the BAT pack thread.
        """

        if self.bat_v2_packer is not None and not self.bat_v2_packer.is_done:
            self.log.info("Aborting BAT packer")
            self.bat_v2_packer.abort()
            self.bat_v2_packer = None

        if self.temp_blendfile is not None:
            self.log.info("Removing temporary file %s", self.temp_blendfile)
            self.temp_blendfile.unlink(missing_ok=True)

        if self.timer is not None:
            context.window_manager.event_timer_remove(self.timer)
            self.timer = None
        return {"FINISHED"}


class FLAMENCO_OT_abort(bpy.types.Operator):
    bl_idname = "flamenco.abort"
    bl_label = "Abort"
    bl_description = (
        "Abort a running job submission.\nBlender make take a while to respond to this"
    )

    ABORTABLE_STATES = {
        "INVESTIGATING",
        "REWRITING",
        "TRANSFERRING",
        "COMMUNICATING",
    }

    @classmethod
    def poll(cls, context: bpy.types.Context) -> bool:
        wm = context.window_manager
        return wm.flamenco_can_abort and wm.flamenco_bat_status in cls.ABORTABLE_STATES

    def execute(self, context: bpy.types.Context) -> set[str]:
        wm = context.window_manager
        wm.flamenco_bat_status = "ABORTING"
        return {"FINISHED"}


class FLAMENCO3_OT_explore_file_path(bpy.types.Operator):
    """Opens the given path in a file explorer.

    If the path cannot be found, this operator tries to open its parent.
    """

    bl_idname = "flamenco3.explore_file_path"
    bl_label = "Open in file explorer"
    bl_description = __doc__.rstrip(".")

    path: bpy.props.StringProperty(  # type: ignore
        name="Path", description="Path to explore", subtype="DIR_PATH"
    )

    def execute(self, context):
        import pathlib
        import platform

        # Possibly open a parent of the path
        to_open = pathlib.Path(self.path)
        while to_open.parent != to_open:  # while we're not at the root
            if to_open.exists():
                break
            to_open = to_open.parent
        else:
            self.report(
                {"ERROR"}, "Unable to open %s or any of its parents." % self.path
            )
            return {"CANCELLED"}

        if platform.system() == "Windows":
            import os

            # Ignore the mypy error here, as os.startfile() only exists on Windows.
            os.startfile(str(to_open))  # type: ignore

        elif platform.system() == "Darwin":
            import subprocess

            subprocess.Popen(["open", str(to_open)])

        else:
            import subprocess

            subprocess.Popen(["xdg-open", str(to_open)])

        return {"FINISHED"}


classes = (
    FLAMENCO_OT_ping_manager,
    FLAMENCO_OT_eval_setting,
    FLAMENCO_OT_submit_job,
    FLAMENCO_OT_abort,
    FLAMENCO3_OT_explore_file_path,
)
register, unregister = bpy.utils.register_classes_factory(classes)


def parse_api_error(api_client: _ApiClient, ex: _ApiException) -> _Error:
    """Parse the body of an ApiException into an manager.models.Error instance."""

    from .manager.models import Error

    class MockResponse:
        data: str

    response = MockResponse()
    response.data = ex.body

    error: _Error = api_client.deserialize(response, (Error,), True)
    return error
