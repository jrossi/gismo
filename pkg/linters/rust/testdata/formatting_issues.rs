// A Rust file with formatting issues that rustfmt would fix

fn main()  {
    let x=5;let y=10;
    println!("x: {}, y: {}",x,y);
    
    if x>3{
    println!("x is greater than 3");
    }else{
    println!("x is not greater than 3");
    }
    
    let  vec  =  vec![  1,2,3,4,5  ];
    for   i   in   vec.iter()  {
        println!("{}",i);
    }
}

struct  Person{
name:String,
age:u32,
}

impl   Person  {
    fn   new(name:&str,age:u32)->Self{
        Person{name:name.to_string(),age}
    }
}

fn   add  (  a  :  i32  ,  b  :  i32  )  ->  i32  {
    a+b
}

mod   tests  {
    use   super::*;
    
    #[test]
    fn   test_add()  {
        assert_eq!(add(2,2),4);
    }
}