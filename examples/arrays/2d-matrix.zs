// 2D Matrix Operations
// Demonstrates: 2D arrays, nested loops, matrix operations

class Matrix {
  public data: i32[][];
  public rows: i32;
  public cols: i32;

  constructor(r: i32, c: i32) {
    this.rows = r;
    this.cols = c;
    this.data = new i32[][];
    
    // Initialize rows
    let i: i32 = 0;
    while (i < r) {
      this.data.push(new i32[c]);
      i = i + 1;
    }
  }

  public set(row: i32, col: i32, value: i32): void {
    this.data[row][col] = value;
  }

  public get(row: i32, col: i32): i32 {
    return this.data[row][col];
  }

  // Sum all elements in the matrix
  public sum(): i32 {
    let total: i32 = 0;
    let i: i32 = 0;
    
    while (i < this.rows) {
      let j: i32 = 0;
      while (j < this.cols) {
        total = total + this.data[i][j];
        j = j + 1;
      }
      i = i + 1;
    }
    
    return total;
  }
}

function main(): i32 {
  // Create a 3x3 matrix
  let m: Matrix = new Matrix(3, 3);
  
  // Set identity matrix
  m.set(0, 0, 1);
  m.set(1, 1, 1);
  m.set(2, 2, 1);
  
  // Add some values
  m.set(0, 1, 5);
  m.set(1, 2, 10);
  
  // Sum should be 1 + 1 + 1 + 5 + 10 = 18
  return m.sum();
}

