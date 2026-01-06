class Calculator {
  public result: i32;

  constructor() {
    this.result = 0;
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

  public getResult(): i32 {
    return this.result;
  }
}

function main(): i32 {
  let calc: Calculator = new Calculator();
  
  calc.add(100);      // result = 100
  calc.subtract(20);  // result = 80
  calc.multiply(2);   // result = 160
  calc.divide(4);     // result = 40
  
  return calc.getResult();  // 40
}

