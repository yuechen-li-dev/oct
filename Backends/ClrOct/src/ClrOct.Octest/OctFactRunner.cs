namespace ClrOct.Octest;

public static class OctFactRunner
{
    public static OctRunResult Run(IReadOnlyList<OctFactCase> facts)
    {
        var outcomes = new List<OctFactOutcome>(facts.Count);

        foreach (var fact in facts)
        {
            try
            {
                fact.Body();
                outcomes.Add(OctFactOutcome.Pass(fact.Name));
            }
            catch (Exception ex)
            {
                outcomes.Add(OctFactOutcome.Fail(fact.Name, ex.Message));
            }
        }

        return new OctRunResult(outcomes);
    }
}
