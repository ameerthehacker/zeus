function main(): i8 {
  let message: string = "Hello World! 👋";

  console.log(message);

  let messageEdited: u8[] = message;

  messageEdited[0] = 'h';

  // Cast u8[] back to string for log
  let editedString: string = messageEdited;
  console.log(editedString);

  return 0;
}
