// A valid Rust file with proper formatting and no issues

fn main() {
    println!("Hello, world!");
    
    let numbers = vec![1, 2, 3, 4, 5];
    let sum: i32 = numbers.iter().sum();
    
    println!("Sum: {}", sum);
    
    if sum > 10 {
        println!("Sum is greater than 10");
    } else {
        println!("Sum is less than or equal to 10");
    }
}

#[derive(Debug)]
struct Person {
    name: String,
    age: u32,
}

impl Person {
    fn new(name: &str, age: u32) -> Self {
        Person {
            name: name.to_string(),
            age,
        }
    }
    
    fn greet(&self) {
        println!("Hello, my name is {} and I'm {} years old", self.name, self.age);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_person_creation() {
        let person = Person::new("Alice", 30);
        assert_eq!(person.name, "Alice");
        assert_eq!(person.age, 30);
    }
}