// A method call on an integer literal autoboxes it into Number. Parentheses are required so the
// lexer does not read `5.` as a float.
console.log((5).toString());
