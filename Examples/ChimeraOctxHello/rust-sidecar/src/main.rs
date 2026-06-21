use std::io::{self, Read, Write};
use std::process;

const MAGIC: &[u8; 8] = b"OCTWRAP\0";
const ABI_MAJOR: u16 = 0;
const ABI_MINOR: u16 = 1;
const RUST_VALUE: i64 = 35;

fn main() {
    if let Err(err) = run() {
        eprintln!("chimera-octx-sidecar: {err}");
        process::exit(1);
    }
}

fn run() -> Result<(), String> {
    let stdin = io::stdin();
    let stdout = io::stdout();
    let mut input = stdin.lock();
    let mut output = stdout.lock();

    read_handshake(&mut input)?;
    write_handshake(&mut output)?;

    let frame = read_frame(&mut input)?;
    let response = match parse_request(&frame) {
        Ok(req) => success_response(req.id, req.go_value),
        Err(err) => error_response(err.id.unwrap_or(0), &err.message),
    };
    write_frame(&mut output, &response)?;
    output.flush().map_err(|err| err.to_string())?;
    Ok(())
}

fn read_handshake(input: &mut impl Read) -> Result<(), String> {
    let mut magic = [0u8; 8];
    input
        .read_exact(&mut magic)
        .map_err(|err| format!("failed to read handshake magic: {err}"))?;
    if &magic != MAGIC {
        return Err("invalid OCTWRAP handshake magic".to_string());
    }
    let major = read_u16(input).map_err(|err| format!("failed to read ABI major: {err}"))?;
    let _minor = read_u16(input).map_err(|err| format!("failed to read ABI minor: {err}"))?;
    if major != ABI_MAJOR {
        return Err(format!("unsupported OCTWRAP ABI major {major}"));
    }
    Ok(())
}

fn write_handshake(output: &mut impl Write) -> Result<(), String> {
    output.write_all(MAGIC).map_err(|err| err.to_string())?;
    output
        .write_all(&ABI_MAJOR.to_le_bytes())
        .map_err(|err| err.to_string())?;
    output
        .write_all(&ABI_MINOR.to_le_bytes())
        .map_err(|err| err.to_string())?;
    output.flush().map_err(|err| err.to_string())?;
    Ok(())
}

fn read_frame(input: &mut impl Read) -> Result<String, String> {
    let len =
        read_u32(input).map_err(|err| format!("failed to read frame length: {err}"))? as usize;
    let mut body = vec![0u8; len];
    input
        .read_exact(&mut body)
        .map_err(|err| format!("failed to read frame body: {err}"))?;
    String::from_utf8(body).map_err(|err| format!("frame is not UTF-8: {err}"))
}

fn write_frame(output: &mut impl Write, body: &str) -> Result<(), String> {
    let len = u32::try_from(body.len()).map_err(|_| "response frame too large".to_string())?;
    output
        .write_all(&len.to_le_bytes())
        .map_err(|err| err.to_string())?;
    output
        .write_all(body.as_bytes())
        .map_err(|err| err.to_string())?;
    Ok(())
}

fn read_u16(input: &mut impl Read) -> io::Result<u16> {
    let mut bytes = [0u8; 2];
    input.read_exact(&mut bytes)?;
    Ok(u16::from_le_bytes(bytes))
}

fn read_u32(input: &mut impl Read) -> io::Result<u32> {
    let mut bytes = [0u8; 4];
    input.read_exact(&mut bytes)?;
    Ok(u32::from_le_bytes(bytes))
}

struct ChimeraRequest {
    id: i64,
    go_value: i64,
}

struct RequestError {
    id: Option<i64>,
    message: String,
}

fn parse_request(frame: &str) -> Result<ChimeraRequest, RequestError> {
    // M0 intentionally recognizes only the exact Octxiliary typed-record subset
    // emitted by the Go example. This is not a general Octagon parser: it does
    // not parse arbitrary records, arrays, floats, strings in DTO payloads,
    // handles, comments, dimensions, package-qualified constructors, callbacks,
    // or a C interop layer.
    let id = extract_after(frame, "OctxiliaryRequest { id: ", " family: ").ok();
    let fail = |id, message: &str| RequestError {
        id,
        message: message.to_string(),
    };

    let id_value = id.ok_or_else(|| fail(None, "missing request id"))?;
    require_contains(frame, "family: \"ChimeraOctx\"")
        .map_err(|message| fail(Some(id_value), &message))?;
    require_contains(frame, "function: \"ChimeraHello\"")
        .map_err(|message| fail(Some(id_value), &message))?;
    require_contains(frame, "kind: \"Record\" recordType: \"ChimeraRequest\"")
        .map_err(|message| fail(Some(id_value), &message))?;
    require_contains(
        frame,
        "OctxiliaryField { name: \"GoValue\" value: OctxiliaryValue { kind: \"Int\" int: ",
    )
    .map_err(|message| fail(Some(id_value), &message))?;
    let go_value = extract_after(
        frame,
        "name: \"GoValue\" value: OctxiliaryValue { kind: \"Int\" int: ",
        " }",
    )
    .map_err(|_| fail(Some(id_value), "missing GoValue Int field"))?;
    Ok(ChimeraRequest {
        id: id_value,
        go_value,
    })
}

fn require_contains(frame: &str, needle: &str) -> Result<(), String> {
    if frame.contains(needle) {
        Ok(())
    } else {
        Err(format!("request missing {needle}"))
    }
}

fn extract_after(frame: &str, prefix: &str, suffix: &str) -> Result<i64, String> {
    let start = frame
        .find(prefix)
        .ok_or_else(|| format!("missing {prefix}"))?
        + prefix.len();
    let rest = &frame[start..];
    let end = rest
        .find(suffix)
        .ok_or_else(|| format!("missing terminator {suffix}"))?;
    rest[..end]
        .trim()
        .parse::<i64>()
        .map_err(|err| err.to_string())
}

fn success_response(id: i64, go_value: i64) -> String {
    let total = go_value + RUST_VALUE;
    format!(
        "OctxiliaryResponse {{ id: {id} ok: true value: OctxiliaryValue {{ kind: \"Record\" recordType: \"ChimeraResponse\" fields: [ OctxiliaryField {{ name: \"GoValue\" value: OctxiliaryValue {{ kind: \"Int\" int: {go_value} }} }} OctxiliaryField {{ name: \"RustValue\" value: OctxiliaryValue {{ kind: \"Int\" int: {RUST_VALUE} }} }} OctxiliaryField {{ name: \"Total\" value: OctxiliaryValue {{ kind: \"Int\" int: {total} }} }} ] }} }}"
    )
}

fn error_response(id: i64, error: &str) -> String {
    let escaped = error.replace('\\', "\\\\").replace('"', "\\\"");
    format!("OctxiliaryResponse {{ id: {id} ok: false error: \"{escaped}\" }}")
}
