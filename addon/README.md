# Flamenco Blender add-on

## Setting up development environment

First [install UV](https://docs.astral.sh/uv/#installation), then run:

```sh
$ uv run mypy
```

## Generating the OpenAPI client

1. Make sure Java is installed (so `java --version` shows something sensible).
2. In the root directory of the repository, run `make generate-py`


## Type annotations and lazy imports

This add-on tries to only load Python packages from wheel files when necessary. Loading things from wheels is tricky, as they basically pollute the `sys.modules` dictionary and thus can "leak" to other add-ons. This can cause conflicts when, for example, another add-on is using a different version of the same package.

The result is that sometimes there are some strange hoops to jump through. The most obvious one is for type annotations. This is why you'll see code like:

```
if TYPE_CHECKING:
    from .bat_interface import _PackThread
else:
    _PackThread = object
```

This makes it possible to declare a function with `def func() -> _PackThread`, without having to load `bat_interface` immediately at import time.
