from typing import TYPE_CHECKING

# SORRY FOR THE COMPLEXITY!
#
# Flamenco needs to be able to deal with BAT versions 1 and 2 at the same time,
# by loading them from different wheel files, depending on which version of
# Blender is running.
#
# At the same time, there's developers who will be really happy when mypy can do
# its thing, and when Blender can load mypy and BAT from virtual environments.

if TYPE_CHECKING:
    # When type-checking, BAT should be importable.
    from blender_asset_tracer import (
        file_usage,
        pack,
        path_rewriting,
        path_rewriting_process,
    )
else:
    # For development only: if we can import BAT directly, just assume it's the
    # right version and go with it.
    import os

    if "VIRTUAL_ENV" in os.environ:
        import site
        from pathlib import Path

        venv_path = Path(os.environ["VIRTUAL_ENV"])
        print(f"Reactivating virtualenv: {venv_path}")

        # Add the virtual environments libraries.
        lib_dirs_posix = list(venv_path.rglob("lib/*/site-packages"))
        lib_dirs_windows = list(venv_path.rglob("Lib/site-packages"))
        for lib_dir in lib_dirs_posix + lib_dirs_windows:
            site.addsitedir(str(lib_dir))

    try:
        from blender_asset_tracer import (
            file_usage,
            pack,
            path_rewriting,
            path_rewriting_process,
        )
    except ImportError:
        # At runtime, some trickery is necessary to load BAT from the bundled wheel file, without making
        # it available in `sys.modules` (to prevent interaction with other add-ons).
        from .. import wheels

        # Load all the submodules we need from BAT in one go.
        _bat_modules = wheels.load_wheel(
            "blender_asset_tracer",
            ("file_usage", "pack", "path_rewriting", "path_rewriting_process"),
            filename_prefix="blender_asset_tracer-2.",
        )
        bat_toplevel, file_usage, pack, path_rewriting, path_rewriting_process = (
            _bat_modules
        )
