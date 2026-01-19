package main

import "fmt"

// Calculator provides basic arithmetic operations
type Calculator struct {
	name string
}

// NewCalculator creates a new Calculator instance
func NewCalculator(name string) *Calculator {
	return &Calculator{name: name}
}

// Add returns the sum of two integers
func (c *Calculator) Add(a, b int) int {
	return a + b
}

// Multiply returns the product of two integers
func (c *Calculator) Multiply(a, b int) int {
	return a * b
}

// Greet returns a greeting message
func (c *Calculator) Greet() string {
	return fmt.Sprintf("Hello from %s!", c.name)
}

func main() {
	calc := NewCalculator("GoCalc")
	fmt.Println(calc.Greet())
	sum := calc.Add(5, 3)
	product := calc.Multiply(5, 3)
	fmt.Printf("Sum: %d, Product: %d\n", sum, product)
}
