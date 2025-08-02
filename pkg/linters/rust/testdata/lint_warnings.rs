// A Rust file with various linting warnings

use std::io::*; // Wildcard import warning

fn main() {
    let unused_variable = 42; // Unused variable warning
    
    let mut x = 5; // Mutable variable that's never mutated
    println!("x is {}", x);
    
    // Redundant clone
    let s = String::from("hello");
    let s2 = s.clone().clone();
    
    // Unnecessary return
    let result = calculate(10, 20);
    println!("Result: {}", result);
}

// Function with unnecessary return statement
fn calculate(a: i32, b: i32) -> i32 {
    return a + b; // Should just be `a + b`
}

// Dead code warning
fn unused_function() {
    println!("This function is never called");
}

// Naming convention warning (should be SCREAMING_SNAKE_CASE)
const myConstant: i32 = 100;

// Empty implementation
impl std::fmt::Display for MyStruct {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        Ok(())
    }
}

struct MyStruct {
    field: String,
}

// Comparison to empty string
fn check_empty(s: &str) -> bool {
    s == "" // Should use s.is_empty()
}

// Needless borrow
fn print_string(s: &String) { // Should be &str
    println!("{}", &s); // Needless borrow
}