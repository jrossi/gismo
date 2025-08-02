"""A Python file with various linting issues but valid syntax."""

import os
import sys  # Unused import
from typing import Any  # Using Any type


def badly_named_function(x, y, z):  # Missing spaces after commas, single letter params
    """Missing type hints and poor naming."""
    unused_var = 42  # Unused variable
    if x == True:  # Should use 'is True' or just 'if x'
        print("bad spacing")  # Extra spaces
    return x + y + z  # Missing spaces around operator


class bad_class_name:  # Class name should be CamelCase
    def __init__(self):
        self.x = 1
        self.y = 2
        self.z = 3  # Could use a data structure

    def Method_With_Bad_Name(self):  # Method should be snake_case
        list = [1, 2, 3]  # Shadowing built-in
        for i in range(len(list)):  # Should iterate directly
            print(list[i])


# Line too long - this is a really long comment that exceeds the typical line length limit of 88 characters that ruff enforces by default
def function_with_too_many_arguments(
    arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8, arg9, arg10
):
    """Too many arguments."""
    pass


# Missing blank line before function
def no_blank_line():
    d = {"a": 1, "b": 2}
    # Should use .get() to avoid KeyError
    if "c" in d.keys():  # .keys() is redundant
        print(d["c"])


def compare_none(value: Any) -> bool:
    """Using Any type and wrong None comparison."""
    if value == None:  # Should use 'is None'
        return True
    return False


# Global variable (not constant)
global_var = "this should be GLOBAL_VAR"


def mutable_default_arg(items=[]):  # Dangerous mutable default
    """Mutable default argument."""
    items.append(1)
    return items
