class Person {
  name: string;

  constructor(name: string) {
    this.name = name;
  }
}

function main(): i32 {
  let person: Person;
  log(person.name);
  return 0;
}
