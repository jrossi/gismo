"""Test file for testing Python test execution."""

import pytest


def add(a: int, b: int) -> int:
    """Add two numbers."""
    return a + b


def divide(a: float, b: float) -> float:
    """Divide two numbers."""
    if b == 0:
        raise ValueError("Cannot divide by zero")
    return a / b


class TestMath:
    """Test class for math functions."""

    def test_add_positive_numbers(self) -> None:
        """Test addition with positive numbers."""
        assert add(2, 3) == 5
        assert add(0, 0) == 0
        assert add(100, 200) == 300

    def test_add_negative_numbers(self) -> None:
        """Test addition with negative numbers."""
        assert add(-1, -1) == -2
        assert add(-5, 5) == 0
        assert add(10, -20) == -10

    def test_divide_normal(self) -> None:
        """Test normal division."""
        assert divide(10, 2) == 5.0
        assert divide(9, 3) == 3.0
        assert divide(1, 1) == 1.0

    def test_divide_by_zero(self) -> None:
        """Test division by zero raises exception."""
        with pytest.raises(ValueError, match="Cannot divide by zero"):
            divide(10, 0)

    def test_divide_float_precision(self) -> None:
        """Test division with float precision."""
        result = divide(1, 3)
        assert abs(result - 0.333333) < 0.00001


def test_standalone_function() -> None:
    """A standalone test function."""
    assert add(1, 1) == 2
    assert add(0, 5) == 5
