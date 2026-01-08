function main(): i8 {
  let message: string = "Hello World! 👋";

  log(message);

  let messageEdited: u8[] = message;

  messageEdited[0] = 'h';

  // Cast u8[] back to string for log
  let editedString: string = messageEdited;
  log(editedString);

  return 0;
}
