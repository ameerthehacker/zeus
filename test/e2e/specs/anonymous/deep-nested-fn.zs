function main(): i32 {
  function outer(): i32 {
    function middle(): i32 {
      function inner(): i32 {
        return 7;
      }
      return inner() + 1;
    }
    return middle() + 2;
  }
  return outer();
}
