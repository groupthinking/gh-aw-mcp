"""Calculator module providing basic arithmetic operations."""

from typing import Optional


class Calculator:
    """Calculator class for basic arithmetic operations."""

    def __init__(self, name: str) -> None:
        """Initialize Calculator with a name.
        
        Args:
            name: The calculator's name
        """
        self.name = name

    def add(self, a: int, b: int) -> int:
        """Add two integers.
        
        Args:
            a: First number
            b: Second number
            
        Returns:
            The sum of a and b
        """
        return a + b

    def multiply(self, a: int, b: int) -> int:
        """Multiply two integers.
        
        Args:
            a: First number
            b: Second number
            
        Returns:
            The product of a and b
        """
        return a * b

    def greet(self) -> str:
        """Return a greeting message.
        
        Returns:
            Greeting string
        """
        return f"Hello from {self.name}!"


def main() -> None:
    """Main function to demonstrate calculator usage."""
    calc = Calculator("PyCalc")
    print(calc.greet())
    sum_result = calc.add(5, 3)
    product = calc.multiply(5, 3)
    print(f"Sum: {sum_result}, Product: {product}")


if __name__ == "__main__":
    main()
