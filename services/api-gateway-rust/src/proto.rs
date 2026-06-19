// Сгенерированный tonic/prost код подключаем одним include.
// allow(dead_code), потому что из buf.validate.proto генерируется много неиспользуемых типов.
#[allow(dead_code)]
pub mod generated {
    include!(concat!(env!("OUT_DIR"), "/proto.rs"));
}

pub use generated::*;
