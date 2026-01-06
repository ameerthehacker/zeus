class FloatCalculator {
  public result: f64;

  constructor() {
    this.result = 0.0;
  }

  public add(value: f64): f64 {
    this.result = this.result + value;
    return this.result;
  }

  public multiply(value: f64): f64 {
    this.result = this.result * value;
    return this.result;
  }

  public getResult(): f64 {
    return this.result;
  }
}

function main(): i32 {
  let calc: FloatCalculator = new FloatCalculator();
  
  calc.add(10.5);
  calc.multiply(2.0);  // 21.0
  
  return 0;
}

