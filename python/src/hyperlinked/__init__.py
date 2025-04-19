import builtins
import inspect
import os
import sys
import traceback
from pathlib import Path
from typing import Any, Optional

SCHEME = os.environ.get("HYPERLINKED_SCHEME", "file")


def print(
    *args: Any,
    sep: str = " ",
    end: str = "\n",
    file: Any = None,
    flush: bool = False,
    scheme: str = SCHEME,
) -> None:
    """
    Print to terminal, hyperlinked to the current file and line number.

    The same as the built-in print function, but adds an OSC8 hyperlink to the call site.
    """
    if (frame := inspect.currentframe()) is not None:
        if len(caller_frame_info := inspect.getouterframes(frame, 2)) >= 2:
            caller_frame = caller_frame_info[1]
            builtins.print(
                hyperlink_to_path(
                    text=sep.join(map(str, args)),
                    path=Path(caller_frame.filename).resolve(),
                    line=caller_frame.lineno,
                    scheme=scheme,
                ),
                end=end,
                file=file or sys.stdout,
                flush=flush,
            )
            return
    print(*args, sep=sep, end=end, file=file, flush=flush)


def hyperlinked(text: str) -> str:
    """
    Return the given string with an OSC8 hyperlink added, pointing to the call site.
    """
    frame = inspect.currentframe()
    if frame is None:
        return text
    caller_frame_info = inspect.getouterframes(frame, 2)
    if len(caller_frame_info) < 2:
        return text
    caller_info = caller_frame_info[1]
    return hyperlink_to_path(
        text, Path(caller_info.filename).resolve(), caller_info.lineno
    )


def hyperlink(text: str, url: str) -> str:
    """Formats a string with OSC8 escape codes to create a terminal hyperlink."""
    osc = "\x1b]"
    st = "\x1b\\"
    return f"{osc}8;;{url}{st}{text}{osc}8;;{st}"


def hyperlink_to_path(
    text: str,
    path: Path,
    line: Optional[int] = None,
    scheme: str = SCHEME,
) -> str:
    """Creates an OSC8 hyperlink pointing to a file path, optionally with a line number."""
    url = f"{scheme}://file/{path.resolve()}"
    if line is not None:
        url += f":{line}"
    return hyperlink(text=text, url=url)


def print_stack(f=None, limit=None, file=None, *, scheme: str = SCHEME):
    """
    Print a hyperlinked stack trace from its invocation point.

    Mimics traceback.print_stack but hyperlinks file paths.
    Args:
        f: Print the stack trace from this frame. If None, use the current frame.
        limit: Limit the number of stack frames to print.
        file: Print the stack trace to this file-like object. Defaults to sys.stderr.
        scheme: The URI scheme to use for hyperlinks.
    """
    if file is None:
        file = sys.stderr

    if f is None:
        # traceback.extract_stack includes the current frame, which we usually don't want
        # if called directly like traceback.print_stack(). We skip it.
        # If f is provided, we assume the caller knows which frame they want.
        stack = traceback.extract_stack(limit=limit + 1 if limit is not None else None)[
            :-1
        ]
    else:
        stack = traceback.extract_stack(f, limit=limit)

    for frame in stack:
        formatted_line = _format_hyperlinked_frame(frame, scheme)
        builtins.print(formatted_line, end="", file=file)


def excepthook(exc_type, exc_value, tb, *, scheme: str = SCHEME):
    """
    Exception hook to print hyperlinked tracebacks to sys.stderr.

    Args:
        exc_type: The type of the exception.
        exc_value: The exception instance.
        tb: The traceback object.
        scheme: The URI scheme to use for hyperlinks.
    """
    # Mimic the standard traceback output format
    builtins.print("Traceback (most recent call last):", file=sys.stderr)
    extracted_tb = traceback.extract_tb(tb)
    for frame in extracted_tb:
        formatted_line = _format_hyperlinked_frame(frame, scheme)
        builtins.print(formatted_line, end="", file=sys.stderr)

    # Format and print the exception type and value
    exc_lines = traceback.format_exception_only(exc_type, exc_value)
    for line in exc_lines:
        builtins.print(line, end="", file=sys.stderr)


def _format_hyperlinked_frame(frame: traceback.FrameSummary, scheme: str) -> str:
    """Formats a single stack frame with a hyperlink."""
    location_text = f'File "{frame.filename}", line {frame.lineno}'
    hyperlinked_location = hyperlink_to_path(
        text=location_text,
        path=Path(frame.filename),
        line=frame.lineno,
        scheme=scheme,
    )
    line = f"  {hyperlinked_location}, in {frame.name}\n"
    if frame.line:
        line += f"    {frame.line.strip()}\n"
    return line


if __name__ == "__main__":
    name = "World"
    print("Hello", name, "from hprint!")
    print("This is a normal print statement.")
    x = 10
    print(f"The value of x is: {x}")
