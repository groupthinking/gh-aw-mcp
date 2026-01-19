/**
 * Calculator class provides basic arithmetic operations
 */
class Calculator {
    /**
     * Creates a new Calculator
     * @param {string} name - The calculator's name
     */
    constructor(name) {
        this.name = name;
    }

    /**
     * Adds two numbers
     * @param {number} a - First number
     * @param {number} b - Second number
     * @returns {number} The sum of a and b
     */
    add(a, b) {
        return a + b;
    }

    /**
     * Multiplies two numbers
     * @param {number} a - First number
     * @param {number} b - Second number
     * @returns {number} The product of a and b
     */
    multiply(a, b) {
        return a * b;
    }

    /**
     * Returns a greeting message
     * @returns {string} Greeting string
     */
    greet() {
        return `Hello from ${this.name}!`;
    }
}

// Main execution
const calc = new Calculator("JSCalc");
console.log(calc.greet());
const sum = calc.add(5, 3);
const product = calc.multiply(5, 3);
console.log(`Sum: ${sum}, Product: ${product}`);

module.exports = Calculator;
