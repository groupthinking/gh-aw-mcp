/**
 * Calculator provides basic arithmetic operations
 */
public class Calculator {
    private String name;

    /**
     * Creates a new Calculator with the given name
     * @param name The calculator's name
     */
    public Calculator(String name) {
        this.name = name;
    }

    /**
     * Adds two integers
     * @param a First number
     * @param b Second number
     * @return The sum of a and b
     */
    public int add(int a, int b) {
        return a + b;
    }

    /**
     * Multiplies two integers
     * @param a First number
     * @param b Second number
     * @return The product of a and b
     */
    public int multiply(int a, int b) {
        return a * b;
    }

    /**
     * Returns a greeting message
     * @return Greeting string
     */
    public String greet() {
        return "Hello from " + this.name + "!";
    }

    public static void main(String[] args) {
        Calculator calc = new Calculator("JavaCalc");
        System.out.println(calc.greet());
        int sum = calc.add(5, 3);
        int product = calc.multiply(5, 3);
        System.out.printf("Sum: %d, Product: %d%n", sum, product);
    }
}
