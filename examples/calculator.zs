// Simple Calculator with Class
// Run with: make run file=calculator
// Demonstrates: classes, methods, state management

class Calculator {
  public result: i32;

  constructor() {
    this.result = 0;
  }

  public set(value: i32): void {
    this.result = value;
  }

  public add(value: i32): i32 {
    this.result = this.result + value;
    return this.result;
  }

  public subtract(value: i32): i32 {
    this.result = this.result - value;
    return this.result;
  }

  public multiply(value: i32): i32 {
    this.result = this.result * value;
    return this.result;
  }

  public divide(value: i32): i32 {
    if (value != 0) {
      this.result = this.result / value;
    }
    return this.result;
  }

  public get(): i32 {
    return this.result;
  }
}

function main(): i32 {
  let calc: Calculator = new Calculator();
  
  // Calculate: ((10 + 5) * 3 - 15) / 3 = 10
  calc.set(10);
  calc.add(5);       // 15
  calc.multiply(3);  // 45
  calc.subtract(15); // 30
  calc.divide(3);    // 10
  
  return calc.get();
}

