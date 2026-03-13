from .. import wheels

# Load all the submodules we need from BAT in one go.
_bat_modules = wheels.load_wheel(
    "blender_asset_tracer",
    ("blendfile", "pack", "pack.progress", "pack.transfer", "pack.shaman", "bpathlib"),
    filename_prefix="blender_asset_tracer-1.",
)
bat_toplevel, blendfile, pack, progress, transfer, shaman, bpathlib = _bat_modules
