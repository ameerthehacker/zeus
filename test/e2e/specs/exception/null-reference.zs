class Person {
  public name: string;

  constructor(name: string) {
    this.name = name;
  }
}

function main(): i32 {
  let person: Person;
  console.log(person.name);
  return 0;
}
