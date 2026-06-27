#![allow(non_snake_case)]

use oct_octxiliary::{Dispatcher, Field, MainLoop, Response};
use std::process;

const RUST_VALUE: i64 = 35;

fn main() {
    let Dispatcher = match Dispatcher::New("ChimeraOctx").Handle("ChimeraHello", |Request| {
        let GoValue = Request.FieldInt("GoValue")?;
        Ok(Response::OkRecord(
            "ChimeraResponse",
            vec![
                Field::Int("GoValue", GoValue),
                Field::Int("RustValue", RUST_VALUE),
                Field::Int("Total", GoValue + RUST_VALUE),
            ],
        ))
    }) {
        Ok(Dispatcher) => Dispatcher,
        Err(err) => {
            eprintln!("chimera-octx-sidecar: {err}");
            process::exit(1);
        }
    };

    if let Err(err) = MainLoop(Dispatcher) {
        eprintln!("chimera-octx-sidecar: {err}");
        process::exit(1);
    }
}
