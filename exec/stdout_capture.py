from __future__ import annotations

import contextlib
import io
import sys
from typing import Iterator, Tuple


@contextlib.contextmanager
def capture() -> Iterator[Tuple[io.StringIO, io.StringIO]]:
    old_out, old_err = sys.stdout, sys.stderr
    out, err = io.StringIO(), io.StringIO()
    sys.stdout, sys.stderr = out, err
    try:
        yield out, err
    finally:
        sys.stdout, sys.stderr = old_out, old_err
