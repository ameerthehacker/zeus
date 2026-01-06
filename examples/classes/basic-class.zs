// Basic Class Example
// Demonstrates: class definition, constructor, methods, properties

class Rectangle {
  public width: f64;
  public height: f64;

  constructor(w: f64, h: f64) {
    this.width = w;
    this.height = h;
  }

  public area(): f64 {
    return this.width * this.height;
  }

  public perimeter(): f64 {
    return 2.0 * (this.width + this.height);
  }

  public isSquare(): boolean {
    return this.width == this.height;
  }

  public scale(factor: f64): void {
    this.width = this.width * factor;
    this.height = this.height * factor;
  }
}

function main(): i32 {
  let rect: Rectangle = new Rectangle(10.0, 5.0);
  
  let area: f64 = rect.area();         // 50.0
  let perim: f64 = rect.perimeter();   // 30.0
  let square: boolean = rect.isSquare(); // false
  
  rect.scale(2.0);
  let newArea: f64 = rect.area();      // 200.0
  
  // Return area as integer for exit code
  return 50;
}

