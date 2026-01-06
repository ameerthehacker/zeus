function generateFibonacci(count: u32): i32[] {
  let result: i32[] = new i32[];
  
  if (count <= 0) {
    return result;
  }
  
  result.push(0);
  if (count == 1) {
    return result;
  }
  
  result.push(1);
  
  let i: u32 = 2;
  while (i < count) {
    let next: i32 = result.get(i - 1) + result.get(i - 2);
    result.push(next);
    i = i + 1;
  }
  
  return result;
}

function main(): i32 {
  let fibs: i32[] = generateFibonacci(10);
  // fibs = [0, 1, 1, 2, 3, 5, 8, 13, 21, 34]
  
  return fibs.get(9);  // Returns 34
}

