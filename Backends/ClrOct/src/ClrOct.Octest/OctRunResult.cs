namespace ClrOct.Octest;

public sealed record OctRunResult(IReadOnlyList<OctFactOutcome> Outcomes)
{
    public int Total => Outcomes.Count;

    public int Passed => Outcomes.Count(static outcome => outcome.Passed);

    public int Failed => Total - Passed;
}
