namespace ClrOct.Octest;

public sealed record OctFactOutcome(string Name, bool Passed, string? FailureMessage = null)
{
    public static OctFactOutcome Pass(string name) => new(name, Passed: true);

    public static OctFactOutcome Fail(string name, string failureMessage) =>
        new(name, Passed: false, FailureMessage: failureMessage);
}
