"""A valid Python file with proper formatting and no issues."""


def greet(name: str) -> str:
    """Return a greeting message."""
    return f"Hello, {name}!"


def add(a: int, b: int) -> int:
    """Add two numbers together."""
    return a + b


class Calculator:
    """A simple calculator class."""

    def __init__(self) -> None:
        """Initialize the calculator."""
        self.result = 0

    def add(self, value: int) -> None:
        """Add a value to the result."""
        self.result += value

    def get_result(self) -> int:
        """Get the current result."""
        return self.result


def main() -> None:
    """Main function."""
    print(greet("World"))
    print(f"2 + 3 = {add(2, 3)}")

    calc = Calculator()
    calc.add(10)
    calc.add(20)
    print(f"Calculator result: {calc.get_result()}")


if __name__ == "__main__":
    main()
