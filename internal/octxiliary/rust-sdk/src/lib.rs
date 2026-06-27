#![allow(non_snake_case)]

//! Minimal Rust SDK for Octxiliary sidecars.
//!
//! Public Chimera/Octxiliary SDK APIs intentionally use PascalCase to match
//! Oct, Go, and C# API conventions rather than idiomatic Rust snake_case.

use std::collections::HashMap;
use std::fmt;
use std::io::{self, Read, Write};
use std::panic::{catch_unwind, AssertUnwindSafe};

pub const OCTXILIARY_PROTOCOL_NAME: &str = "OCTWRAP";
pub const OCTXILIARY_ABI_MAJOR: u16 = 0;
pub const OCTXILIARY_ABI_MINOR: u16 = 1;
const MAGIC: &[u8; 8] = b"OCTWRAP\0";

#[derive(Debug, Clone, PartialEq)]
pub struct OctxError {
    pub Message: String,
}

impl OctxError {
    pub fn New(message: impl Into<String>) -> Self {
        Self {
            Message: message.into(),
        }
    }
}

impl fmt::Display for OctxError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.Message)
    }
}

impl std::error::Error for OctxError {}
impl From<String> for OctxError {
    fn from(value: String) -> Self {
        Self::New(value)
    }
}
impl From<&str> for OctxError {
    fn from(value: &str) -> Self {
        Self::New(value)
    }
}
impl From<io::Error> for OctxError {
    fn from(value: io::Error) -> Self {
        Self::New(value.to_string())
    }
}

#[derive(Debug, Clone, PartialEq)]
pub enum Value {
    Void,
    Int(i64),
    Float(f64),
    Bool(bool),
    String(String),
    Record {
        RecordType: String,
        Fields: Vec<Field>,
    },
}

impl Value {
    fn KindName(&self) -> &'static str {
        match self {
            Value::Void => "Void",
            Value::Int(_) => "Int",
            Value::Float(_) => "Float",
            Value::Bool(_) => "Bool",
            Value::String(_) => "String",
            Value::Record { .. } => "Record",
        }
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct Field {
    pub Name: String,
    pub Value: Value,
}

impl Field {
    pub fn Int(name: impl Into<String>, value: i64) -> Self {
        Self {
            Name: name.into(),
            Value: Value::Int(value),
        }
    }
    pub fn Float(name: impl Into<String>, value: f64) -> Self {
        Self {
            Name: name.into(),
            Value: Value::Float(value),
        }
    }
    pub fn Bool(name: impl Into<String>, value: bool) -> Self {
        Self {
            Name: name.into(),
            Value: Value::Bool(value),
        }
    }
    pub fn String(name: impl Into<String>, value: impl Into<String>) -> Self {
        Self {
            Name: name.into(),
            Value: Value::String(value.into()),
        }
    }
    pub fn Record(
        name: impl Into<String>,
        record_type: impl Into<String>,
        fields: Vec<Field>,
    ) -> Self {
        Self {
            Name: name.into(),
            Value: Value::Record {
                RecordType: record_type.into(),
                Fields: fields,
            },
        }
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct Request {
    pub Id: i64,
    pub Family: String,
    pub Function: String,
    pub Args: Vec<Value>,
}

impl Request {
    pub fn FieldInt(&self, name: &str) -> Result<i64, OctxError> {
        self.FirstRecord()?.FieldInt(name)
    }
    pub fn FieldFloat(&self, name: &str) -> Result<f64, OctxError> {
        self.FirstRecord()?.FieldFloat(name)
    }
    pub fn FieldBool(&self, name: &str) -> Result<bool, OctxError> {
        self.FirstRecord()?.FieldBool(name)
    }
    pub fn FieldString(&self, name: &str) -> Result<String, OctxError> {
        self.FirstRecord()?.FieldString(name)
    }
    pub fn FieldRecord(&self, name: &str) -> Result<RecordRef<'_>, OctxError> {
        let record = self.FirstRecord()?;
        record.FieldRecord(name)
    }
    pub fn OptionalInt(&self, name: &str) -> Result<Option<i64>, OctxError> {
        self.FirstRecord()?.OptionalInt(name)
    }
    pub fn OptionalFloat(&self, name: &str) -> Result<Option<f64>, OctxError> {
        self.FirstRecord()?.OptionalFloat(name)
    }
    pub fn OptionalBool(&self, name: &str) -> Result<Option<bool>, OctxError> {
        self.FirstRecord()?.OptionalBool(name)
    }
    pub fn OptionalString(&self, name: &str) -> Result<Option<String>, OctxError> {
        self.FirstRecord()?.OptionalString(name)
    }

    fn FirstRecord(&self) -> Result<RecordRef<'_>, OctxError> {
        match self.Args.first() {
            Some(Value::Record { RecordType, Fields }) => Ok(RecordRef { RecordType, Fields }),
            Some(value) => Err(OctxError::New(format!(
                "first argument must be Record, got {}",
                value.KindName()
            ))),
            None => Err(OctxError::New("request missing record argument")),
        }
    }
}

#[derive(Debug, Clone, Copy)]
pub struct RecordRef<'a> {
    pub RecordType: &'a str,
    pub Fields: &'a [Field],
}

impl<'a> RecordRef<'a> {
    pub fn FieldInt(&self, name: &str) -> Result<i64, OctxError> {
        match self.Required(name)? {
            Value::Int(v) => Ok(*v),
            v => Err(self.WrongType(name, "Int", v)),
        }
    }
    pub fn FieldFloat(&self, name: &str) -> Result<f64, OctxError> {
        match self.Required(name)? {
            Value::Float(v) => Ok(*v),
            v => Err(self.WrongType(name, "Float", v)),
        }
    }
    pub fn FieldBool(&self, name: &str) -> Result<bool, OctxError> {
        match self.Required(name)? {
            Value::Bool(v) => Ok(*v),
            v => Err(self.WrongType(name, "Bool", v)),
        }
    }
    pub fn FieldString(&self, name: &str) -> Result<String, OctxError> {
        match self.Required(name)? {
            Value::String(v) => Ok(v.clone()),
            v => Err(self.WrongType(name, "String", v)),
        }
    }
    pub fn FieldRecord(&self, name: &str) -> Result<RecordRef<'a>, OctxError> {
        match self.Required(name)? {
            Value::Record { RecordType, Fields } => Ok(RecordRef { RecordType, Fields }),
            v => Err(self.WrongType(name, "Record", v)),
        }
    }
    pub fn OptionalInt(&self, name: &str) -> Result<Option<i64>, OctxError> {
        self.Optional(name, Self::FieldInt)
    }
    pub fn OptionalFloat(&self, name: &str) -> Result<Option<f64>, OctxError> {
        self.Optional(name, Self::FieldFloat)
    }
    pub fn OptionalBool(&self, name: &str) -> Result<Option<bool>, OctxError> {
        self.Optional(name, Self::FieldBool)
    }
    pub fn OptionalString(&self, name: &str) -> Result<Option<String>, OctxError> {
        self.Optional(name, Self::FieldString)
    }
    fn Optional<T>(
        &self,
        name: &str,
        f: fn(&Self, &str) -> Result<T, OctxError>,
    ) -> Result<Option<T>, OctxError> {
        if self.Find(name).is_none() {
            Ok(None)
        } else {
            f(self, name).map(Some)
        }
    }
    fn Required(&self, name: &str) -> Result<&'a Value, OctxError> {
        self.Find(name)
            .ok_or_else(|| OctxError::New(format!("missing required field {name:?}")))
    }
    fn Find(&self, name: &str) -> Option<&'a Value> {
        self.Fields
            .iter()
            .find(|f| f.Name == name)
            .map(|f| &f.Value)
    }
    fn WrongType(&self, name: &str, expected: &str, actual: &Value) -> OctxError {
        OctxError::New(format!(
            "field {name:?} expected {expected}, got {}",
            actual.KindName()
        ))
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct Response {
    pub Id: i64,
    pub Ok: bool,
    pub Value: Option<Value>,
    pub Error: Option<String>,
}

impl Response {
    pub fn OkVoid() -> Self {
        Self {
            Id: 0,
            Ok: true,
            Value: Some(Value::Void),
            Error: None,
        }
    }
    pub fn OkInt(value: i64) -> Self {
        Self::OkValue(Value::Int(value))
    }
    pub fn OkFloat(value: f64) -> Self {
        Self::OkValue(Value::Float(value))
    }
    pub fn OkBool(value: bool) -> Self {
        Self::OkValue(Value::Bool(value))
    }
    pub fn OkString(value: impl Into<String>) -> Self {
        Self::OkValue(Value::String(value.into()))
    }
    pub fn OkRecord(record_type: impl Into<String>, fields: Vec<Field>) -> Self {
        Self::OkValue(Value::Record {
            RecordType: record_type.into(),
            Fields: fields,
        })
    }
    pub fn Err(message: impl Into<String>) -> Self {
        Self {
            Id: 0,
            Ok: false,
            Value: None,
            Error: Some(message.into()),
        }
    }
    fn OkValue(value: Value) -> Self {
        Self {
            Id: 0,
            Ok: true,
            Value: Some(value),
            Error: None,
        }
    }
}

type Handler = Box<dyn Fn(&Request) -> Result<Response, OctxError> + Send + Sync>;

pub trait Dispatch {
    fn Dispatch(&self, request: &Request) -> Response;
}

pub struct Dispatcher {
    Family: String,
    Handlers: HashMap<String, Handler>,
}

impl Dispatcher {
    pub fn New(family: impl Into<String>) -> Self {
        Self {
            Family: family.into(),
            Handlers: HashMap::new(),
        }
    }
    pub fn Handle<F>(mut self, function: impl Into<String>, handler: F) -> Result<Self, OctxError>
    where
        F: Fn(&Request) -> Result<Response, OctxError> + Send + Sync + 'static,
    {
        let function = function.into();
        if self.Handlers.contains_key(&function) {
            return Err(OctxError::New(format!(
                "duplicate handler for {}.{function}",
                self.Family
            )));
        }
        self.Handlers.insert(function, Box::new(handler));
        Ok(self)
    }
}

impl Dispatch for Dispatcher {
    fn Dispatch(&self, request: &Request) -> Response {
        if request.Family != self.Family {
            return Response::Err(format!("unsupported family {:?}", request.Family));
        }
        let Some(handler) = self.Handlers.get(&request.Function) else {
            return Response::Err(format!("unsupported function {:?}", request.Function));
        };
        match catch_unwind(AssertUnwindSafe(|| handler(request))) {
            Ok(Ok(resp)) => resp,
            Ok(Err(err)) => Response::Err(err.Message),
            Err(_) => Response::Err("handler panic"),
        }
    }
}

pub struct CompositeDispatcher {
    Dispatchers: HashMap<String, Dispatcher>,
}
impl CompositeDispatcher {
    pub fn New() -> Self {
        Self {
            Dispatchers: HashMap::new(),
        }
    }
    pub fn Add(mut self, dispatcher: Dispatcher) -> Result<Self, OctxError> {
        if self.Dispatchers.contains_key(&dispatcher.Family) {
            return Err(OctxError::New(format!(
                "duplicate dispatcher family {:?}",
                dispatcher.Family
            )));
        }
        self.Dispatchers
            .insert(dispatcher.Family.clone(), dispatcher);
        Ok(self)
    }
}
impl Dispatch for CompositeDispatcher {
    fn Dispatch(&self, request: &Request) -> Response {
        match self.Dispatchers.get(&request.Family) {
            Some(d) => d.Dispatch(request),
            None => Response::Err(format!("unsupported family {:?}", request.Family)),
        }
    }
}

pub fn MainLoop<D: Dispatch>(dispatcher: D) -> Result<(), OctxError> {
    let stdin = io::stdin();
    let stdout = io::stdout();
    Serve(stdin.lock(), stdout.lock(), dispatcher)
}

pub fn Serve<D: Dispatch>(
    mut input: impl Read,
    mut output: impl Write,
    dispatcher: D,
) -> Result<(), OctxError> {
    ReadHandshake(&mut input)?;
    WriteHandshake(&mut output)?;
    loop {
        let frame = match ReadFrame(&mut input) {
            Ok(f) => f,
            Err(e) if e.kind() == io::ErrorKind::UnexpectedEof => return Ok(()),
            Err(e) => return Err(e.into()),
        };
        let mut response = match ParseRequest(&frame) {
            Ok(req) => {
                let mut r = dispatcher.Dispatch(&req);
                if r.Id == 0 {
                    r.Id = req.Id;
                }
                r
            }
            Err(err) => Response::Err(format!("parse request: {err}")),
        };
        WriteFrame(&mut output, &EncodeResponse(&mut response))?;
        output.flush()?;
    }
}

fn ReadHandshake(input: &mut impl Read) -> Result<(), OctxError> {
    let mut magic = [0u8; 8];
    input.read_exact(&mut magic)?;
    if &magic != MAGIC {
        return Err("invalid OCTWRAP handshake magic".into());
    }
    let major = read_u16(input)?;
    let _minor = read_u16(input)?;
    if major != OCTXILIARY_ABI_MAJOR {
        return Err(format!("unsupported OCTWRAP ABI major {major}").into());
    }
    Ok(())
}
fn WriteHandshake(output: &mut impl Write) -> Result<(), OctxError> {
    output.write_all(MAGIC)?;
    output.write_all(&OCTXILIARY_ABI_MAJOR.to_le_bytes())?;
    output.write_all(&OCTXILIARY_ABI_MINOR.to_le_bytes())?;
    output.flush()?;
    Ok(())
}
fn ReadFrame(input: &mut impl Read) -> io::Result<String> {
    let len = read_u32(input)? as usize;
    let mut body = vec![0u8; len];
    input.read_exact(&mut body)?;
    String::from_utf8(body).map_err(|e| io::Error::new(io::ErrorKind::InvalidData, e))
}
fn WriteFrame(output: &mut impl Write, body: &str) -> io::Result<()> {
    let len = u32::try_from(body.len())
        .map_err(|_| io::Error::new(io::ErrorKind::InvalidInput, "response frame too large"))?;
    output.write_all(&len.to_le_bytes())?;
    output.write_all(body.as_bytes())
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

pub fn EncodeResponse(resp: &mut Response) -> String {
    if resp.Ok {
        format!(
            "OctxiliaryResponse {{ id: {} ok: true value: {} }}",
            resp.Id,
            EncodeValue(resp.Value.as_ref().unwrap_or(&Value::Void))
        )
    } else {
        format!(
            "OctxiliaryResponse {{ id: {} ok: false error: {:?} }}",
            resp.Id,
            resp.Error.clone().unwrap_or_default()
        )
    }
}
fn EncodeValue(value: &Value) -> String {
    match value {
        Value::Void => "OctxiliaryValue { kind: \"Void\" }".to_string(),
        Value::Int(v) => format!("OctxiliaryValue {{ kind: \"Int\" int: {v} }}"),
        Value::Float(v) => format!("OctxiliaryValue {{ kind: \"Float\" float: {v} }}"),
        Value::Bool(v) => format!("OctxiliaryValue {{ kind: \"Bool\" bool: {v} }}"),
        Value::String(v) => format!("OctxiliaryValue {{ kind: \"String\" string: {v:?} }}"),
        Value::Record { RecordType, Fields } => format!(
            "OctxiliaryValue {{ kind: \"Record\" recordType: {RecordType:?} fields: [ {} ] }}",
            Fields.iter().map(EncodeField).collect::<Vec<_>>().join(" ")
        ),
    }
}
fn EncodeField(field: &Field) -> String {
    format!(
        "OctxiliaryField {{ name: {:?} value: {} }}",
        field.Name,
        EncodeValue(&field.Value)
    )
}

pub fn ParseRequest(s: &str) -> Result<Request, OctxError> {
    let prefix = "OctxiliaryRequest { id: ";
    if !s.starts_with(prefix) || !s.ends_with(" }") || !s.contains(" args: [ ") {
        return Err("unsupported request shape".into());
    }
    let mut body = s.strip_prefix(prefix).unwrap().strip_suffix(" }").unwrap();
    let (id, rest) = scan_int_then(body, " family: ")?;
    body = rest;
    let (family, rest) = scan_quoted_then(body, " function: ")?;
    body = rest;
    let (function, rest) = scan_quoted_then(body, " args: [ ")?;
    body = rest;
    if !body.ends_with(" ]") {
        return Err("malformed args payload".into());
    }
    let args = parse_values_list(body.strip_suffix(" ]").unwrap().trim())?;
    Ok(Request {
        Id: id,
        Family: family,
        Function: function,
        Args: args,
    })
}
fn parse_values_list(mut s: &str) -> Result<Vec<Value>, OctxError> {
    let mut out = Vec::new();
    s = s.trim();
    while !s.is_empty() {
        let (text, next) = take_balanced(s, "OctxiliaryValue")?;
        out.push(parse_value(text)?);
        s = next.trim();
    }
    Ok(out)
}
fn parse_value(s: &str) -> Result<Value, OctxError> {
    let prefix = "OctxiliaryValue { kind: ";
    if !s.starts_with(prefix) || !s.ends_with(" }") {
        return Err("malformed OctxiliaryValue".into());
    }
    let body = s.strip_prefix(prefix).unwrap().strip_suffix(" }").unwrap();
    let (kind, rest) = scan_quoted_remainder(body)?;
    let rest = rest.trim();
    match kind.as_str() {
        "Int" => Ok(Value::Int(
            rest.strip_prefix("int: ")
                .ok_or("Int value missing int payload")?
                .trim()
                .parse()
                .map_err(|e| OctxError::New(format!("{e}")))?,
        )),
        "Float" => Ok(Value::Float(
            rest.strip_prefix("float: ")
                .ok_or("Float value missing float payload")?
                .trim()
                .parse()
                .map_err(|e| OctxError::New(format!("{e}")))?,
        )),
        "Bool" => Ok(Value::Bool(
            rest.strip_prefix("bool: ")
                .ok_or("Bool value missing bool payload")?
                .trim()
                .parse()
                .map_err(|e| OctxError::New(format!("{e}")))?,
        )),
        "String" => Ok(Value::String(unquote(
            rest.strip_prefix("string: ")
                .ok_or("String value missing string payload")?
                .trim(),
        )?)),
        "Record" => {
            let rest = rest
                .strip_prefix("recordType: ")
                .ok_or("Record value missing recordType payload")?;
            let (record_type, rest) = scan_quoted_then(rest, " fields: [ ")?;
            if !rest.ends_with(" ]") {
                return Err("Record value missing fields payload".into());
            }
            Ok(Value::Record {
                RecordType: record_type,
                Fields: parse_fields(rest.strip_suffix(" ]").unwrap().trim())?,
            })
        }
        "Void" => Ok(Value::Void),
        _ => Err(format!("unsupported Octxiliary value kind {kind:?}").into()),
    }
}
fn parse_fields(mut s: &str) -> Result<Vec<Field>, OctxError> {
    let mut out = Vec::new();
    s = s.trim();
    while !s.is_empty() {
        let (text, next) = take_balanced(s, "OctxiliaryField")?;
        out.push(parse_field(text)?);
        s = next.trim();
    }
    Ok(out)
}
fn parse_field(s: &str) -> Result<Field, OctxError> {
    let prefix = "OctxiliaryField { name: ";
    if !s.starts_with(prefix) || !s.ends_with(" }") {
        return Err("malformed OctxiliaryField".into());
    }
    let body = s.strip_prefix(prefix).unwrap().strip_suffix(" }").unwrap();
    let (name, rest) = scan_quoted_then(body, " value: ")?;
    Ok(Field {
        Name: name,
        Value: parse_value(rest.trim())?,
    })
}
fn take_balanced<'a>(s: &'a str, label: &str) -> Result<(&'a str, &'a str), OctxError> {
    let prefix = format!("{label} {{ ");
    if !s.starts_with(&prefix) {
        return Err(format!("expected {label}").into());
    }
    let mut depth = 0i32;
    let mut in_quote = false;
    let mut escaped = false;
    for (i, ch) in s.char_indices() {
        if in_quote {
            if escaped {
                escaped = false;
                continue;
            }
            if ch == '\\' {
                escaped = true;
                continue;
            }
            if ch == '"' {
                in_quote = false;
            }
            continue;
        }
        if ch == '"' {
            in_quote = true;
            continue;
        }
        match ch {
            '{' | '[' => depth += 1,
            '}' | ']' => {
                depth -= 1;
                if depth == 0 {
                    let end = i + ch.len_utf8();
                    return Ok((&s[..end], &s[end..]));
                }
            }
            _ => {}
        }
    }
    Err(format!("unterminated {label}").into())
}
fn scan_int_then<'a>(s: &'a str, delim: &str) -> Result<(i64, &'a str), OctxError> {
    let idx = s.find(delim).ok_or("missing delimiter")?;
    Ok((
        s[..idx]
            .trim()
            .parse()
            .map_err(|e| OctxError::New(format!("{e}")))?,
        &s[idx + delim.len()..],
    ))
}
fn scan_quoted_then<'a>(s: &'a str, delim: &str) -> Result<(String, &'a str), OctxError> {
    let (v, rest) = scan_quoted_remainder(s)?;
    if !rest.starts_with(delim) {
        return Err("missing delimiter".into());
    }
    Ok((v, &rest[delim.len()..]))
}
fn scan_quoted_remainder(s: &str) -> Result<(String, &str), OctxError> {
    if !s.starts_with('"') {
        return Err("missing quote".into());
    }
    let mut escaped = false;
    for (i, ch) in s.char_indices().skip(1) {
        if ch == '\\' && !escaped {
            escaped = true;
            continue;
        }
        if ch == '"' && !escaped {
            return Ok((unquote(&s[..=i])?, &s[i + 1..]));
        }
        escaped = false;
    }
    Err("unterminated quote".into())
}
fn unquote(s: &str) -> Result<String, OctxError> {
    if s.len() < 2 || !s.starts_with('"') || !s.ends_with('"') {
        return Err("malformed quoted string".into());
    }
    let inner = &s[1..s.len() - 1];
    let mut out = String::new();
    let mut chars = inner.chars();
    while let Some(ch) = chars.next() {
        if ch != '\\' {
            out.push(ch);
            continue;
        }
        match chars.next() {
            Some('\\') => out.push('\\'),
            Some('"') => out.push('"'),
            Some('n') => out.push('\n'),
            Some('r') => out.push('\r'),
            Some('t') => out.push('\t'),
            Some(other) => {
                out.push('\\');
                out.push(other);
            }
            None => return Err("trailing escape in quoted string".into()),
        }
    }
    Ok(out)
}

#[cfg(test)]
mod tests {
    use super::*;
    fn request() -> Request {
        Request {
            Id: 1,
            Family: "ChimeraOctx".into(),
            Function: "ChimeraHello".into(),
            Args: vec![Value::Record {
                RecordType: "ChimeraRequest".into(),
                Fields: vec![
                    Field::Int("GoValue", 7),
                    Field::String("Name", "Oct"),
                    Field::Bool("Enabled", true),
                ],
            }],
        }
    }
    #[test]
    fn field_helpers_extract_required_and_optional_fields() {
        let req = request();
        assert_eq!(req.FieldInt("GoValue").unwrap(), 7);
        assert_eq!(req.FieldString("Name").unwrap(), "Oct");
        assert_eq!(req.FieldBool("Enabled").unwrap(), true);
        assert_eq!(req.OptionalInt("Missing").unwrap(), None);
        assert!(req
            .FieldFloat("GoValue")
            .unwrap_err()
            .Message
            .contains("expected Float"));
    }
    #[test]
    fn dispatcher_routes_closures_and_sets_errors() {
        let d = Dispatcher::New("ChimeraOctx")
            .Handle("ChimeraHello", |Request| {
                Ok(Response::OkInt(Request.FieldInt("GoValue")? + 35))
            })
            .unwrap();
        let mut resp = d.Dispatch(&request());
        resp.Id = 1;
        assert_eq!(EncodeResponse(&mut resp), "OctxiliaryResponse { id: 1 ok: true value: OctxiliaryValue { kind: \"Int\" int: 42 } }");
    }
    #[test]
    fn composite_dispatcher_rejects_unknown_family() {
        let d = Dispatcher::New("ChimeraOctx");
        let c = CompositeDispatcher::New().Add(d).unwrap();
        let mut req = request();
        req.Family = "Other".into();
        assert!(!c.Dispatch(&req).Ok);
    }
    #[test]
    fn parses_go_encoded_record_request() {
        let frame="OctxiliaryRequest { id: 1 family: \"ChimeraOctx\" function: \"ChimeraHello\" args: [ OctxiliaryValue { kind: \"Record\" recordType: \"ChimeraRequest\" fields: [ OctxiliaryField { name: \"GoValue\" value: OctxiliaryValue { kind: \"Int\" int: 7 } } ] } ] }";
        let req = ParseRequest(frame).unwrap();
        assert_eq!(req.FieldInt("GoValue").unwrap(), 7);
    }
}
