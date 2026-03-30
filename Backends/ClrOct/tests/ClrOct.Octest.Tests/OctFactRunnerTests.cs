using ClrOct.Octest;

namespace ClrOct.Octest.Tests;

public sealed class OctFactRunnerTests
{
    [Fact]
    public void Run_ReportsDeterministicPassFailResults()
    {
        var facts = new[]
        {
            new OctFactCase("fact.pass", static () => OctAssert.True(true)),
            new OctFactCase("fact.fail", static () => OctAssert.Equal(1, 2)),
            new OctFactCase("fact.pass.two", static () => OctAssert.Equal("oct", "oct"))
        };

        var result = OctFactRunner.Run(facts);

        Assert.Equal(3, result.Total);
        Assert.Equal(2, result.Passed);
        Assert.Equal(1, result.Failed);

        Assert.Collection(
            result.Outcomes,
            pass =>
            {
                Assert.Equal("fact.pass", pass.Name);
                Assert.True(pass.Passed);
            },
            fail =>
            {
                Assert.Equal("fact.fail", fail.Name);
                Assert.False(fail.Passed);
                Assert.Equal("Expected <1> but was <2>.", fail.FailureMessage);
            },
            pass =>
            {
                Assert.Equal("fact.pass.two", pass.Name);
                Assert.True(pass.Passed);
            });
    }
}
