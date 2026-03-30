namespace ClrOct.Octest;

public delegate void OctFactBody();

public sealed record OctFactCase(string Name, OctFactBody Body);
